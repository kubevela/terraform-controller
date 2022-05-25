package backend

import (
	"context"
	"encoding/base64"
	"reflect"
	"testing"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestK8SBackend_HCL(t *testing.T) {
	type fields struct {
		HCLCode      string
		SecretSuffix string
		SecretNS     string
	}

	k8sClient := fake.NewClientBuilder().Build()

	tests := []struct {
		name   string
		fields fields
		want   string
	}{
		{
			name: "HCLCode is empty",
			fields: fields{
				SecretSuffix: "tt",
				SecretNS:     "ac",
			},
			want: `
terraform {
  backend "kubernetes" {
    secret_suffix     = "tt"
    in_cluster_config = true
    namespace         = "ac"
  }
}
`,
		},
		{
			name: "HCLCode is not empty",
			fields: fields{
				HCLCode: `
terraform {
  backend "kubernetes" {
    secret_suffix     = "tt"
    in_cluster_config = true
    namespace         = "ac"
  }
}
`,
			},
			want: `
terraform {
  backend "kubernetes" {
    secret_suffix     = "tt"
    in_cluster_config = true
    namespace         = "ac"
  }
}
`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			k := &K8SBackend{
				Client:       k8sClient,
				HCLCode:      tt.fields.HCLCode,
				SecretSuffix: tt.fields.SecretSuffix,
				SecretNS:     tt.fields.SecretNS,
			}
			if got := k.HCL(); got != tt.want {
				t.Errorf("HCL() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestK8SBackend_GetTFStateJSON(t *testing.T) {
	type fields struct {
		Client       client.Client
		HCLCode      string
		SecretSuffix string
		SecretNS     string
	}
	type args struct {
		ctx context.Context
	}
	tfStateData, _ := base64.StdEncoding.DecodeString("H4sIAAAAAAAA/4SQzarbMBCF934KoXUdPKNf+1VKCWNp5AocO8hyaSl592KlcBd3cZfnHPHpY/52QshfXI68b3IS+tuVK5dCaS+P+8ci4TbcULb94JJplZPAFte8MS18PQrKBO8Q+xk59SHa1AMA9M4YmoN3FGJ8M/azPs96yElcCkLIsG+V8sblnqOc3uXlRuvZ0GxSSuiCRUYbw2gGHRFGPxitEgJYQDQ0a68I2ChNo1cAZJ2bR20UtW8bsv55NuJRS94W2erXe5X5QQs3A/FZ4fhJaOwUgZTVMRjto1HGpSGSQuuD955hdDDPcR6NY1ZpQJ/YwagTRAvBpsi8LXn7Pa1U+ahfWHX/zWThYz9L4Otg3390r+5fAAAA//8hmcuNuQEAAA==")
	secret := &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "tfstate-default-a",
			Namespace: "default",
		},
		Type: v1.SecretTypeOpaque,
		Data: map[string][]byte{
			TerraformStateNameInSecret: tfStateData,
		},
	}
	k8sClient := fake.NewClientBuilder().WithObjects(secret).Build()
	k8sClient2 := fake.NewClientBuilder().Build()
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    []byte
		wantErr bool
	}{
		{
			name: "valid",
			fields: fields{
				Client:       k8sClient,
				HCLCode:      "",
				SecretSuffix: "a",
				SecretNS:     "default",
			},
			args: args{ctx: context.Background()},
			want: []byte(`{
  "version": 4,
  "terraform_version": "1.0.2",
  "serial": 2,
  "lineage": "c35c8722-b2ef-cd6f-1111-755abc87acdd",
  "outputs": {
    "container_id":{
      "value": "e5fff27c62e26dc9504d21980543f21161225ab483a1e534a98311a677b9453a",
      "type": "string"
    },
    "image_id": {
      "value": "sha256:d1a364dc548d5357f0da3268c888e1971bbdb957ee3f028fe7194f1d61c6fdeenginx:latest",
      "type": "string"
    }
  },
  "resources": []
}
`),
		},
		{
			name: "secret doesn't exist",
			fields: fields{
				Client:       k8sClient2,
				HCLCode:      "",
				SecretSuffix: "a",
				SecretNS:     "default",
			},
			args:    args{ctx: context.Background()},
			want:    nil,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			k := &K8SBackend{
				Client:       tt.fields.Client,
				HCLCode:      tt.fields.HCLCode,
				SecretSuffix: tt.fields.SecretSuffix,
				SecretNS:     tt.fields.SecretNS,
			}
			got, err := k.GetTFStateJSON(tt.args.ctx)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetTFStateJSON() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GetTFStateJSON() got = %v, want %v", got, tt.want)
				t.Errorf("GetTFStateJSON() got = %s, want %s", got, tt.want)
			}
		})
	}
}

func TestK8SBackend_CleanUp(t *testing.T) {
	type fields struct {
		Client       client.Client
		HCLCode      string
		SecretSuffix string
		SecretNS     string
	}
	type args struct {
		ctx context.Context
	}
	secret := v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "tfstate-default-a",
			Namespace: "default",
		},
		Type: v1.SecretTypeOpaque,
	}
	k8sClient := fake.NewClientBuilder().WithObjects(&secret).Build()
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		{
			name: "valid",
			fields: fields{
				Client:       k8sClient,
				SecretSuffix: "a",
				SecretNS:     "default",
			},
			args: args{ctx: context.Background()},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			k := &K8SBackend{
				Client:       tt.fields.Client,
				HCLCode:      tt.fields.HCLCode,
				SecretSuffix: tt.fields.SecretSuffix,
				SecretNS:     tt.fields.SecretNS,
			}
			if err := k.CleanUp(tt.args.ctx); (err != nil) != tt.wantErr {
				t.Errorf("CleanUp() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
