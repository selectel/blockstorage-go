/*
Package blockstorage provides a Go SDK to work with the Selectel public
Block Storage API, which is an OpenStack Cinder v3 API.

The SDK is a transport client only. It accepts a ready to use endpoint,
an authentication token, a User-Agent and an HTTP transport from the caller.
It does not perform Keystone authentication, does not poll for resource state
transitions and does not retry requests on its own.

Every method performs a single HTTP request and returns either a typed
response with the actual resource status or a typed error that keeps the
available status code, request ID and a safe diagnostic message.
*/
package blockstorage
