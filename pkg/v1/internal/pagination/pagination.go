package pagination

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"

	v1 "github.com/selectel/blockstorage-go/pkg/v1"
	"github.com/selectel/blockstorage-go/pkg/v1/internal/transport"
)

const maxPages = 10000

type Link struct {
	Href string `json:"href"`
	Rel  string `json:"rel"`
}

func NextHref(links []Link) string {
	for _, candidate := range links {
		if candidate.Rel == "next" {
			return candidate.Href
		}
	}

	return ""
}

type Page[T any] interface {
	Items() []T
	NextHref() string
}

func ReadAll[T any, P Page[T]](
	ctx context.Context,
	client *v1.Client,
	path string,
	responseEnvelope string,
	query url.Values,
	limit int,
	newPage func() P,
) ([]T, error) {
	options := []transport.RequestOption{
		transport.WithResponseEnvelope(responseEnvelope),
	}
	mergedQuery := make(url.Values, len(query)+1)
	for key, values := range query {
		mergedQuery[key] = values
	}
	if limit > 0 {
		mergedQuery.Set("limit", strconv.Itoa(limit))
	}
	if len(mergedQuery) > 0 {
		options = append(options, transport.WithQuery(mergedQuery))
	}

	items := make([]T, 0)
	visited := make(map[string]struct{})

	for page := 1; ; page++ {
		if page > maxPages {
			return nil, incomplete(fmt.Sprintf("the listing did not end within %d pages", maxPages), nil)
		}

		decoded := newPage()

		if _, err := transport.DoRequest(
			ctx, client, http.MethodGet, path, http.StatusOK, nil, decoded, options...,
		); err != nil {
			return nil, incomplete(fmt.Sprintf("the listing failed on page %d", page), err)
		}

		items = append(items, decoded.Items()...)

		next := decoded.NextHref()
		if next == "" {
			return items, nil
		}

		if _, repeated := visited[next]; repeated {
			return nil, incomplete("the listing repeated a page of itself", nil)
		}

		visited[next] = struct{}{}
		path = next
		options = []transport.RequestOption{
			transport.WithResponseEnvelope(responseEnvelope),
		}
	}
}

func incomplete(message string, cause error) *v1.Error {
	return &v1.Error{Kind: v1.KindIncompleteList, Message: message, Err: cause}
}
