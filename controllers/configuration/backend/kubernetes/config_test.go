/*
Copyright 2022 The KubeVela Authors.

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

package kubernetes

import (
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/oam-dev/terraform-controller/controllers/configuration/backend/util"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

func createTmpKubeConfig() string {
	tmpConfig, _ := os.CreateTemp("", "kubeconfig")
	defer tmpConfig.Close()
	tmpConfig.Write([]byte(`apiVersion: v1
clusters:
- cluster:
    certificate-authority-data: LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0tCk1JSUJkakNDQVIyZ0F3SUJBZ0lCQURBS0JnZ3Foa2pPUFFRREFqQWpNU0V3SHdZRFZRUUREQmhyTTNNdGMyVnkKZG1WeUxXTmhRREUyTlRFeU9UVTRNRGd3SGhjTk1qSXdORE13TURVeE5qUTRXaGNOTXpJd05ESTNNRFV4TmpRNApXakFqTVNFd0h3WURWUVFEREJock0zTXRjMlZ5ZG1WeUxXTmhRREUyTlRFeU9UVTRNRGd3V1RBVEJnY3Foa2pPClBRSUJCZ2dxaGtqT1BRTUJCd05DQUFSYy9RYUl1WlVzVFRLcE1uTVdUWHdTNDJZeUFCWHdLdHBnbmMyVGZWU3IKaVNldVVwVDRZVUtkN1JuT3JERlpZd0plMW1hQmkzbFVmT25aNlp6QTRybXdvMEl3UURBT0JnTlZIUThCQWY4RQpCQU1DQXFRd0R3WURWUjBUQVFIL0JBVXdBd0VCL3pBZEJnTlZIUTRFRmdRVU5rZUFuVFBIUEc2NnNjWnRhRGorClo3RVp6WDB3Q2dZSUtvWkl6ajBFQXdJRFJ3QXdSQUlnUzYxYkN1WkFEb2FJWUorT1lyN3lpM0VSOG1LRFFnYnQKRFdXRy9YOWx4VjBDSUdPS2plRUZXemRkMDBiTUQvdUFCbTZLK2ZVckJ5dDdrZEUrbkF1TTZCaFoKLS0tLS1FTkQgQ0VSVElGSUNBVEUtLS0tLQo=
    server: https://127.0.0.1:6443
  name: default
contexts:
- context:
    cluster: default
    user: default
  name: default
current-context: default
kind: Config
preferences: {}
users:
- name: default
  user:
    client-certificate-data: LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0tCk1JSUJrVENDQVRlZ0F3SUJBZ0lJTEJMdzRGaU1FYXd3Q2dZSUtvWkl6ajBFQXdJd0l6RWhNQjhHQTFVRUF3d1kKYXpOekxXTnNhV1Z1ZEMxallVQXhOalV4TWprMU9EQTRNQjRYRFRJeU1EUXpNREExTVRZME9Gb1hEVEl6TURRegpNREExTVRZME9Gb3dNREVYTUJVR0ExVUVDaE1PYzNsemRHVnRPbTFoYzNSbGNuTXhGVEFUQmdOVkJBTVRESE41CmMzUmxiVHBoWkcxcGJqQlpNQk1HQnlxR1NNNDlBZ0VHQ0NxR1NNNDlBd0VIQTBJQUJCdzNCSVVSWWR1bEY4OWsKUjBXTEdsbHY0eGo4Ym9TZnFWclNhWnQxdTlXd1VMWHVUU1hNWEJxNXV2M3pUZmhJODZ4SVU3Z2d2RU9tYlJrcgpwNXQyQitHalNEQkdNQTRHQTFVZER3RUIvd1FFQXdJRm9EQVRCZ05WSFNVRUREQUtCZ2dyQmdFRkJRY0RBakFmCkJnTlZIU01FR0RBV2dCUkhCZzNGWUFtd2dZekFqUU9xbXBvdVFxelBkakFLQmdncWhrak9QUVFEQWdOSUFEQkYKQWlFQXRhODZTbW9aQU4wZ2VrNy9zaFZxTFJla3hrVHJ0WTg2NE0zZmdwM0swQ0lDSUNqNVVtem92NkZHRitBMQpTMkJ6YzFvLzFQVVdLV0RCUEttWXR1dEltOUN1Ci0tLS0tRU5EIENFUlRJRklDQVRFLS0tLS0KLS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0tCk1JSUJkekNDQVIyZ0F3SUJBZ0lCQURBS0JnZ3Foa2pPUFFRREFqQWpNU0V3SHdZRFZRUUREQmhyTTNNdFkyeHAKWlc1MExXTmhRREUyTlRFeU9UVTRNRGd3SGhjTk1qSXdORE13TURVeE5qUTRXaGNOTXpJd05ESTNNRFV4TmpRNApXakFqTVNFd0h3WURWUVFEREJock0zTXRZMnhwWlc1MExXTmhRREUyTlRFeU9UVTRNRGd3V1RBVEJnY3Foa2pPClBRSUJCZ2dxaGtqT1BRTUJCd05DQUFSQVc4TzU3MGZzYUIvUlVObDRHTjI0d051SGhHcmIwUVd0ZTZvNTg1WVAKMkdlK21UWU9Xa1k4NllMODZ0MG96QnFuMTRtK1BhNmN4YzF5MGhGZk1uZU9vMEl3UURBT0JnTlZIUThCQWY4RQpCQU1DQXFRd0R3WURWUjBUQVFIL0JBVXdBd0VCL3pBZEJnTlZIUTRFRmdRVVJ3WU54V0FKc0lHTXdJMERxcHFhCkxrS3N6M1l3Q2dZSUtvWkl6ajBFQXdJRFNBQXdSUUlnRTZGUHd0STBwVU5uaGM4eU5QSFNzOW9VeU1Wc2hVNlAKME95aloxRUZlb01DSVFESkpGeUtZZG9wd0Z3ZXFFdXIrc1FjbmU3cWxoZzFuVHlXdEZ4eVBySEthQT09Ci0tLS0tRU5EIENFUlRJRklDQVRFLS0tLS0K
    client-key-data: LS0tLS1CRUdJTiBFQyBQUklWQVRFIEtFWS0tLS0tCk1IY0NBUUVFSUdPSjlTVjZjQ2l6dXQzOENjNDJrMlBJT2Y5UHFpYXcweXJvR1U3WW5IL1lvQW9HQ0NxR1NNNDkKQXdFSG9VUURRZ0FFSERjRWhSRmgyNlVYejJSSFJZc2FXVy9qR1B4dWhKK3BXdEpwbTNXNzFiQlF0ZTVOSmN4YwpHcm02L2ZOTitFanpyRWhUdUNDOFE2WnRHU3VubTNZSDRRPT0KLS0tLS1FTkQgRUMgUFJJVkFURSBLRVktLS0tLQo=
`))
	return tmpConfig.Name()
}

func prepareConfigData(kubeConfigPath string) util.ConfData {
	return map[string]interface{}{
		"config_path":              kubeConfigPath,
		"in_cluster_config":        false,
		"namespace":                "default",
		"secret_suffix":            "a",
		"config_context":           "default",
		"config_context_auth_info": "default",
		"config_context_cluster":   "default",
		"exec": []interface{}{
			map[string]interface{}{
				"api_version": "1.21.6",
				"args":        []interface{}{"a", "b"},
				"command":     "a",
				"env": map[string]interface{}{
					"a": "b",
				},
			},
		},
		"host":                   "127.0.0.1",
		"username":               "a",
		"password":               "b",
		"insecure":               false,
		"cluster_ca_certificate": "abc",
		"client_certificate":     "abc",
		"client_key":             "abc",
		"token":                  "a",
		"labels": map[string]interface{}{
			"a": "b",
		},
	}
}

func Test_newBackend(t *testing.T) {
	type args struct {
		conf util.ConfData
	}

	tmpConfigPath := createTmpKubeConfig()
	defer os.Remove(tmpConfigPath)

	cc := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		&clientcmd.ClientConfigLoadingRules{ExplicitPath: tmpConfigPath},
		&clientcmd.ConfigOverrides{
			CurrentContext: "default",
			Context: clientcmdapi.Context{
				AuthInfo: "default",
				Cluster:  "default",
			},
			AuthInfo: clientcmdapi.AuthInfo{
				Exec: &clientcmdapi.ExecConfig{
					Command: "a",
					Args:    []string{"a", "b"},
					Env: []clientcmdapi.ExecEnvVar{
						{Name: "a", Value: "b"},
					},
					APIVersion: "1.21.6",
				},
			},
		},
	)
	fullConfig, _ := cc.ClientConfig()
	fullConfig.UserAgent = "HashiCorp/1.0 Terraform/1.3.0"
	fullConfig.Host = "127.0.0.1"
	fullConfig.Username = "a"
	fullConfig.Password = "b"
	fullConfig.Insecure = false
	fullConfig.CAData = []byte("abc")
	fullConfig.CertData = []byte("abc")
	fullConfig.KeyData = []byte("abc")
	fullConfig.BearerToken = "a"

	tests := []struct {
		name    string
		args    args
		want    *backend
		wantErr bool
		errMsg  string
	}{
		{
			name: "full configurations, in cluster config, need error",
			args: args{
				conf: map[string]interface{}{
					"in_cluster_config": true,
					"namespace":         "default",
					"secret_suffix":     "a",
				},
			},
			want:    nil,
			wantErr: true,
			errMsg:  "unable to load in-cluster configuration, KUBERNETES_SERVICE_HOST and KUBERNETES_SERVICE_PORT must be defined",
		},
		{
			name: "full configurations, not in cluster config",
			args: args{
				conf: prepareConfigData(tmpConfigPath),
			},
			want: &backend{
				config:    fullConfig,
				namespace: "default",
				labels: map[string]string{
					"a": "b",
				},
				nameSuffix: "a",
			},
			wantErr: false,
			errMsg:  "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := newBackend(tt.args.conf)
			if (err != nil) != tt.wantErr {
				t.Errorf("newBackend() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err != nil && !strings.Contains(err.Error(), tt.errMsg) {
				t.Errorf("newBackend() get errMsg = %s, wantErr %s", err.Error(), tt.errMsg)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("newBackend() got = %#v, want %#v", got, tt.want)
				t.Errorf("newBackend() got.config = %#v, want.config %#v", got.config, tt.want.config)
			}
		})
	}
}
