package volume

import (
	"context"
	"reflect"
	"testing"

	v1 "github.com/selectel/blockstorage-go/pkg/v1"
	"github.com/stretchr/testify/require"
)

var (
	_ func(context.Context, *v1.Client, CreateOpts) (*View, *v1.Response, error)         = Create
	_ func(context.Context, *v1.Client, string) (*View, *v1.Response, error)             = Get
	_ func(context.Context, *v1.Client, string, UpdateOpts) (*View, *v1.Response, error) = Update
	_ func(context.Context, *v1.Client, string, ExtendOpts) (*v1.Response, error)        = Extend
	_ func(context.Context, *v1.Client, string) (*v1.Response, error)                    = Delete
	_ func(context.Context, *v1.Client, ListOpts) ([]View, error)                        = ListDetail

	_ = CreateOpts{
		Size: int(0), Name: string(""), Description: string(""), AvailabilityZone: string(""),
		Metadata: map[string]string(nil), VolumeType: string(""), SnapshotID: string(""),
		SourceVolID: string(""), ImageID: string(""), BackupID: string(""),
	}
	_ = UpdateOpts{Name: (*string)(nil), Description: (*string)(nil), Metadata: map[string]string(nil)}
	_ = ExtendOpts{NewSize: int(0), Attached: bool(false)}
	_ = ListOpts{Limit: int(0)}
	_ = View{
		ID: string(""), Status: string(""), Size: int(0), Name: string(""), Description: string(""),
		AvailabilityZone: string(""), VolumeType: string(""), SnapshotID: string(""),
		SourceVolID: string(""), Metadata: map[string]string(nil), Attachments: []Attachment(nil),
		Bootable: string(""),
	}
	_ = Attachment{ID: string(""), AttachmentID: string(""), ServerID: string(""), Device: string("")}
)

func TestVolume_PublicDTOFields(t *testing.T) {
	t.Parallel()

	assertPublicFields(t, reflect.TypeOf(CreateOpts{}),
		"Size", "Name", "Description", "AvailabilityZone", "Metadata", "VolumeType",
		"SnapshotID", "SourceVolID", "ImageID", "BackupID")
	assertPublicFields(t, reflect.TypeOf(UpdateOpts{}), "Name", "Description", "Metadata")
	assertPublicFields(t, reflect.TypeOf(ExtendOpts{}), "NewSize", "Attached")
	assertPublicFields(t, reflect.TypeOf(ListOpts{}), "Limit")
	assertPublicFields(t, reflect.TypeOf(View{}),
		"ID", "Status", "Size", "Name", "Description", "AvailabilityZone", "VolumeType",
		"SnapshotID", "SourceVolID", "Metadata", "Attachments", "Bootable")
	assertPublicFields(t, reflect.TypeOf(Attachment{}), "ID", "AttachmentID", "ServerID", "Device")
}

func assertPublicFields(t *testing.T, typ reflect.Type, expected ...string) {
	t.Helper()

	actual := make([]string, 0, typ.NumField())
	for index := 0; index < typ.NumField(); index++ {
		field := typ.Field(index)
		if field.IsExported() {
			actual = append(actual, field.Name)
		}
	}

	require.ElementsMatch(t, expected, actual, "unexpected public fields of %s", typ)
}
