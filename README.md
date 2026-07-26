# blockstorage-go: Go SDK for the Selectel Block Storage API

Package `blockstorage-go` provides a Go SDK to work with the Selectel public Block Storage API,
which is an OpenStack Cinder v3 API.

The SDK is a transport client only:

* it accepts a regional Cinder v3 endpoint, a project-scoped token, a `User-Agent`
  and an HTTP client from the caller;
* it does not perform Keystone authentication;
* it does not poll for resource state transitions and does not retry requests on its own;
* every method performs a single HTTP request and returns either a typed response with the
  actual resource status or a typed error.

Waiting for state transitions, retries, timeouts and any lifecycle decisions belong to the caller,
for example to `terraform-provider-selectel`.

## Status

The module is under initial development. The public API is not stable yet and the module
has not been released.

## Installation

```bash
go get github.com/selectel/blockstorage-go
```

## Authentication

The SDK does not authenticate. Obtain a project-scoped Keystone token and a regional
`volumev3` endpoint from the service catalog first, for example with
[go-selvpcclient](https://github.com/selectel/go-selvpcclient), and pass them to the client.

## Usage

```go
package main

import (
    "log"

    "github.com/selectel/blockstorage-go"
)

func main() {
    client := blockstorage.NewClient(
        endpoint, // regional Cinder v3 endpoint from the service catalog
        token,    // project-scoped Keystone token
        blockstorage.WithUserAgentPrefix("my-application/v1.0.0"),
    )

    log.Println(client.Endpoint(), client.UserAgent())
}
```

## Errors

Every failure is reported as a `*blockstorage.Error` with a `Kind` that classifies it, and with
the available status code, request ID and a safe diagnostic message. Use `blockstorage.IsKind`
to branch on the error class instead of parsing messages:

```go
if blockstorage.IsKind(err, blockstorage.KindNotFound) {
    // the resource is missing for the current scope
}
```

Note that a public Cinder endpoint can answer `404` when the token lacks permissions,
so an absence has to be confirmed by the caller before it is acted upon.

## Development

```bash
make        # lint + unit tests
make lint   # golangci-lint run ./...
make test   # go test ./...
make cover  # coverage report
```

`make lint` requires `golangci-lint` v2.11 or later built with Go 1.25, the same version that CI
uses. An older binary fails with `the Go language version used to build golangci-lint is lower
than the targeted Go version`. Without a suitable binary the linter can be run directly:

```bash
go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.11.0 run ./...
```

Integration tests run against a real public endpoint and are guarded by `BLOCKSTORAGE_ACC=1`:

```bash
make test-acc
```
