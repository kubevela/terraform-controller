package backend

import (
	"reflect"
	"strings"
	"testing"

	"github.com/oam-dev/terraform-controller/api/v1beta2"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestParseConfigurationBackend(t *testing.T) {
	type args struct {
		configuration *v1beta2.Configuration
		credentials   map[string]string
	}
	type want struct {
		backend Backend
		errMsg  string
	}

	secret := &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "a",
			Name:      "secretref",
		},
		Data: map[string][]byte{
			"access": []byte("access_key"),
		},
	}
	configMap := &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "a",
			Name:      "configmapref",
		},
		Data: map[string]string{
			"token": "token",
		},
	}
	k8sClient := fake.NewClientBuilder().WithObjects(secret, configMap).Build()

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
    namespace         = ""
  }
}
`,
					SecretSuffix: "",
					SecretNS:     "",
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
    namespace         = ""
  }
}
`,
					SecretSuffix: "",
					SecretNS:     "",
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
							BackendType: backendTypeK8S,
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
    namespace         = ""
  }
}
`,
					SecretSuffix: "suffix",
					SecretNS:     "",
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
    namespace         = "a"
  }
}
`,
					SecretSuffix: "",
					SecretNS:     "a",
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
							BackendType: backendTypeK8S,
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
							BackendType: backendTypeK8S,
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
			got, err := ParseConfigurationBackend(tc.args.configuration, k8sClient, tc.args.credentials)
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
