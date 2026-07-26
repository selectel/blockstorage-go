package volumetype

import (
	"context"
	"reflect"
	"testing"

	v1 "github.com/selectel/blockstorage-go/pkg/v1"
	"github.com/stretchr/testify/require"
)

var (
	_ func(context.Context, *v1.Client, string) (*View, *v1.Response, error)   = Get
	_ func(context.Context, *v1.Client, ListOpts) ([]View, error)              = List
	_ func(context.Context, *v1.Client) ([]QoSLimitsView, *v1.Response, error) = ListQoSLimits
	_                                                                          = ListOpts{Limit: int(0)}
	_                                                                          = View{
		ID: string(""), Name: string(""), Description: string(""), IsPublic: bool(false),
		ExtraSpecs: map[string]string(nil),
	}
	_ = QoSLimitsView{
		VolumeTypeID: string(""), QoSSpecs: map[string]int(nil),
		AllowUserQoS: bool(false), FullQoSDiskType: bool(false),
	}
)

func TestVolumeType_PublicDTOFields(t *testing.T) {
	t.Parallel()

	assertPublicFields(t, reflect.TypeOf(ListOpts{}), "Limit")
	assertPublicFields(t, reflect.TypeOf(View{}), "ID", "Name", "Description", "IsPublic", "ExtraSpecs")
	assertPublicFields(
		t,
		reflect.TypeOf(QoSLimitsView{}),
		"VolumeTypeID",
		"QoSSpecs",
		"AllowUserQoS",
		"FullQoSDiskType",
	)
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
