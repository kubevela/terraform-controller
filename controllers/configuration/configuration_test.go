package configuration

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/oam-dev/terraform-controller/api/types"
	crossplane "github.com/oam-dev/terraform-controller/api/types/crossplane-runtime"
	"github.com/oam-dev/terraform-controller/api/v1beta1"
	"github.com/oam-dev/terraform-controller/api/v1beta2"
)

func TestValidConfigurationObject(t *testing.T) {
	type args struct {
		configuration *v1beta2.Configuration
	}
	type want struct {
		configurationType types.ConfigurationType
		errMsg            string
	}

	testcases := []struct {
		name string
		args args
		want want
	}{
		{
			name: "hcl",
			args: args{
				configuration: &v1beta2.Configuration{
					Spec: v1beta2.ConfigurationSpec{
						HCL: "abc",
					},
				},
			},
			want: want{
				configurationType: types.ConfigurationHCL,
			},
		},
		{
			name: "remote",
			args: args{
				configuration: &v1beta2.Configuration{
					Spec: v1beta2.ConfigurationSpec{
						Remote: "def",
					},
				},
			},
			want: want{
				configurationType: types.ConfigurationRemote,
			},
		},
		{
			name: "remote and hcl are set",
			args: args{
				configuration: &v1beta2.Configuration{
					Spec: v1beta2.ConfigurationSpec{
						HCL:    "abc",
						Remote: "def",
					},
				},
			},
			want: want{
				configurationType: "",
				errMsg:            "spec.HCL and spec.Remote cloud not be set at the same time",
			},
		},
		{
			name: "remote and hcl are not set",
			args: args{
				configuration: &v1beta2.Configuration{
					Spec: v1beta2.ConfigurationSpec{},
				},
			},
			want: want{
				configurationType: "",
				errMsg:            "spec.HCL or spec.Remote should be set",
			},
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ValidConfigurationObject(tc.args.configuration)
			if tc.want.errMsg != "" && !strings.Contains(err.Error(), tc.want.errMsg) {
				t.Errorf("ValidConfigurationObject() error = %v, wantErr %v", err, tc.want.errMsg)
				return
			}
			if got != tc.want.configurationType {
				t.Errorf("ValidConfigurationObject() = %v, want %v", got, tc.want.configurationType)
			}
		})
	}

}

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
	_ = v1beta1.AddToScheme(s)
	_ = v1beta2.AddToScheme(s)
	provider2 := &v1beta1.Provider{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "default",
			Namespace: "default",
		},
		Status: v1beta1.ProviderStatus{
			State: types.ProviderIsNotReady,
		},
	}
	provider3 := &v1beta1.Provider{
		ObjectMeta: metav1.ObjectMeta{
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
	k8sClient4 := fake.NewClientBuilder().Build()

	configuration := &v1beta2.Configuration{
		ObjectMeta: metav1.ObjectMeta{
			Name: "abc",
		},
	}
	configuration.Spec.ProviderReference = &crossplane.Reference{
		Name:      "default",
		Namespace: "default",
	}
	configuration.Spec.InlineCredentials = false

	defaultConfiguration := &v1beta2.Configuration{}
	defaultConfiguration.Spec.InlineCredentials = false

	provisioningConfiguration := &v1beta2.Configuration{
		Status: v1beta2.ConfigurationStatus{
			Apply: v1beta2.ConfigurationApplyStatus{
				State: types.ConfigurationProvisioningAndChecking,
			},
		},
	}
	provisioningConfiguration.Spec.InlineCredentials = false

	readyConfiguration := &v1beta2.Configuration{
		Status: v1beta2.ConfigurationStatus{
			Apply: v1beta2.ConfigurationApplyStatus{
				State: types.Available,
			},
		},
	}
	readyConfiguration.Spec.InlineCredentials = false

	inlineConfiguration := &v1beta2.Configuration{}
	inlineConfiguration.Spec.InlineCredentials = true

	type args struct {
		configuration *v1beta2.Configuration
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
				configuration: defaultConfiguration,
			},
			want: want{
				deletable: true,
			},
		},
		{
			name: "provider is not ready, use default providerRef",
			args: args{
				k8sClient:     k8sClient2,
				configuration: defaultConfiguration,
			},
			want: want{
				deletable: true,
			},
		},
		{
			name: "provider is not ready, providerRef is set in configuration spec",
			args: args{
				k8sClient:     k8sClient2,
				configuration: configuration,
			},
			want: want{
				deletable: true,
			},
		},
		{
			name: "configuration is provisioning",
			args: args{
				k8sClient:     k8sClient3,
				configuration: provisioningConfiguration,
			},
			want: want{
				errMsg: "Destroy could not complete and needs to wait for Provision to complete first",
			},
		},
		{
			name: "configuration is ready",
			args: args{
				k8sClient:     k8sClient3,
				configuration: readyConfiguration,
			},
			want: want{},
		},
		{
			name: "failed to get provider",
			args: args{
				k8sClient:     k8sClient4,
				configuration: defaultConfiguration,
			},
			want: want{
				errMsg: "failed to get Provider object",
			},
		},
		{
			name: "no provider is needed",
			args: args{
				k8sClient:     k8sClient4,
				configuration: inlineConfiguration,
			},
			want: want{
				deletable: false,
			},
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

func TestSetRegion(t *testing.T) {
	ctx := context.Background()
	s := runtime.NewScheme()
	_ = v1beta2.AddToScheme(s)
	k8sClient := fake.NewClientBuilder().WithScheme(s).Build()
	configuration1 := v1beta2.Configuration{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "abc",
			Namespace: "default",
		},
		Spec: v1beta2.ConfigurationSpec{},
	}
	configuration1.Spec.Region = "xxx"
	assert.Nil(t, k8sClient.Create(ctx, &configuration1))

	configuration2 := v1beta2.Configuration{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "def",
			Namespace: "default",
		},
		Spec: v1beta2.ConfigurationSpec{},
	}
	assert.Nil(t, k8sClient.Create(ctx, &configuration2))

	provider := &v1beta1.Provider{
		Spec: v1beta1.ProviderSpec{
			Region: "yyy",
		},
	}

	type args struct {
		namespace string
		name      string
	}

	type want struct {
		region string
		errMsg string
	}

	testcases := map[string]struct {
		args args
		want want
	}{
		"configuration is available, region is set": {
			args: args{
				namespace: "default",
				name:      "abc",
			},
			want: want{
				region: "xxx",
			},
		},
		"configuration is available, region is not set": {
			args: args{
				namespace: "default",
				name:      "def",
			},
			want: want{
				region: "yyy",
			},
		},
		"configuration isn't available": {
			args: args{
				namespace: "default",
				name:      "ghi",
			},
			want: want{
				errMsg: "failed to get configuration",
			},
		},
	}
	for name, tc := range testcases {
		t.Run(name, func(t *testing.T) {
			region, err := SetRegion(ctx, k8sClient, tc.args.namespace, tc.args.name, provider)
			if tc.want.errMsg != "" && !strings.Contains(err.Error(), tc.want.errMsg) {
				t.Errorf("SetRegion() error = %v, wantErr %v", err, tc.want.errMsg)
			}
			if region != tc.want.region {
				t.Errorf("SetRegion() want = %s, got %s", tc.want.region, region)

			}
		})
	}
}

