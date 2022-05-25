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
	"encoding/json"
	"fmt"
	"strconv"

	"k8s.io/apimachinery/pkg/runtime"
)

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
	return ret, nil
}

// Interface2String converts an interface{} type to string
func Interface2String(v interface{}) (string, error) {
	var value string
	switch v := v.(type) {
	case string:
		value = v
	case int:
		value = strconv.Itoa(v)
	case float64:
		value = fmt.Sprint(v)
	case bool:
		value = strconv.FormatBool(v)
	default:
		valueJSON, err := json.Marshal(v)
		if err != nil {
			return "", fmt.Errorf("cloud not convert %v to string", v)
		}
		value = string(valueJSON)
	}
	return value, nil
}
