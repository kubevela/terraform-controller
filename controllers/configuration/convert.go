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

package configuration

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"
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

type backendVars struct {
	SecretSuffix    string
	InClusterConfig bool
	Namespace       string
}

// RenderTemplate renders Backend template
func RenderTemplate(backend *v1beta1.Backend, namespace string) (string, error) {
	tmpl, err := template.New("backend").Funcs(template.FuncMap(sprig.FuncMap())).Parse(backendTF)
	if err != nil {
		return "", err
	}

	templateVars := backendVars{
		SecretSuffix:    backend.SecretSuffix,
		InClusterConfig: backend.InClusterConfig,
		Namespace:       namespace,
	}
	var wr bytes.Buffer
	err = tmpl.Execute(&wr, templateVars)
	if err != nil {
		return "", err
	}
	return wr.String(), nil
}

func Interface2String(v interface{}) (string, error) {
	var value string
	switch v.(type) {
	case string:
		value = v.(string)
	case int:
		value = strconv.Itoa(v.(int))
	case bool:
		value = strconv.FormatBool(v.(bool))
	case []interface{}:
		var tmp string
		for _, i := range v.([]interface{}) {
			switch i.(type) {
			case string:
				tmp += fmt.Sprintf("\"%v\", ", i)
			case int, bool:
				tmp += fmt.Sprintf("%v, ", i)
			}
		}
		value = fmt.Sprintf(`'[%s]'`, tmp)
	default:
		return "", fmt.Errorf("could not convert %v to string", v)
	}
	return value, nil
}
