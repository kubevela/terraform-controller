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

// ConfData contains all the attributes in the backend block of the hcl code
type ConfData map[string]interface{}

// GetOk returns (value, true) if the ConfData has the entry for key
// Otherwise, it returns (nil, false)
func (d ConfData) GetOk(key string) (interface{}, bool) {
	v, ok := d[key]
	return v, ok
}

// Get returns the value if the ConfData has the entry for key
// Otherwise, it returns nil
func (d ConfData) Get(key string) interface{} {
	return d[key]
}
