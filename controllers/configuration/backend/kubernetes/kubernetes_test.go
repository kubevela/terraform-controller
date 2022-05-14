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
	"context"
	"os"
	"reflect"
	"testing"

	"github.com/oam-dev/terraform-controller/controllers/configuration/backend/util"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/dynamic"
	dynamicfake "k8s.io/client-go/dynamic/fake"
)

func TestClient_Get(t *testing.T) {
	type fields struct {
		kubernetesSecretClient dynamic.ResourceInterface
		namespace              string
		labels                 map[string]string
		nameSuffix             string
	}

	secret1 := &unstructured.Unstructured{}
	secret1.SetUnstructuredContent(map[string]interface{}{
		"apiVersion": "v1",
		"kind":       "Secret",
		"metadata": map[string]interface{}{
			"name":      "tfstate-default-a",
			"namespace": "default",
		},
		"data": map[string]interface{}{
			"tfstate": "H4sIAAAAAAAA/5xUT2/bPgy951MYOtep4ya/JgF66G/oaX8O67ZLGwiyRLvaFCog5XTGkO8+WEmcxGm3Yjebj3rvkSL1a5AkYg3E1qOYJ+OL9j8AkSo9LeUBEaNhNsxFxBnIKifmSR5/nUVQFbRJemRmJpvMUjMdzVIzKSepKfNJmk9hWhajosyu/tty+Dqs6sBinrQWkkT8//Xd+7sv8tPtx7su2HpTro7UnalUewzknQNK8yzPs3E+TWfZ0DOnGtMC7HeL1VA529Soeaj9MkpGutCsIhsHsliJGN4MkmQTTRGwr0lDa+shYp2PpTfx4FKhqsCcMypntfO1kZ5ZFrX+AeGQhGoZk7bxVGl3wFbk19YAtfj+++FREFSWAzXDrvCh9ZdPip+s9rS63Os9isWByyIHhccFHBcRU1g/wVId3Wx2cQyrEMgWdQA+uoU9pt3WpF2rAOLiFN3V/Iar6p/UnlhS7doWPSz6IIEK1qM0reY8ES1Rmk3SbNrngZ+BFEKQgGblLUYzbxmL3fnSkwZpgAP5RsyTUjmGXo41/1Khxb86Sy0GIFTujxadLUE32sGr/XJex36dK5xx+apqwy+SREhaBlRFFMLauV6Wf8bt2I5G19lknGXj/Oo6y8ejvtLKO6vbjoo+QmBqNAp1I/eL9OHz/XlWCQQktcfSvmiYgdZAkq0BCaipWcWZea1JHDypCqR2ittBF/dBoVFk+spBVXERNv04KeQSSCqtwQHtW36mtNuzV/r8DAXbsLV4hJyoCQZkG+wa5MlynpJ1W9m+MviNi9ubG9Hhm93XonvwFoPN4HcAAAD//6YF2S39BQAA",
		},
	})
	client1 := dynamicfake.NewSimpleDynamicClient(runtime.NewScheme(), secret1).Resource(secretResource).Namespace("default")

	tests := []struct {
		name    string
		fields  fields
		want    []byte
		wantErr bool
	}{
		{
			name:   "normal",
			fields: fields{kubernetesSecretClient: client1, namespace: "default", labels: nil, nameSuffix: "a"},
			want: []byte(`{
  "version": 4,
  "terraform_version": "1.0.2",
  "serial": 2,
  "lineage": "c1d9d059-d819-d5f5-df25-28e8fb1bf036",
  "outputs": {
    "BUCKET_NAME": {
      "value": "terraform-controller-20220428-90.oss-cn-beijing.aliyuncs.com",
      "type": "string"
    }
  },
  "resources": [
    {
      "mode": "managed",
      "type": "alicloud_oss_bucket",
      "name": "bucket-acl",
      "provider": "provider[\"registry.terraform.io/hashicorp/alicloud\"]",
      "instances": [
        {
          "schema_version": 0,
          "attributes": {
            "acl": "private",
            "bucket": "terraform-controller-20220428-90",
            "cors_rule": [],
            "creation_date": "2022-05-08",
            "extranet_endpoint": "oss-cn-beijing.aliyuncs.com",
            "force_destroy": false,
            "id": "terraform-controller-20220428-90",
            "intranet_endpoint": "oss-cn-beijing-internal.aliyuncs.com",
            "lifecycle_rule": [],
            "location": "oss-cn-beijing",
            "logging": [],
            "logging_isenable": null,
            "owner": "1170540042370241",
            "policy": "",
            "redundancy_type": "LRS",
            "referer_config": [],
            "server_side_encryption_rule": [],
            "storage_class": "Standard",
            "tags": {},
            "transfer_acceleration": [],
            "versioning": [],
            "website": []
          },
          "sensitive_attributes": [],
          "private": "bnVsbA=="
        }
      ]
    }
  ]
}
`),
		},
		{
			name: "invalid suffix",
			fields: fields{
				kubernetesSecretClient: client1,
				namespace:              "default",
				nameSuffix:             "*w4789-&a",
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "cannot find secret",
			fields: fields{
				kubernetesSecretClient: client1,
				namespace:              "default",
				nameSuffix:             "bbc",
			},
			want:    nil,
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Client{
				kubernetesSecretClient: tt.fields.kubernetesSecretClient,
				namespace:              tt.fields.namespace,
				labels:                 tt.fields.labels,
				nameSuffix:             tt.fields.nameSuffix,
			}
			got, err := c.Get(context.Background())
			if (err != nil) != tt.wantErr {
				t.Errorf("Get() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Get() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNewClient(t *testing.T) {

	kubeConfigPath := createTmpKubeConfig()
	defer os.Remove(kubeConfigPath)

	tests := []struct {
		name     string
		confData util.ConfData
		want     util.Client
		wantErr  bool
	}{
		{
			name:     "normal",
			confData: prepareConfigData(kubeConfigPath),
			want:     nil,
			wantErr:  true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewClient(tt.confData)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewClient() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("NewClient() got = %v, want %v", got, tt.want)
			}
		})
	}
}
