package container

import (
	"testing"

	"github.com/oam-dev/terraform-controller/api/types"
)

func Test_getCheckoutObj(t *testing.T) {
	tests := []struct {
		name string
		ref  types.GitRef
		want string
	}{
		{
			name: "only branch",
			ref: types.GitRef{
				Branch: "feature",
			},
			want: "feature",
		},
		{
			name: "tag take precedence over branch",
			ref: types.GitRef{
				Branch: "feature",
				Tag:    "v1.0.0",
			},
			want: "v1.0.0",
		},
		{
			name: "commit take precedence over tag",
			ref: types.GitRef{
				Branch: "feature",
				Tag:    "v1.0.0",
				Commit: "123456",
			},
			want: "123456",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := getCheckoutObj(tt.ref); got != tt.want {
				t.Errorf("getCheckoutObj() = %v, want %v", got, tt.want)
			}
		})
	}
}
