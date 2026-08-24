package snapshot

import (
	"encoding/json"
	"time"

	"github.com/selectel/blockstorage-go/pkg/v1/internal/pagination"
)

const cinderTimeLayout = "2006-01-02T15:04:05.999999"

type View struct {
	ID          string            `json:"id"`
	CreatedAt   time.Time         `json:"-"`
	Name        string            `json:"name"`
	Description string            `json:"description"`
	VolumeID    string            `json:"volume_id"`
	Status      string            `json:"status"`
	Size        int               `json:"size"`
	Metadata    map[string]string `json:"metadata"`
}

func (view *View) UnmarshalJSON(data []byte) error {
	type plain View
	decoded := struct {
		*plain
		CreatedAt string `json:"created_at"`
	}{plain: (*plain)(view)}

	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	if decoded.CreatedAt == "" {
		return nil
	}

	createdAt, err := time.Parse(cinderTimeLayout, decoded.CreatedAt)
	if err != nil {
		return err
	}
	view.CreatedAt = createdAt

	return nil
}

type viewEnvelope struct {
	Snapshot View `json:"snapshot"`
}

type pageEnvelope struct {
	Snapshots []View            `json:"snapshots"`
	Links     []pagination.Link `json:"snapshots_links"`
}

func (page *pageEnvelope) Items() []View { return page.Snapshots }

func (page *pageEnvelope) NextHref() string { return pagination.NextHref(page.Links) }
