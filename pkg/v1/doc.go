/*
Package v1 provides the client and common types for the Selectel Block Storage SDK.
The package version is independent of the Cinder API version.

The client uses a Cinder v3 endpoint and a project-scoped token supplied by the caller. It does
not authenticate, retry requests, or wait for resource state changes. API operations are provided
by resource packages such as github.com/selectel/blockstorage-go/pkg/v1/volume.
*/
package v1
