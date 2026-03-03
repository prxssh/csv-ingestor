SRC_DIR := cmd/query-service
BINARY := query-service
BUILD_DIR := build

.PHONY: clean format test

docker-build: 
	mkdir -p ${BUILD_DIR}
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -o ${BUILD_DIR}/${BINARY} ${SRC_DIR}/main.go

run: 
	go run ${SRC_DIR}/main.go

clean: 
	go clean 
	rm -rf build

format: 
	golines -m 100 -t 8 --shorten-comments -w .
	gofmt -w .

test: 
	go test ./...
