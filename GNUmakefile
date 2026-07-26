default: tests

tests: lint test

build:
	go build ./...

fmt:
	golangci-lint fmt ./...

lint:
	golangci-lint run ./...

test:
	go test ./...

cover:
	go test -coverprofile=coverage.out -covermode count ./...
	go tool cover -func coverage.out

# test-acc runs the integration tests against a real public Block Storage endpoint.
# It requires BLOCKSTORAGE_ACC=1 and the endpoint and token environment variables.
test-acc:
	BLOCKSTORAGE_ACC=1 go test ./... -run '^TestIntegration' -count=1 -v

.PHONY: tests build fmt lint test cover test-acc
