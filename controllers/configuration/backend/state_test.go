/*
Copyright 2018 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package backend

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestTFConfigurationMeta_GetStateJSON(t *testing.T) {

	type args struct {
		k8sClient   client.Client
		namespace   string
		backendConf Conf
	}

	secret1 := corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "abc",
			Namespace: "default",
		},
		Data: map[string][]byte{
			"d": []byte(`MTIzNDU=`),
		},
	}
	k8sClient1 := fake.NewClientBuilder().WithObjects(&secret1).Build()

	tests := []struct {
		name    string
		args    args
		want    []byte
		wantErr string
	}{
		{
			name: "custom kubernetes, invalid kubeconfig content",
			args: args{
				k8sClient: k8sClient1,
				namespace: "default",
				backendConf: Conf{
					BackendType: "kubernetes",
					UseCustom:   true,
					HCL: `
terraform {
	backend "kubernetes" {
		secret_suffix     = "ali-oss"
		namespace         = "default"
		in_cluster_config = false
		config_path       = "backend-conf-secret/abc/d"
	}
}
`,
					Secrets: []*ConfSecretSelector{
						{
							Name: "abc",
							Path: "backend-conf-secret/abc/",
							Keys: []string{"d"},
						},
					},
				},
			},
			want:    nil,
			wantErr: "failed to initialize kubernetes configuration",
		},
		{
			name: "custom kubernetes, can not get conf secret",
			args: args{
				k8sClient: k8sClient1,
				namespace: "default",
				backendConf: Conf{
					BackendType: "kubernetes",
					UseCustom:   true,
					HCL: `
terraform {
	backend "kubernetes" {
		secret_suffix     = "ali-oss"
		namespace         = "default"
		in_cluster_config = false
		config_path       = "backend-conf-secret/abc/d"
	}
}
`,
					Secrets: []*ConfSecretSelector{
						{
							Name: "abcd",
							Path: "backend-conf-secret/abcd/",
							Keys: []string{"d"},
						},
					},
				},
			},
			want:    nil,
			wantErr: "cannot find the secret{Name: abcd, Namespace: default}",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := GetStateJSON(context.Background(), tt.args.k8sClient, tt.args.namespace, &tt.args.backendConf)
			if tt.wantErr != "" && !strings.Contains(err.Error(), tt.wantErr) || tt.wantErr == "" && err != nil {
				t.Errorf("get error: %s, but want: %s", err, tt.wantErr)
			}
			assert.Equalf(t, tt.want, got, "getStateJSON(%v, %v)", tt.args.namespace, tt.args.k8sClient)
		})
	}
}
