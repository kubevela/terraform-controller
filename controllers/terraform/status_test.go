package terraform

import (
	"context"
	"testing"

	"github.com/agiledragon/gomonkey/v2"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client/config"

	"github.com/oam-dev/terraform-controller/api/types"
)

func TestGetTerraformStatus(t *testing.T) {
	ctx := context.Background()
	type args struct {
		namespace     string
		name          string
		containerName string
	}
	type want struct {
		state  types.ConfigurationState
		errMsg string
	}

	gomonkey.ApplyFunc(config.GetConfigWithContext, func(context string) (*rest.Config, error) {
		return &rest.Config{}, nil
	})

	testcases := []struct {
		name string
		args args
		want want
	}{
		{
			name: "logs are not available",
			args: args{
				namespace:     "default",
				name:          "test",
				containerName: "terraform-executor",
			},
			want: want{
				state:  types.ConfigurationProvisioningAndChecking,
				errMsg: "",
			},
		},
	}
	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			state, err := GetTerraformStatus(ctx, tc.args.namespace, tc.args.name, tc.args.containerName)
			if tc.want.errMsg != "" {
				assert.EqualError(t, err, tc.want.errMsg)
			} else {
				assert.Equal(t, tc.want.state, state)

			}
		})
	}
}

func TestGetTerraformStatus2(t *testing.T) {
	ctx := context.Background()
	type args struct {
		namespace     string
		name          string
		containerName string
	}
	type want struct {
		state  types.ConfigurationState
		errMsg string
	}

	gomonkey.ApplyFunc(config.GetConfigWithContext, func(context string) (*rest.Config, error) {
		return nil, errors.New("failed to init clientSet")
	})

	testcases := []struct {
		name string
		args args
		want want
	}{
		{
			name: "cloud are init clientSet",
			args: args{},
			want: want{
				state:  types.ConfigurationProvisioningAndChecking,
				errMsg: "failed to init clientSet",
			},
		},
	}
	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			state, err := GetTerraformStatus(ctx, tc.args.namespace, tc.args.name, tc.args.containerName)
			if tc.want.errMsg != "" {
				assert.Contains(t, err.Error(), tc.want.errMsg)
			} else {
				assert.Equal(t, tc.want.state, state)

			}
		})
	}
}
