package backend

import (
	"reflect"
	"strings"
	"testing"

	"github.com/oam-dev/terraform-controller/api/v1beta2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestParseConfigurationBackend(t *testing.T) {
	type args struct {
		configuration             *v1beta2.Configuration
		terraformBackendNamespace string
	}
	type want struct {
		backendConf *Conf
		errMsg      string
	}

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
				terraformBackendNamespace: "vela-system",
			},
			want: want{
				backendConf: &Conf{
					BackendType: "kubernetes",
					HCL: `
terraform {
  backend "kubernetes" {
    secret_suffix     = ""
    in_cluster_config = true
    namespace         = "vela-system"
  }
}
`,
					UseCustom: false,
					Secrets:   nil,
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
				terraformBackendNamespace: "vela-system",
			},
			want: want{
				backendConf: &Conf{
					BackendType: "kubernetes",
					HCL: `
terraform {
  backend "kubernetes" {
    secret_suffix     = ""
    in_cluster_config = true
    namespace         = "vela-system"
  }
}
`,
					UseCustom: false,
					Secrets:   nil,
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
				terraformBackendNamespace: "vela-system",
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
				terraformBackendNamespace: "vela-system",
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
				terraformBackendNamespace: "vela-system",
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
				terraformBackendNamespace: "vela-system",
			},
			want: want{
				errMsg: "",
				backendConf: &Conf{
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
					UseCustom: true,
					Secrets:   nil,
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
				terraformBackendNamespace: "vela-system",
			},
			want: want{
				errMsg: "",
				backendConf: &Conf{
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
					UseCustom: true,
					Secrets:   nil,
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
				terraformBackendNamespace: "vela-system",
			},
			want: want{
				errMsg: "config_path is not supported in the inline backend hcl code as we cannot use local file paths in the kubernetes cluster",
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
								ConfigSecret: &v1beta2.CurrentNSSecretSelector{
									Name: "abc",
									Key:  "d",
								},
							},
						},
					},
				},
				terraformBackendNamespace: "vela-system",
			},
			want: want{
				errMsg: "",
				backendConf: &Conf{
					BackendType: "kubernetes",
					HCL: `
terraform {
	backend "kubernetes" {
secret_suffix = "suffix"

config_path = "backend-conf-secret/abc/d"

	}
}
`,
					UseCustom: true,
					Secrets: []*ConfSecretSelector{
						{
							Name: "abc",
							Path: "backend-conf-secret/abc",
							Keys: []string{"d"},
						},
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
							Kubernetes: &v1beta2.KubernetesBackendConf{
								SecretSuffix: "suffix",
							},
						},
					},
				},
				terraformBackendNamespace: "vela-system",
			},
			want: want{
				errMsg: "",
				backendConf: &Conf{
					BackendType: "kubernetes",
					HCL: `
terraform {
  backend "kubernetes" {
    secret_suffix     = ""
    in_cluster_config = true
    namespace         = "vela-system"
  }
}
`,
					UseCustom: false,
					Secrets:   nil,
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
				terraformBackendNamespace: "vela-system",
			},
			want: want{
				errMsg: "there is no configuration for backendType kubernetes",
			},
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ParseConfigurationBackend(tc.args.configuration, tc.args.terraformBackendNamespace)
			if tc.want.errMsg != "" && !strings.Contains(err.Error(), tc.want.errMsg) {
				t.Errorf("ValidConfigurationObject() error = %v, wantErr %v", err, tc.want.errMsg)
				return
			}
			if !reflect.DeepEqual(tc.want.backendConf, got) {
				t.Errorf("got %#v, want %#v", got, tc.want.backendConf)
			}
		})
	}
}
