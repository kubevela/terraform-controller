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

package util

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/tmccombs/hcl2json/convert"
)

func TestConfData_GetOk(t *testing.T) {

	tests := []struct {
		name  string
		hcl   string
		key   string
		want  interface{}
		want1 bool
	}{
		{
			name:  "simple string",
			hcl:   `name="a"`,
			key:   "name",
			want:  "a",
			want1: true,
		},
		{
			name:  "simple bool",
			hcl:   `check=false`,
			key:   "check",
			want:  false,
			want1: true,
		},
		{
			name:  "simple int",
			hcl:   `check=1`,
			key:   "check",
			want:  float64(1),
			want1: true,
		},
		{
			name:  "not ok",
			hcl:   `check=1`,
			key:   "check2",
			want:  nil,
			want1: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			confValue, _ := convert.Bytes([]byte(tt.hcl), "backend", convert.Options{})
			confData := ConfData(make(map[string]interface{}))
			_ = json.Unmarshal(confValue, &confData)
			got, got1 := confData.GetOk(tt.key)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GetOk() got = %v, want %v", got, tt.want)
			}
			if got1 != tt.want1 {
				t.Errorf("GetOk() got1 = %v, want %v", got1, tt.want1)
			}
		})
	}
}

func TestConfData_Get(t *testing.T) {

	tests := []struct {
		name string
		hcl  string
		key  string
		want interface{}
	}{
		{
			name: "simple string",
			hcl:  `name="a"`,
			key:  "name",
			want: "a",
		},
		{
			name: "simple bool",
			hcl:  `check=false`,
			key:  "check",
			want: false,
		},
		{
			name: "simple bool, missing",
			hcl:  `check=false`,
			key:  "check1",
			want: nil,
		},
		{
			name: "simple int",
			hcl:  `check=1`,
			key:  "check",
			want: float64(1),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			confValue, _ := convert.Bytes([]byte(tt.hcl), "backend", convert.Options{})
			confData := ConfData(make(map[string]interface{}))
			_ = json.Unmarshal(confValue, &confData)
			got := confData.Get(tt.key)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GetOk() got = %v, want %v", got, tt.want)
			}
		})
	}
}