func TestGetConfigurationsWithSameBackendReference(t *testing.T) {
	ctx := context.Background()
	s := runtime.NewScheme()
	_ = v1beta2.AddToScheme(s)
	k8sClient := fake.NewClientBuilder().WithScheme(s).Build()

	var (
		region    = "cn"
		namespace = "default"
	)

	configuration1 := v1beta2.Configuration{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "c1",
			Namespace: "default",
		},
		Spec: v1beta2.ConfigurationSpec{},
	}
	assert.Nil(t, k8sClient.Create(ctx, &configuration1))

	configuration2 := v1beta2.Configuration{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "c2",
			Namespace: "default",
		},
		Spec: v1beta2.ConfigurationSpec{
			Backend: &v1beta2.Backend{
				SecretSuffix: "s1",
			},
		},
	}
	assert.Nil(t, k8sClient.Create(ctx, &configuration2))

	configuration3 := v1beta2.Configuration{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "c3",
			Namespace: "default",
		},
		Spec: v1beta2.ConfigurationSpec{
			Backend: &v1beta2.Backend{
				SecretSuffix: "s2",
			},
		},
	}
	assert.Nil(t, k8sClient.Create(ctx, &configuration3))

	configuration4 := v1beta2.Configuration{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "c4",
			Namespace: "default",
		},
		Spec: v1beta2.ConfigurationSpec{
			Backend: &v1beta2.Backend{
				SecretSuffix: "s2",
			},
		},
	}
	assert.Nil(t, k8sClient.Create(ctx, &configuration4))

	configuration5 := v1beta2.Configuration{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "c5",
			Namespace: "default",
		},
		Spec: v1beta2.ConfigurationSpec{
			Backend: &v1beta2.Backend{
				Inline: "inline",
			},
		},
	}
	assert.Nil(t, k8sClient.Create(ctx, &configuration5))

	configuration6 := v1beta2.Configuration{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "c6",
			Namespace: "default",
		},
		Spec: v1beta2.ConfigurationSpec{
			Backend: &v1beta2.Backend{
				Inline: "inline",
			},
		},
	}
	assert.Nil(t, k8sClient.Create(ctx, &configuration6))

	configuration7 := v1beta2.Configuration{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "c7",
			Namespace: "default",
		},
		Spec: v1beta2.ConfigurationSpec{
			Backend: &v1beta2.Backend{
				BackendType: "s3",
				S3: &v1beta2.S3BackendConf{
					Region: &region,
					Bucket: "test",
					Key:    "test",
				},
			},
		},
	}
	assert.Nil(t, k8sClient.Create(ctx, &configuration7))

	configuration8 := v1beta2.Configuration{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "c8",
			Namespace: "default",
		},
		Spec: v1beta2.ConfigurationSpec{
			Backend: &v1beta2.Backend{
				BackendType: "s3",
				S3: &v1beta2.S3BackendConf{
					Region: &region,
					Bucket: "test",
					Key:    "test",
				},
			},
		},
	}
	assert.Nil(t, k8sClient.Create(ctx, &configuration8))

	configuration9 := v1beta2.Configuration{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "c9",
			Namespace: "default",
		},
		Spec: v1beta2.ConfigurationSpec{
			Backend: &v1beta2.Backend{
				BackendType: "s3",
				S3: &v1beta2.S3BackendConf{
					Region: &region,
					Bucket: "test2",
					Key:    "test2",
				},
			},
		},
	}
	assert.Nil(t, k8sClient.Create(ctx, &configuration9))

	configuration10 := v1beta2.Configuration{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "c10",
			Namespace: "default",
		},
		Spec: v1beta2.ConfigurationSpec{
			Backend: &v1beta2.Backend{
				BackendType: "kubernetes",
				Kubernetes: &v1beta2.KubernetesBackendConf{
					SecretSuffix: "k8s",
					Namespace:    &namespace,
				},
			},
		},
	}
	assert.Nil(t, k8sClient.Create(ctx, &configuration10))

	configuration11 := v1beta2.Configuration{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "c11",
			Namespace: "default",
		},
		Spec: v1beta2.ConfigurationSpec{
			Backend: &v1beta2.Backend{
				BackendType: "kubernetes",
				Kubernetes: &v1beta2.KubernetesBackendConf{
					SecretSuffix: "k8s",
					Namespace:    &namespace,
				},
			},
		},
	}
	assert.Nil(t, k8sClient.Create(ctx, &configuration11))

	configuration12 := v1beta2.Configuration{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "c12",
			Namespace: "default",
		},
		Spec: v1beta2.ConfigurationSpec{
			Backend: &v1beta2.Backend{
				BackendType: "kubernetes",
				Kubernetes: &v1beta2.KubernetesBackendConf{
					SecretSuffix: "k8s2",
					Namespace:    &namespace,
				},
			},
		},
	}
	assert.Nil(t, k8sClient.Create(ctx, &configuration12))

	type args struct {
		backend *v1beta2.Backend
	}

	type want struct {
		references []*crossplane.Reference
		errMsg     string
	}

	emptyReferences := make([]*crossplane.Reference, 0)

	testcases := map[string]struct {
		args args
		want want
	}{
		"Using the default backend": {
			args: args{
				backend: nil,
			},
			want: want{
				references: emptyReferences,
				errMsg:     "",
			},
		},
		"Configured with secret suffix": {
			args: args{
				backend: &v1beta2.Backend{SecretSuffix: "s2"},
			},
			want: want{
				references: []*crossplane.Reference{
					{
						Name:      "c3",
						Namespace: "default",
					},
					{
						Name:      "c4",
						Namespace: "default",
					},
				},
				errMsg: "",
			},
		},
		"Configured with inline": {
			args: args{
				backend: &v1beta2.Backend{Inline: "inline"},
			},
			want: want{
				references: []*crossplane.Reference{
					{
						Name:      "c5",
						Namespace: "default",
					},
					{
						Name:      "c6",
						Namespace: "default",
					},
				},
				errMsg: "",
			},
		},
		"Using S3 as the backend": {
			args: args{
				backend: &v1beta2.Backend{
					BackendType: "s3",
					S3: &v1beta2.S3BackendConf{
						Region: &region,
						Bucket: "test",
						Key:    "test",
					},
				},
			},
			want: want{
				references: []*crossplane.Reference{
					{
						Name:      "c7",
						Namespace: "default",
					},
					{
						Name:      "c8",
						Namespace: "default",
					},
				},
				errMsg: "",
			},
		},
		"Using Kubernetes as the backend": {
			args: args{
				backend: &v1beta2.Backend{
					BackendType: "kubernetes",
					Kubernetes: &v1beta2.KubernetesBackendConf{
						SecretSuffix: "k8s",
						Namespace:    &namespace,
					},
				},
			},
			want: want{
				references: []*crossplane.Reference{
					{
						Name:      "c10",
						Namespace: "default",
					},
					{
						Name:      "c11",
						Namespace: "default",
					},
				},
				errMsg: "",
			},
		},
	}
	for name, tc := range testcases {
		t.Run(name, func(t *testing.T) {
			refs, err := GetConfigurationsWithSameBackendReference(ctx, k8sClient, &v1beta2.Configuration{
				Spec: v1beta2.ConfigurationSpec{
					Backend: tc.args.backend,
				},
			})
			if tc.want.errMsg != "" && !strings.Contains(err.Error(), tc.want.errMsg) {
				t.Errorf("GetConfigurationsWithSameBackendReference() error = %v, wantErr %v", err, tc.want.errMsg)
			}
			if !reflect.DeepEqual(refs, tc.want.references) {
				wantReferencesStr := ""
				for _, r := range tc.want.references {
					if wantReferencesStr != "" {
						wantReferencesStr += ","
					}
					wantReferencesStr += fmt.Sprintf("%v", r)
				}
				referencesStr := ""
				for _, r := range refs {
					if referencesStr != "" {
						referencesStr += ","
					}
					referencesStr += fmt.Sprintf("%v", r)
				}
				t.Errorf("GetConfigurationsWithSameBackendReference() want = [%s], got [%s]", wantReferencesStr, referencesStr)
			}
		})
	}

}
