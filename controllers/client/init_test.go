package client

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/agiledragon/gomonkey/v2"
	"github.com/stretchr/testify/assert"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
)

func TestInit(t *testing.T) {
	type args struct {
		configFile string
	}

	type want struct {
		errMsg string
	}

	pwd, err := os.Getwd()
	assert.NoError(t, err)
	kubeConfig := filepath.Join(pwd, "config")
	assert.NoError(t, os.WriteFile(kubeConfig, []byte(""), 0400))
	defer func() {
		if err := os.Remove(kubeConfig); err != nil {
			t.Errorf("failed to remove kubeConfig: %v", err)
		}
	}()
	if err := os.Setenv("KUBECONFIG", kubeConfig); err != nil {
		t.Errorf("failed to set KUBECONFIG: %v", err)
	}

	testcases := []struct {
		name string
		args args
		want want
	}{
		{
			name: "init",
			args: args{},
			want: want{
				errMsg: "invalid configuration",
			},
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := Init(); tc.want.errMsg != "" && !strings.Contains(err.Error(), tc.want.errMsg) {
				t.Errorf("Init() error = %v, wantErr %v", err, tc.want.errMsg)
			}
		})
	}
}

func TestInitWithWrongConfig(t *testing.T) {
	type args struct {
		configFile string
	}

	type want struct {
		errMsg string
	}

	gomonkey.ApplyFunc(config.GetConfigWithContext, func(_ string) (*rest.Config, error) {
		return &rest.Config{}, nil
	})

	testcases := []struct {
		name string
		args args
		want want
	}{
		{
			name: "init",
			args: args{},
			want: want{},
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := Init(); tc.want.errMsg != "" && !strings.Contains(err.Error(), tc.want.errMsg) {
				t.Errorf("Init() error = %v, wantErr %v", err, tc.want.errMsg)
			}
		})
	}
}
