/*
Copyright 2021 The KubeVela Authors.

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

package util

import (
	"bytes"
	"encoding/json"
	"text/template"

	"github.com/Masterminds/sprig/v3"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/oam-dev/terraform-controller/api/v1beta1"
)

var backendTF = `
terraform {
  backend "kubernetes" {
    secret_suffix     = "{{.SecretSuffix}}"
    in_cluster_config = {{.InClusterConfig}}
    namespace         = "{{.Namespace}}"
  }
}
`

// RawExtension2Map will convert rawExtension to map
// This function is copied from oam-dev/kubevela
func RawExtension2Map(raw *runtime.RawExtension) (map[string]interface{}, error) {
	if raw == nil {
		return nil, nil
	}
	data, err := raw.MarshalJSON()
	if err != nil {
		return nil, err
	}
	var ret map[string]interface{}
	err = json.Unmarshal(data, &ret)
	if err != nil {
		return nil, err
	}
	return ret, err
}

func renderTemplate(backend *v1beta1.Backend) (string, error) {
	tmpl, err := template.New("backend").Funcs(template.FuncMap(sprig.FuncMap())).Parse(backendTF)
	if err != nil {
		return "", err
	}
	var wr bytes.Buffer
	err = tmpl.Execute(&wr, backend)
	if err != nil {
		return "", err
	}
	return wr.String(), nil
}
