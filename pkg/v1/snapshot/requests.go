package snapshot

import (
	"context"
	"net/http"
	"net/url"

	v1 "github.com/selectel/blockstorage-go/pkg/v1"
	"github.com/selectel/blockstorage-go/pkg/v1/internal/pagination"
	"github.com/selectel/blockstorage-go/pkg/v1/internal/transport"
)

const basePath = "snapshots"

type ListOpts struct {
	Name     string
	Status   string
	VolumeID string
}

// Get returns one snapshot by ID.
func Get(ctx context.Context, client *v1.Client, snapshotID string) (*View, *v1.Response, error) {
	if snapshotID == "" {
		return nil, nil, &v1.Error{Kind: v1.KindInvalidRequest, Message: "the snapshot ID is required"}
	}

	envelope := &viewEnvelope{}
	response, err := transport.DoRequest(
		ctx,
		client,
		http.MethodGet,
		basePath+"/"+url.PathEscape(snapshotID),
		http.StatusOK,
		nil,
		envelope,
		transport.WithResponseEnvelope("snapshot"),
	)
	if err != nil {
		return nil, response, err
	}

	return &envelope.Snapshot, response, nil
}

// List returns all matching snapshots and never returns a partial result.
func List(ctx context.Context, client *v1.Client, opts ListOpts) ([]View, error) {
	query := url.Values{}
	if opts.Name != "" {
		query.Set("name", opts.Name)
	}
	if opts.Status != "" {
		query.Set("status", opts.Status)
	}
	if opts.VolumeID != "" {
		query.Set("volume_id", opts.VolumeID)
	}

	return pagination.ReadAll[View](ctx, client, basePath, "snapshots", query, 0, func() *pageEnvelope {
		return &pageEnvelope{}
	})
}
