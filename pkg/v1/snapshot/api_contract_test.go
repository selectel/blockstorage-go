package snapshot_test

import (
	"context"
	"reflect"
	"testing"
	"time"

	v1 "github.com/selectel/blockstorage-go/pkg/v1"
	"github.com/selectel/blockstorage-go/pkg/v1/snapshot"
	"github.com/stretchr/testify/require"
)

var (
	_ func(context.Context, *v1.Client, string) (*snapshot.View, *v1.Response, error) = snapshot.Get
	_ func(context.Context, *v1.Client, snapshot.ListOpts) ([]snapshot.View, error)   = snapshot.List
	_                                                                                 = snapshot.ListOpts{
		Name: string(""), Status: string(""), VolumeID: string(""),
	}
	_ = snapshot.View{
		ID: string(""), CreatedAt: time.Time{}, Name: string(""), Description: string(""),
		VolumeID: string(""), Status: string(""), Size: int(0), Metadata: map[string]string(nil),
	}
)

func TestSnapshotPublicFields(t *testing.T) {
	t.Parallel()

	assertPublicFields(t, reflect.TypeOf(snapshot.ListOpts{}), "Name", "Status", "VolumeID")
	assertPublicFields(t, reflect.TypeOf(snapshot.View{}),
		"ID", "CreatedAt", "Name", "Description", "VolumeID", "Status", "Size", "Metadata")
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
