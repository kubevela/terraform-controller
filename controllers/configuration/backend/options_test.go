package backend

import (
	"context"
	"testing"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestOptionSource_getOption(t *testing.T) {
	type fields struct {
		Envs []v1.EnvVar
	}
	type args struct {
		k8sClient client.Client
		namespace string
		key       string
	}

	secret1 := &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "a",
			Name:      "secretref",
		},
		Data: map[string][]byte{
			"access": []byte("access_key"),
		},
	}
	envs1 := []v1.EnvVar{
		{
			Name: "accessKey",
			ValueFrom: &v1.EnvVarSource{
				SecretKeyRef: &v1.SecretKeySelector{
					LocalObjectReference: v1.LocalObjectReference{Name: "secretref"},
					Key:                  "access",
				},
			},
		},
	}
	k8sClient1 := fake.NewClientBuilder().WithObjects(secret1).Build()

	configMap2 := &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "a",
			Name:      "configmapref",
		},
		Data: map[string]string{
			"token": "token",
		},
	}
	envs2 := []v1.EnvVar{
		{
			Name: "token2",
			ValueFrom: &v1.EnvVarSource{
				ConfigMapKeyRef: &v1.ConfigMapKeySelector{
					LocalObjectReference: v1.LocalObjectReference{Name: "configmapref"},
					Key:                  "token",
				},
			},
		},
	}
	k8sClient2 := fake.NewClientBuilder().WithObjects(configMap2).Build()

	tests := []struct {
		name    string
		fields  fields
		args    args
		want    string
		want1   bool
		wantErr bool
	}{
		{
			name:   "cannot find in envs",
			fields: fields{Envs: nil},
			args: args{
				k8sClient: nil,
				namespace: "ns",
				key:       "a",
			},
			want:    "",
			want1:   false,
			wantErr: false,
		},
		{
			name: "explicit env value in envs",
			fields: fields{
				Envs: []v1.EnvVar{
					{
						Name:  "kk",
						Value: "bb",
					},
				},
			},
			args: args{
				k8sClient: fake.NewClientBuilder().Build(),
				namespace: "a",
				key:       "kk",
			},
			want:  "bb",
			want1: true,
		},
		{
			name:   "secretref key in envs",
			fields: fields{Envs: envs1},
			args: args{
				k8sClient: k8sClient1,
				namespace: "a",
				key:       "accessKey",
			},
			want:  "access_key",
			want1: true,
		},
		{
			name:   "configmapref key in envs",
			fields: fields{Envs: envs2},
			args: args{
				k8sClient: k8sClient2,
				namespace: "a",
				key:       "token2",
			},
			want:  "token",
			want1: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			source := OptionSource{
				Envs: tt.fields.Envs,
			}
			got, got1, err := source.getOption(context.Background(), tt.args.k8sClient, tt.args.namespace, tt.args.key)
			if (err != nil) != tt.wantErr {
				t.Errorf("getOption() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("getOption() got = %v, want %v", got, tt.want)
			}
			if got1 != tt.want1 {
				t.Errorf("getOption() got1 = %v, want %v", got1, tt.want1)
			}
		})
	}
}
