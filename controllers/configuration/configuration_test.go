package configuration

import (
	"context"
	"reflect"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
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

func TestRenderConfiguration(t *testing.T) {
	type args struct {
		k8sClient         client.Client
		configuration     *v1beta2.Configuration
		ns                string
		configurationType types.ConfigurationType
	}
	type want struct {
		cfg         string
		backendConf *BackendConf
		errMsg      string
	}

	secret1 := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "abc",
			Namespace: "a",
		},
		Data: map[string][]byte{"d": []byte("something")},
	}
	k8sClient1 := fake.NewClientBuilder().WithObjects(secret1).Build()

	testcases := []struct {
		name string
		args args
		want want
	}{
		{
			name: "backend is not nil, configuration is hcl",
			args: args{
				configuration: &v1beta2.Configuration{
					Spec: v1beta2.ConfigurationSpec{
						Backend: &v1beta2.Backend{},
						HCL:     "image_id=123",
					},
				},
				ns:                "vela-system",
				configurationType: types.ConfigurationHCL,
			},
			want: want{
				cfg: `image_id=123

terraform {
	backend "kubernetes" {
secret_suffix     = ""
namespace         = "vela-system"
in_cluster_config = true

	}
}
`,
				backendConf: &BackendConf{
					BackendType: "kubernetes",
					HCL: `
terraform {
	backend "kubernetes" {
secret_suffix     = ""
namespace         = "vela-system"
in_cluster_config = true

	}
}
`,
					UseDefault: true,
					Secrets:    make(map[string][]string),
				},
			},
		},
		{
			name: "backend is nil, configuration is remote",
			args: args{
				configuration: &v1beta2.Configuration{
					Spec: v1beta2.ConfigurationSpec{
						Remote: "https://github.com/a/b.git",
					},
				},
				ns:                "vela-system",
				configurationType: types.ConfigurationRemote,
			},
			want: want{
				cfg: `
terraform {
	backend "kubernetes" {
secret_suffix     = ""
namespace         = "vela-system"
in_cluster_config = true

	}
}
`,
				backendConf: &BackendConf{
					BackendType: "kubernetes",
					HCL: `
terraform {
	backend "kubernetes" {
secret_suffix     = ""
namespace         = "vela-system"
in_cluster_config = true

	}
}
`,
					UseDefault: true,
					Secrets:    make(map[string][]string),
				},
			},
		},
		{
			name: "backend is nil, configuration is not supported",
			args: args{
				configuration: &v1beta2.Configuration{
					Spec: v1beta2.ConfigurationSpec{},
				},
				ns: "vela-system",
			},
			want: want{
				errMsg: "Unsupported Configuration Type",
			},
		},
		{
			name: "backend is not nil, use invalid(has syntax error) inline backend conf",
			args: args{
				configuration: &v1beta2.Configuration{
					Spec: v1beta2.ConfigurationSpec{
						Backend: &v1beta2.Backend{
							Inline: `
terraform {
`,
						},
					},
				},
				ns: "vela-system",
			},
			want: want{
				errMsg: "there are syntax errors in the inline backend hcl code",
			},
		},
		{
			name: "backend is not nil, use invalid inline backend conf",
			args: args{
				configuration: &v1beta2.Configuration{
					Spec: v1beta2.ConfigurationSpec{
						Backend: &v1beta2.Backend{
							Inline: `
terraform {
}
`,
						},
					},
				},
				ns: "vela-system",
			},
			want: want{
				errMsg: "the inline backend hcl code is not valid Terraform backend configuration",
			},
		},
		{
			name: "backend is not nil, use invalid (unsupported backendType) inline backend conf",
			args: args{
				configuration: &v1beta2.Configuration{
					Spec: v1beta2.ConfigurationSpec{
						Backend: &v1beta2.Backend{
							Inline: `
terraform {
	backend "local" {
		path = "/some/path"
	}
}
`,
						},
					},
				},
				ns: "vela-system",
			},
			want: want{
				errMsg: "backendType \"local\" is not supported",
			},
		},
		{
			name: "backend is not nil, use valid inline backend conf",
			args: args{
				configuration: &v1beta2.Configuration{
					Spec: v1beta2.ConfigurationSpec{
						Backend: &v1beta2.Backend{
							Inline: `
terraform {
	backend "kubernetes" {
		secret_suffix     = ""
		namespace         = "vela-system"
		in_cluster_config = true
	}
}
`,
						},
					},
				},
				configurationType: types.ConfigurationHCL,
				ns:                "vela-system",
			},
			want: want{
				errMsg: "",
				cfg: `

terraform {
	backend "kubernetes" {
		secret_suffix     = ""
		namespace         = "vela-system"
		in_cluster_config = true
	}
}
`,
				backendConf: &BackendConf{
					BackendType: "kubernetes",
					HCL: `
terraform {
	backend "kubernetes" {
		secret_suffix     = ""
		namespace         = "vela-system"
		in_cluster_config = true
	}
}
`,
					UseDefault: false,
					Secrets:    make(map[string][]string),
				},
			},
		},
		{
			name: "backend is not nil, use valid inline backend conf, should wrap",
			args: args{
				configuration: &v1beta2.Configuration{
					Spec: v1beta2.ConfigurationSpec{
						Backend: &v1beta2.Backend{
							Inline: `backend "kubernetes" {
	secret_suffix     = ""
	namespace         = "vela-system"
	in_cluster_config = true
}`,
						},
					},
				},
				configurationType: types.ConfigurationHCL,
				ns:                "vela-system",
			},
			want: want{
				errMsg: "",
				cfg: `

terraform {
backend "kubernetes" {
	secret_suffix     = ""
	namespace         = "vela-system"
	in_cluster_config = true
}
}
`,
				backendConf: &BackendConf{
					BackendType: "kubernetes",
					HCL: `
terraform {
backend "kubernetes" {
	secret_suffix     = ""
	namespace         = "vela-system"
	in_cluster_config = true
}
}
`,
					UseDefault: false,
					Secrets:    make(map[string][]string),
				},
			},
		},
		{
			name: "backend is not nil, use invalid (has invalid fields) inline backend conf",
			args: args{
				configuration: &v1beta2.Configuration{
					Spec: v1beta2.ConfigurationSpec{
						Backend: &v1beta2.Backend{
							Inline: `
terraform {
	backend "kubernetes" {
		secret_suffix     = ""
		namespace         = "vela-system"
		in_cluster_config = true
		config_path       = "~/.kube/config"
	}
}
`,
						},
					},
				},
				configurationType: types.ConfigurationHCL,
				ns:                "vela-system",
			},
			want: want{
				errMsg: "config_path is not supported in the inline backend hcl code as we cannot use local file paths in the kubernetes cluster",
			},
		},
		{
			name: "backend is not nil, use explicit backend conf",
			args: args{
				k8sClient: k8sClient1,
				configuration: &v1beta2.Configuration{
					Spec: v1beta2.ConfigurationSpec{
						Backend: &v1beta2.Backend{
							BackendType: "s3",
							S3: &v1beta2.S3BackendConf{
								Bucket: "my_bucket",
								Key:    "my_key",
								Region: "my_region",
								SharedCredentialsSecret: &crossplane.SecretKeySelector{
									SecretReference: crossplane.SecretReference{
										Name:      "abc",
										Namespace: "a",
									},
									Key: "d",
								},
							},
						},
					},
				},
				configurationType: types.ConfigurationHCL,
				ns:                "vela-system",
			},
			want: want{
				errMsg: "",
				cfg: `

terraform {
	backend "s3" {
bucket = "my_bucket"
key    = "my_key"
region = "my_region"

shared_credentials_file = "/kubevela-terraform-controller-backend-secret/abc-terraform-core-oam-dev/d"

	}
}
`,
				backendConf: &BackendConf{
					BackendType: "s3",
					HCL: `
terraform {
	backend "s3" {
bucket = "my_bucket"
key    = "my_key"
region = "my_region"

shared_credentials_file = "/kubevela-terraform-controller-backend-secret/abc-terraform-core-oam-dev/d"

	}
}
`,
					UseDefault: false,
					Secrets: map[string][]string{
						"abc-terraform-core-oam-dev": {"d"},
					},
				},
			},
		},
		{
			name: "backend is not nil, use explicit backend conf, secret in the same namespace as the configuration",
			args: args{
				k8sClient: k8sClient1,
				configuration: &v1beta2.Configuration{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "a",
					},
					Spec: v1beta2.ConfigurationSpec{
						Backend: &v1beta2.Backend{
							BackendType: "s3",
							S3: &v1beta2.S3BackendConf{
								Bucket: "my_bucket",
								Key:    "my_key",
								Region: "my_region",
								SharedCredentialsSecret: &crossplane.SecretKeySelector{
									SecretReference: crossplane.SecretReference{
										Name:      "abc",
										Namespace: "a",
									},
									Key: "d",
								},
							},
						},
					},
				},
				configurationType: types.ConfigurationHCL,
				ns:                "vela-system",
			},
			want: want{
				errMsg: "",
				cfg: `

terraform {
	backend "s3" {
bucket = "my_bucket"
key    = "my_key"
region = "my_region"

shared_credentials_file = "/kubevela-terraform-controller-backend-secret/abc/d"

	}
}
`,
				backendConf: &BackendConf{
					BackendType: "s3",
					HCL: `
terraform {
	backend "s3" {
bucket = "my_bucket"
key    = "my_key"
region = "my_region"

shared_credentials_file = "/kubevela-terraform-controller-backend-secret/abc/d"

	}
}
`,
					UseDefault: false,
					Secrets: map[string][]string{
						"abc": {"d"},
					},
				},
			},
		},
		{
			name: "backend is not nil, use explicit backend conf, no backendType",
			args: args{
				configuration: &v1beta2.Configuration{
					ObjectMeta: metav1.ObjectMeta{Namespace: "a"},
					Spec: v1beta2.ConfigurationSpec{
						Backend: &v1beta2.Backend{
							S3: &v1beta2.S3BackendConf{
								Bucket: "my_bucket",
								Key:    "my_key",
								Region: "my_region",
								SharedCredentialsSecret: &crossplane.SecretKeySelector{
									SecretReference: crossplane.SecretReference{
										Name:      "abc",
										Namespace: "a",
									},
									Key: "d",
								},
							},
						},
					},
				},
				configurationType: types.ConfigurationHCL,
				ns:                "vela-system",
			},
			want: want{
				errMsg: "",
				cfg: `

terraform {
	backend "kubernetes" {
secret_suffix     = ""
namespace         = "vela-system"
in_cluster_config = true

	}
}
`,
				backendConf: &BackendConf{
					BackendType: "kubernetes",
					HCL: `
terraform {
	backend "kubernetes" {
secret_suffix     = ""
namespace         = "vela-system"
in_cluster_config = true

	}
}
`,
					UseDefault: true,
					Secrets:    map[string][]string{},
				},
			},
		},
		{
			name: "backend is not nil, use explicit backend conf, unsupported backendType",
			args: args{
				configuration: &v1beta2.Configuration{
					ObjectMeta: metav1.ObjectMeta{Namespace: "a"},
					Spec: v1beta2.ConfigurationSpec{
						Backend: &v1beta2.Backend{
							BackendType: "someType",
							S3: &v1beta2.S3BackendConf{
								Bucket: "my_bucket",
								Key:    "my_key",
								Region: "my_region",
								SharedCredentialsSecret: &crossplane.SecretKeySelector{
									SecretReference: crossplane.SecretReference{
										Name:      "abc",
										Namespace: "a",
									},
									Key: "d",
								},
							},
						},
					},
				},
				configurationType: types.ConfigurationHCL,
				ns:                "vela-system",
			},
			want: want{
				errMsg: "someType is unsupported backendType",
			},
		},
		{
			name: "backend is not nil, use explicit backend conf, invalid backendType",
			args: args{
				configuration: &v1beta2.Configuration{
					ObjectMeta: metav1.ObjectMeta{Namespace: "a"},
					Spec: v1beta2.ConfigurationSpec{
						Backend: &v1beta2.Backend{
							BackendType: "kubernetes",
							S3: &v1beta2.S3BackendConf{
								Bucket: "my_bucket",
								Key:    "my_key",
								Region: "my_region",
								SharedCredentialsSecret: &crossplane.SecretKeySelector{
									SecretReference: crossplane.SecretReference{
										Name:      "abc",
										Namespace: "a",
									},
									Key: "d",
								},
							},
						},
					},
				},
				configurationType: types.ConfigurationHCL,
				ns:                "vela-system",
			},
			want: want{
				errMsg: "there is no configuration for backendType kubernetes",
			},
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			k8sClient := tc.args.k8sClient
			if k8sClient == nil {
				k8sClient = fake.NewClientBuilder().Build()
			}
			got, backendConf, err := RenderConfiguration(context.Background(), k8sClient, tc.args.configuration, tc.args.ns, tc.args.configurationType)
			if tc.want.errMsg != "" && !strings.Contains(err.Error(), tc.want.errMsg) {
				t.Errorf("ValidConfigurationObject() error = %v, wantErr %v", err, tc.want.errMsg)
				return
			}
			if got != tc.want.cfg {
				t.Errorf("ValidConfigurationObject() = %v, want %v", got, tc.want.cfg)
				return
			}

			if !reflect.DeepEqual(tc.want.backendConf, backendConf) {
				t.Errorf("ValidBackendSecretList() = %#v, want %#v", backendConf, tc.want.backendConf)
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
	v1beta1.AddToScheme(s)
	v1beta2.AddToScheme(s)
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
	v1beta2.AddToScheme(s)
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
