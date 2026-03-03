########## BUILD ##########
FROM golang:1.25-alpine3.23 AS build

RUN apk add --no-cache make git

WORKDIR /app
COPY . .
RUN go mod download
RUN make docker-build

########## DEPLOY ##########
FROM alpine:3.23

WORKDIR /app
COPY --from=build /app/build/ingest-service ./ingest-service

EXPOSE 8080
ENTRYPOINT ["./ingest-service"]
