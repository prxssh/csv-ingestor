# CSV Ingestor

A distributed system for uploading large CSV files, asynchronously processing
them, and querying the ingested data via a REST API.

---

## Architecture Diagram

![High Level Design](assets/HLD.svg)

> [Edit on Excalidraw](https://excalidraw.com/#json=Mpti5ej08wBY5XJAYy3SH,Nd4UAlFH3_5z2dPxV7_eSQ)

---

## Architecture Overview

### Services

**ingest-service** (port `6970`) — handles multipart CSV uploads and async
processing.
- Clients upload directly to S3 via presigned URLs — no file data passes
  through the service
- Tracks per-part upload progress in MongoDB
- On completion, enqueues a `csv:process` task to Redis (Asynq)
- An embedded worker streams the CSV from S3, parses rows, and batch-upserts
  movies into MongoDB

**query-service** (port `6969`) — read-only REST API for querying ingested
movie data.
- Supports filtering by year and language, sorting by release date or vote
  average, and pagination
- Uses `SecondaryPreferred` read preference to offload reads from the MongoDB
  primary

### Databases

| Store | Role |
|---|---|
| MongoDB replica set (1 primary + 2 secondaries) | Persistent storage for movies and upload jobs |
| Redis | Asynq job queue for CSV processing tasks |

### Blob Storage

AWS S3 (or any S3-compatible store). Files are never routed through the service
— clients PUT parts directly to S3 using presigned URLs. S3 keys follow the
pattern:

```
uploads/{env}/{year}/{month}/{day}/{uuid}/{filename}
```

### Nginx

Acts as a reverse proxy and load balancer in front of both services.

**Routing:**
- `/v1/uploads/*` → `ingest-service`
- `/v1/movies/*` → `query-service`
- `/ping` → `ingest-service`

**Rate limiting** (applied to all application routes):
- Request rate: `10 req/s` per IP, burst of `20` (`nodelay`)
- Concurrent connections: `10` per IP

**Load balancing:** Uses Docker's internal DNS resolver (`127.0.0.11`) with
variable-based `proxy_pass` — new instances are picked up automatically on
`--scale` without any config changes.

### SigNoz Observability

Distributed tracing, metrics, and logs via OpenTelemetry → SigNoz (ClickHouse-backed).

**Instrumented:** HTTP requests, MongoDB operations, S3 operations, Asynq task
lifecycle. Trace context is propagated across the async boundary (HTTP → Redis
→ Worker).

| Endpoint | Address |
|---|---|
| SigNoz UI | `http://localhost:3301` |
| OTel collector HTTP | `localhost:4318` |
| OTel collector gRPC | `localhost:4317` |

---

## Installation & Setup

### Prerequisites

- [Docker Desktop](https://docs.docker.com/desktop/) (includes Compose v2)
- [jq](https://jqlang.github.io/jq/download/)
- AWS S3 bucket (or S3-compatible store like MinIO)

### Clone & Run

```bash
git clone https://github.com/prxssh/csv-ingestor.git
cd csv-ingestor

# Generate MongoDB keyfile (required for replica set auth)
openssl rand -base64 756 > devops/mongo-keyfile
chmod 400 devops/mongo-keyfile

# Set S3 credentials
export S3_BUCKET=your-bucket-name
export S3_REGION=ap-southeast-2
export S3_ACCESS_KEY_ID=your-access-key
export S3_SECRET_ACCESS_KEY=your-secret-key
export S3_ENDPOINT=   # leave empty for AWS, set for MinIO

# Start everything
docker compose -f devops/docker-compose.yml up -d

# Verify
curl http://localhost/ping       # → {"data":"PONG","status":"success"}
curl http://localhost/v1/movies  # → {"data":{...},"status":"success"}
```

All other credentials (MongoDB, Redis, ClickHouse) use defaults defined in the
Compose files.

### Environment Variables

<details>
<summary>ingest-service</summary>

| Variable | Required | Default | Description |
|---|---|---|---|
| `DATABASE_URL` | ✅ | — | MongoDB connection string |
| `REDIS_URL` | ✅ | — | Redis connection string |
| `OTEL_EXPORTER_URL` | ✅ | — | OTLP HTTP endpoint |
| `S3_BUCKET` | ✅ | — | S3 bucket name |
| `S3_REGION` | ✅ | — | AWS region |
| `S3_ACCESS_KEY_ID` | ✅ | — | AWS access key |
| `S3_SECRET_ACCESS_KEY` | ✅ | — | AWS secret key |
| `S3_ENDPOINT` | — | — | Override for S3-compatible stores |
| `S3_PRESIGN_TTL_MINS` | — | `60` | Presigned URL TTL in minutes |
| `PORT` | — | `6970` | HTTP server port |
| `ENVIRONMENT` | — | `dev` | Used in S3 key prefix |
| `OTEL_SAMPLING_RATE` | — | `0.02` | Trace sampling rate (1.0 = 100%) |
| `DATABASE_POOL_MIN_CONNECTIONS` | — | `1` | MongoDB min pool size |
| `DATABASE_POOL_MAX_CONNECTIONS` | — | `2` | MongoDB max pool size |

</details>

<details>
<summary>query-service</summary>

| Variable | Required | Default | Description |
|---|---|---|---|
| `DATABASE_URL` | ✅ | — | MongoDB connection string |
| `OTEL_EXPORTER_URL` | ✅ | — | OTLP HTTP endpoint |
| `PORT` | — | `6969` | HTTP server port |
| `ENVIRONMENT` | — | `dev` | Runtime environment label |
| `OTEL_SAMPLING_RATE` | — | `0.02` | Trace sampling rate |
| `DATABASE_POOL_MIN_CONNECTIONS` | — | `1` | MongoDB min pool size |
| `DATABASE_POOL_MAX_CONNECTIONS` | — | `2` | MongoDB max pool size |

</details>

### Scaling

Both services are stateless and horizontally scalable:

```bash
docker compose -f devops/docker-compose.yml up -d \
  --scale ingest-service=3 \
  --scale query-service=3
```

### API — Postman Collection

A ready-to-use collection covering all endpoints is at:
[data/CSV Service Postman Collection.postman_collection.json](data/CSV%20Service%20Postman%20Collection.postman_collection.json)

Import into Postman, set the `base_url` variable to `http://localhost`, and run requests in order: **Init → Upload Parts → Report Parts → Complete → Query**.

**Endpoints at a glance:**

| Service | Method | Path |
|---|---|---|
| ingest-service | `POST` | `/v1/uploads/multipart/init` |
| ingest-service | `PUT` | `<presigned_url>` *(direct to S3)* |
| ingest-service | `PATCH` | `/v1/uploads/multipart/:id/part` |
| ingest-service | `GET` | `/v1/uploads/multipart/:id/presign` |
| ingest-service | `POST` | `/v1/uploads/multipart/:id/complete` |
| ingest-service | `DELETE` | `/v1/uploads/multipart/:id/abort` |
| ingest-service | `GET` | `/v1/uploads/:id/status` |
| query-service | `GET` | `/v1/movies` |
| query-service | `GET` | `/v1/movies/:id` |

**Upload job lifecycle:**

```
pending → uploading → completed → processing → processed
                               ↘ aborted
                               ↘ failed
```

### Upload Script

An interactive shell script for testing the full upload flow end-to-end.

```bash
chmod +x scripts/upload.sh
./scripts/upload.sh [base_url]   # default: http://localhost
```

**Requirements:** `bash`, `curl`, `jq`, `dd`

Two flows are available:

**1 — Success Path** — uploads all parts sequentially, prints a summary table,
and completes the upload.

**2 — Resumeable** — crash-safe upload. Saves the `job_id` to
`/tmp/upload_<filename>.jobid`. If interrupted (Ctrl+C, crash, expired URLs),
re-run the script — it detects the state file, fetches fresh presigned URLs for
pending parts, and resumes from where it stopped.
