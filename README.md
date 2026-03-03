# CSV Ingestor

A distributed system for uploading large CSV files, asynchronously processing them, and querying the ingested data via a REST API.

---

## Services

### ingest-service

Handles multipart CSV uploads and asynchronous processing.

- Clients upload directly to S3 using presigned URLs (no file data passes through the service)
- Tracks upload progress per-part in MongoDB
- On completion, enqueues a `csv:process` task to Redis
- An embedded Asynq worker streams the CSV from S3, parses rows, and batch-upserts movies into MongoDB
- Fully instrumented with OpenTelemetry (HTTP, MongoDB, S3, Asynq)

### query-service

Read-only REST API for querying ingested movie data.

- Connects to MongoDB with `SecondaryPreferred` read preference to offload reads from the primary
- Supports filtering by year and language, sorting by release date or vote average, and pagination
- Fully instrumented with OpenTelemetry

---

## Prerequisites

- [Docker Desktop](https://docs.docker.com/desktop/) (includes Docker Compose v2)
- [jq](https://jqlang.github.io/jq/download/) — used by the upload script
- AWS S3 bucket (or S3-compatible store like MinIO)

**Verify installation:**

```bash
docker --version        # Docker version 24+
docker compose version  # Docker Compose version v2+
jq --version            # jq-1.6+
```

---

## Running Locally

### 1. Clone the repository

```bash
git clone https://github.com/prxssh/csv-ingestor.git
cd csv-ingestor
```

### 2. Configure environment variables

The application services read S3 credentials from the host environment at `docker compose up` time.

```bash
export S3_BUCKET=your-bucket-name
export S3_REGION=ap-southeast-2
export S3_ACCESS_KEY_ID=your-access-key
export S3_SECRET_ACCESS_KEY=your-secret-key
export S3_ENDPOINT=           # leave empty for AWS, set for MinIO etc.
```

All other credentials (MongoDB, Redis, ClickHouse) use default values defined in the Compose files.

### 3. Generate the MongoDB keyfile

Required for replica set inter-node authentication:

```bash
openssl rand -base64 756 > devops/mongo-keyfile
chmod 400 devops/mongo-keyfile
```

### 4. Start the infrastructure

```bash
docker compose -f devops/docker-compose.yml up -d
```

This starts: MongoDB replica set, Redis, SigNoz (ClickHouse + OTel Collector + UI), Nginx, ingest-service, query-service.

**Check all services are healthy:**

```bash
docker compose -f devops/docker-compose.yml ps
```

### 5. Verify services are up

```bash
curl http://localhost/ping          # → ingest-service via nginx
curl http://localhost/v1/movies     # → query-service via nginx
```

### 6. Run services locally (without Docker)

Source the dev env file for each service, then run:

```bash
# ingest-service
cd ingest-service
source dev.env
make run

# query-service
cd query-service
source dev.env
make run
```

---

## API Reference

All routes are accessed via Nginx on port 80. The service listens on port 8080 internally.

### ingest-service

Base path: `/v1/uploads`

#### `POST /v1/uploads/multipart/init`

Initializes a multipart upload. Returns presigned S3 URLs for each part.

**Request:**
```json
{
  "filename": "movies.csv",
  "content_type": "text/csv",
  "total_size": 19356450
}
```

Constraints: `content_type` must be `text/csv`, `total_size` 1 byte – 1 GB.

**curl:**
```bash
curl -s -X POST http://localhost/v1/uploads/multipart/init \
  -H "Content-Type: application/json" \
  -d '{
    "filename": "movies.csv",
    "content_type": "text/csv",
    "total_size": 19356450
  }' | jq .
```

**Response `201`:**
```json
{
  "data": {
    "job_id": "69a6722ba521cfc25fe49a00",
    "upload_id": "NZr.d9Xm...",
    "parts": [
      { "part_number": 1, "url": "https://s3.amazonaws.com/...?partNumber=1&..." },
      { "part_number": 2, "url": "https://s3.amazonaws.com/...?partNumber=2&..." }
    ]
  },
  "status": "success"
}
```

Part count is calculated as `ceil(total_size / 5MB)`.

---

#### `PUT <presigned_url>`  *(direct to S3)*

Upload each part directly to S3 using the presigned URL. This request goes to S3, not the service.

```bash
curl -X PUT "<presigned_url>" \
  -H "Content-Type: text/csv" \
  --data-binary @part.bin \
  -D -   # capture ETag from response headers
```

S3 returns an `ETag` header — save it per part for the complete step.

---

#### `PATCH /v1/uploads/multipart/:id/part`

Reports a successfully uploaded part. Updates part status in MongoDB.

**curl:**
```bash
curl -s -X PATCH http://localhost/v1/uploads/multipart/<JOB_ID>/part \
  -H "Content-Type: application/json" \
  -d '{ "part_number": 1, "etag": "8bebb028023dbd5dd552274..." }' | jq .
```

**Request:**
```json
{ "part_number": 1, "etag": "8bebb028023dbd5dd552274..." }
```

**Response `200`:**
```json
{ "data": { "part_number": 1, "status": "completed" }, "status": "success" }
```

---

#### `GET /v1/uploads/multipart/:id/presign?parts=1,2,3`

Returns fresh presigned URLs for the specified parts. Used to resume interrupted uploads after URLs expire.

**curl:**
```bash
curl -s "http://localhost/v1/uploads/multipart/<JOB_ID>/presign?parts=1,2,3" | jq .
```

**Response `200`:**
```json
{
  "data": {
    "job_id": "69a6722ba521cfc25fe49a00",
    "parts": [
      { "part_number": 1, "url": "https://s3.amazonaws.com/..." }
    ]
  },
  "status": "success"
}
```

Returns `409 Conflict` if the job is already completed or aborted.

---

#### `POST /v1/uploads/multipart/:id/complete`

Completes the multipart upload. Notifies S3 to assemble the parts and enqueues the CSV processing job.

**curl:**
```bash
curl -s -X POST http://localhost/v1/uploads/multipart/<JOB_ID>/complete \
  -H "Content-Type: application/json" \
  -d '{
    "parts": [
      { "part_number": 1, "etag": "8bebb028..." },
      { "part_number": 2, "etag": "ea557f24..." }
    ]
  }' | jq .
```

**Request:**
```json
{
  "parts": [
    { "part_number": 1, "etag": "8bebb028..." },
    { "part_number": 2, "etag": "ea557f24..." }
  ]
}
```

**Response `200`:**
```json
{
  "data": {
    "job_id": "69a6722ba521cfc25fe49a00",
    "location": "https://s3.amazonaws.com/csv-ingestor/uploads/..."
  },
  "status": "success"
}
```

---

#### `DELETE /v1/uploads/multipart/:id/abort`

Aborts an in-progress upload. Cleans up the multipart upload on S3. Idempotent — aborting an already-aborted job is a no-op.

**curl:**
```bash
curl -s -X DELETE http://localhost/v1/uploads/multipart/<JOB_ID>/abort | jq .
```

**Response `200`:**
```json
{ "data": { "message": "upload aborted" }, "status": "success" }
```

---

#### `GET /v1/uploads/:id/status`

Returns the current state of an upload job including per-part status.

**curl:**
```bash
curl -s http://localhost/v1/uploads/<JOB_ID>/status | jq .
```

**Response `200`:**
```json
{
  "data": {
    "ID": "69a6722ba521cfc25fe49a00",
    "Status": "processed",
    "TotalParts": 4,
    "Parts": [
      { "PartNumber": 1, "Status": "completed", "ETag": "8bebb028...", "UpdatedAt": "..." },
      ...
    ],
    "CreatedAt": "2026-03-03T05:31:23Z",
    "UpdatedAt": "2026-03-03T05:32:16Z"
  },
  "status": "success"
}
```

**Job status lifecycle:**

```
pending → uploading → completed → processing → processed
                    ↘ aborted
                    ↘ failed
```

---

### query-service

Base path: `/v1/movies`

#### `GET /v1/movies`

Lists movies with optional filtering, sorting, and pagination.

**Query Parameters:**

| Parameter   | Type   | Default        | Description                              |
|-------------|--------|----------------|------------------------------------------|
| `page`      | int    | `1`            | Page number (min: 1)                     |
| `limit`     | int    | `20`           | Results per page (min: 1, max: 100)      |
| `year`      | int    | —              | Filter by release year                   |
| `language`  | string | —              | Filter by language (e.g. `en`, `French`) |
| `sort_by`   | string | `release_date` | `release_date` or `vote_average`         |
| `sort_dir`  | string | `desc`         | `asc` or `desc`                          |

**Example:**
```bash
curl "http://localhost/v1/movies?year=1995&language=English&sort_by=vote_average&sort_dir=desc&page=1&limit=10"
```

**Response `200`:**
```json
{
  "data": {
    "movies": [
      {
        "id": "...",
        "title": "Toy Story",
        "original_title": "Toy Story",
        "original_language": "en",
        "release_date": "1995-11-22T00:00:00Z",
        "year": 1995,
        "vote_average": 7.7,
        "vote_count": 5415,
        "budget": 30000000,
        "revenue": 373554033,
        "runtime": 81,
        "languages": ["English", "Spanish"],
        "genre_id": 16,
        "production_company_id": 3
      }
    ],
    "total": 842,
    "page": 1,
    "limit": 10,
    "total_pages": 85
  },
  "status": "success"
}
```

---

#### `GET /v1/movies/:id`

Returns a single movie by its MongoDB ObjectID.

**curl:**
```bash
curl -s http://localhost/v1/movies/<MOVIE_ID> | jq .
```

**Response `200`:** Same Movie object as above.

**Response `404`:**
```json
{ "error": "movie not found", "status": "error" }
```

---

### Health Check

Both services expose a health check endpoint routed directly (not through the load balancer logic):

```bash
curl http://localhost/ping   # {"data":"PONG","status":"success"}
```

---

## Configuration

### ingest-service

| Variable                        | Required | Default | Description                            |
|---------------------------------|----------|---------|----------------------------------------|
| `DATABASE_URL`                  | ✅       | —       | MongoDB connection string              |
| `DATABASE_POOL_MIN_CONNECTIONS` | —        | `1`     | MongoDB min pool size                  |
| `DATABASE_POOL_MAX_CONNECTIONS` | —        | `2`     | MongoDB max pool size                  |
| `OTEL_EXPORTER_URL`             | ✅       | —       | OTLP HTTP endpoint                     |
| `OTEL_SAMPLING_RATE`            | —        | `0.02`  | Tail sampling rate (1.0 = 100%)        |
| `PORT`                          | —        | `6970`  | HTTP server port                       |
| `ENVIRONMENT`                   | —        | `dev`   | Used in S3 key prefix                  |
| `REDIS_URL`                     | ✅       | —       | Redis connection string                |
| `S3_BUCKET`                     | ✅       | —       | S3 bucket name                         |
| `S3_REGION`                     | ✅       | —       | AWS region                             |
| `S3_ACCESS_KEY_ID`              | ✅       | —       | AWS access key                         |
| `S3_SECRET_ACCESS_KEY`          | ✅       | —       | AWS secret key                         |
| `S3_ENDPOINT`                   | —        | —       | Override for S3-compatible stores      |
| `S3_PRESIGN_TTL_MINS`           | —        | `60`    | Presigned URL TTL in minutes           |

### query-service

| Variable                        | Required | Default | Description                     |
|---------------------------------|----------|---------|---------------------------------|
| `DATABASE_URL`                  | ✅       | —       | MongoDB connection string       |
| `DATABASE_POOL_MIN_CONNECTIONS` | —        | `1`     | MongoDB min pool size           |
| `DATABASE_POOL_MAX_CONNECTIONS` | —        | `2`     | MongoDB max pool size           |
| `OTEL_EXPORTER_URL`             | ✅       | —       | OTLP HTTP endpoint              |
| `OTEL_SAMPLING_RATE`            | —        | `0.02`  | Tail sampling rate              |
| `PORT`                          | —        | `6969`  | HTTP server port                |
| `ENVIRONMENT`                   | —        | `dev`   | Runtime environment label       |

---

## Observability

SigNoz provides distributed tracing, metrics, and logs.

**Access the UI:** http://localhost:3301

**What is instrumented:**
- All HTTP requests (latency, status codes, route)
- MongoDB operations (query, insert, update)
- S3 operations (init, upload, complete, stream)
- Asynq task lifecycle (enqueue, dequeue, process)
- Trace context is propagated across the async boundary (HTTP → Redis → Worker)

**OTel Collector endpoints:**
- gRPC: `localhost:4317`
- HTTP: `localhost:4318`

---

## Scaling

Both application services are stateless and horizontally scalable.

```bash
# Scale to 3 instances of each
docker compose -f devops/docker-compose.yml up -d \
  --scale ingest-service=3 \
  --scale query-service=3
```

Nginx uses Docker's internal DNS resolver (`127.0.0.11`) with `set $upstream`
variable-based `proxy_pass` to dynamically resolve container IPs — no config
change needed when scaling.

---

## Upload Script

An interactive shell script for testing the full upload flow.

**Requirements:** `bash`, `curl`, `jq`, `dd`

```bash
chmod +x scripts/upload.sh
./scripts/upload.sh [base_url]   # default: http://localhost
```

**Flows:**

**1 — Success Path:** Uploads all parts sequentially, prints a summary table, and completes.

**2 — Resumeable:** Crash-safe upload. Saves the `job_id` to
`/tmp/upload_<filename>.jobid`. If interrupted (Ctrl+C, crash), re-run the
script — it automatically detects the state file, fetches fresh presigned URLs
for pending parts from the server, and resumes from where it stopped.

---

## MongoDB Collections

### `movies`

| Field                 | Type      | Notes                                |
|-----------------------|-----------|--------------------------------------|
| `_id`                 | ObjectId  |                                      |
| `title`               | string    |                                      |
| `original_title`      | string    | Part of unique index                 |
| `original_language`   | string    |                                      |
| `overview`            | string    |                                      |
| `release_date`        | date      | Part of unique index                 |
| `year`                | int32     | Derived from release_date            |
| `budget`              | int64     |                                      |
| `revenue`             | int64     |                                      |
| `runtime`             | int32     | Minutes                              |
| `vote_average`        | float64   |                                      |
| `vote_count`          | int32     |                                      |
| `languages`           | []string  | Multikey indexed                     |
| `genre_id`            | int32     |                                      |
| `production_company_id` | int32   |                                      |
| `created_at`          | date      | Set on insert only                   |
| `updated_at`          | date      | Updated on every upsert              |

Upsert key: `{ original_title, release_date }` — re-uploading the same CSV is idempotent.

### `upload_jobs`

Tracks the lifecycle of each multipart upload, including per-part status and ETags.
