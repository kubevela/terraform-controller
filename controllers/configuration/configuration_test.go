package configuration

import (
	"context"
	"strings"
	"testing"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/oam-dev/terraform-controller/api/types"
	"github.com/oam-dev/terraform-controller/api/v1beta1"
)

func TestReplaceTerraformSource(t *testing.T) {
	testcases := []struct {
		remote        string
		githubBlocked string
		expected      string
	}{
		{
			remote:        "",
			expected:      "",
			githubBlocked: "xxx",
		},
		{
			remote:        "https://github.com/kubevela-contrib/terraform-modules.git",
			expected:      "https://github.com/kubevela-contrib/terraform-modules.git",
			githubBlocked: "false",
		},
		{
			remote:        "https://github.com/kubevela-contrib/terraform-modules.git",
			expected:      "https://gitee.com/kubevela-contrib/terraform-modules.git",
			githubBlocked: "true",
		},
		{
			remote:        "https://github.com/abc/terraform-modules.git",
			expected:      "https://gitee.com/kubevela-terraform-source/terraform-modules.git",
			githubBlocked: "true",
		},
		{
			remote:        "abc",
			githubBlocked: "true",
			expected:      "abc",
		},
		{
			remote:        "",
			githubBlocked: "true",
			expected:      "",
		},
	}

	for _, tc := range testcases {
		t.Run(tc.remote, func(t *testing.T) {
			actual := ReplaceTerraformSource(tc.remote, tc.githubBlocked)
			if actual != tc.expected {
				t.Errorf("expected %s, got %s", tc.expected, actual)
			}
		})
	}
}

func TestIsDeletable(t *testing.T) {
	ctx := context.Background()
	s := runtime.NewScheme()
	v1beta1.AddToScheme(s)
	provider2 := &v1beta1.Provider{
		ObjectMeta: ctrl.ObjectMeta{
			Name:      "default",
			Namespace: "default",
		},
		Status: v1beta1.ProviderStatus{
			State: types.ProviderIsNotReady,
		},
	}
	provider3 := &v1beta1.Provider{
		ObjectMeta: ctrl.ObjectMeta{
			Name:      "default",
			Namespace: "default",
		},
		Status: v1beta1.ProviderStatus{
			State: types.ProviderIsReady,
		},
	}
	k8sClient1 := fake.NewClientBuilder().WithScheme(s).Build()
	k8sClient2 := fake.NewClientBuilder().WithScheme(s).WithObjects(provider2).Build()
	k8sClient3 := fake.NewClientBuilder().WithScheme(s).WithObjects(provider3).Build()
	type args struct {
		configuration *v1beta1.Configuration
		k8sClient     client.Client
	}
	type want struct {
		deletable bool
		errMsg    string
	}
	testcases := []struct {
		name string
		args args
		want want
	}{
		{
			name: "provider is not found",
			args: args{
				k8sClient:     k8sClient1,
				configuration: &v1beta1.Configuration{},
			},
			want: want{
				deletable: true,
			},
		},
		{
			name: "provider is not ready",
			args: args{
				k8sClient:     k8sClient2,
				configuration: &v1beta1.Configuration{},
			},
			want: want{
				deletable: true,
			},
		},
		{
			name: "configuration is provisioning",
			args: args{
				k8sClient: k8sClient3,
				configuration: &v1beta1.Configuration{
					Status: v1beta1.ConfigurationStatus{
						Apply: v1beta1.ConfigurationApplyStatus{
							State: types.ConfigurationProvisioningAndChecking,
						},
					},
				},
			},
			want: want{
				errMsg: "Destroy could not complete and needs to wait for Provision to complete first",
			},
		},
		{
			name: "configuration is ready",
			args: args{
				k8sClient: k8sClient3,
				configuration: &v1beta1.Configuration{
					Status: v1beta1.ConfigurationStatus{
						Apply: v1beta1.ConfigurationApplyStatus{
							State: types.Available,
						},
					},
				},
			},
			want: want{},
		},
	}
	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := IsDeletable(ctx, tc.args.k8sClient, tc.args.configuration)
			if err != nil {
				if !strings.Contains(err.Error(), tc.want.errMsg) {
					t.Errorf("IsDeletable() error = %v, wantErr %v", err, tc.want.errMsg)
					return
				}
			}
			if got != tc.want.deletable {
				t.Errorf("IsDeletable() = %v, want %v", got, tc.want.deletable)
			}
		})
	}
}
