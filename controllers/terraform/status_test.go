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
			state, err := GetTerraformStatus(ctx, tc.args.name, tc.args.namespace, tc.args.containerName, "")
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
			name: "failed to init clientSet",
			args: args{},
			want: want{
				state:  types.ConfigurationProvisioningAndChecking,
				errMsg: "failed to init clientSet",
			},
		},
	}
	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			state, err := GetTerraformStatus(ctx, tc.args.name, tc.args.namespace, tc.args.containerName, "")
			if tc.want.errMsg != "" {
				assert.Contains(t, err.Error(), tc.want.errMsg)
			} else {
				assert.Equal(t, tc.want.state, state)

			}
		})
	}
}

func TestAnalyzeTerraformLog(t *testing.T) {
	type args struct {
		logs string
	}
	type want struct {
		success bool
		state   types.ConfigurationState
		errMsg  string
	}

	testcases := []struct {
		name string
		args args
		want want
	}{
		{
			name: "normal failed logs",
			args: args{
				logs: "31mError:",
			},
			want: want{
				success: false,
				state:   types.ConfigurationApplyFailed,
				errMsg:  "31mError:",
			},
		},
		{
			name: "invalid region",
			args: args{
				logs: "31mError:\nInvalid Alibaba Cloud region",
			},
			want: want{
				success: false,
				state:   types.InvalidRegion,
				errMsg:  "Invalid Alibaba Cloud region",
			},
		},
	}
	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			success, state, errMsg := analyzeTerraformLog(tc.args.logs, types.TerraformApply)
			if tc.want.errMsg != "" {
				assert.Contains(t, errMsg, tc.want.errMsg)
			} else {
				assert.Equal(t, tc.want.success, success)
				assert.Equal(t, tc.want.state, state)

			}
		})
	}
}
