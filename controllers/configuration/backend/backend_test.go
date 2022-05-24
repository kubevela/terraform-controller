package backend

import (
	"reflect"
	"strings"
	"testing"

	"github.com/oam-dev/terraform-controller/api/v1beta2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestParseConfigurationBackend(t *testing.T) {
	type args struct {
		configuration *v1beta2.Configuration
	}
	type want struct {
		backend Backend
		errMsg  string
	}

	k8sClient := fake.NewClientBuilder().Build()

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
			},
			want: want{
				backend: &K8SBackend{
					Client: k8sClient,
					HCLCode: `
terraform {
  backend "kubernetes" {
    secret_suffix     = ""
    in_cluster_config = true
    namespace         = "vela-system"
  }
}
`,
					SecretSuffix: "",
					SecretNS:     "vela-system",
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
			},
			want: want{
				backend: &K8SBackend{
					Client: k8sClient,
					HCLCode: `
terraform {
  backend "kubernetes" {
    secret_suffix     = ""
    in_cluster_config = true
    namespace         = "vela-system"
  }
}
`,
					SecretSuffix: "",
					SecretNS:     "vela-system",
				},
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
			},
			want: want{
				errMsg: "the inline backend hcl code is not valid Terraform backend configuration",
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
			},
			want: want{
				errMsg: "",
				backend: &K8SBackend{
					Client: k8sClient,
					HCLCode: `
terraform {
  backend "kubernetes" {
    secret_suffix     = ""
    in_cluster_config = true
    namespace         = "vela-system"
  }
}
`,
					SecretSuffix: "",
					SecretNS:     "vela-system",
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
	secret_suffix     = "tt"
	namespace         = "vela-system"
	in_cluster_config = true
}`,
						},
					},
				},
			},
			want: want{
				errMsg: "",
				backend: &K8SBackend{
					Client: k8sClient,
					HCLCode: `
terraform {
  backend "kubernetes" {
    secret_suffix     = "tt"
    in_cluster_config = true
    namespace         = "vela-system"
  }
}
`,
					SecretSuffix: "tt",
					SecretNS:     "vela-system",
				},
			},
		},
		{
			name: "backend is not nil, use explicit backend conf",
			args: args{
				configuration: &v1beta2.Configuration{
					Spec: v1beta2.ConfigurationSpec{
						Backend: &v1beta2.Backend{
							BackendType: "kubernetes",
							Kubernetes: &v1beta2.KubernetesBackendConf{
								SecretSuffix: "suffix",
							},
						},
					},
				},
			},
			want: want{
				errMsg: "",
				backend: &K8SBackend{
					Client: k8sClient,
					HCLCode: `
terraform {
  backend "kubernetes" {
    secret_suffix     = "suffix"
    in_cluster_config = true
    namespace         = "vela-system"
  }
}
`,
					SecretSuffix: "suffix",
					SecretNS:     "vela-system",
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
							Kubernetes: &v1beta2.KubernetesBackendConf{
								SecretSuffix: "suffix",
							},
						},
					},
				},
			},
			want: want{
				errMsg: "",
				backend: &K8SBackend{
					Client: k8sClient,
					HCLCode: `
terraform {
  backend "kubernetes" {
    secret_suffix     = ""
    in_cluster_config = true
    namespace         = "vela-system"
  }
}
`,
					SecretSuffix: "",
					SecretNS:     "vela-system",
				},
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
						},
					},
				},
			},
			want: want{
				errMsg: "there is no configuration for backendType kubernetes",
			},
		},
		{
			name: "backend is not nil, use both inline and explicit",
			args: args{
				configuration: &v1beta2.Configuration{
					ObjectMeta: metav1.ObjectMeta{Namespace: "a"},
					Spec: v1beta2.ConfigurationSpec{
						Backend: &v1beta2.Backend{
							Inline:      `kkk`,
							BackendType: "kubernetes",
						},
					},
				},
			},
			want: want{
				errMsg: "it's not allowed to set `spec.backend.inline` and `spec.backend.backendType` at the same time",
			},
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ParseConfigurationBackend(tc.args.configuration, k8sClient)
			if tc.want.errMsg != "" && !strings.Contains(err.Error(), tc.want.errMsg) {
				t.Errorf("ValidConfigurationObject() error = %v, wantErr %v", err, tc.want.errMsg)
				return
			}
			if !reflect.DeepEqual(tc.want.backend, got) {
				t.Errorf("got %#v, want %#v", got, tc.want.backend)
			}
		})
	}
}
