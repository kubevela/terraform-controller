/*
Copyright 2026 The KubeVela Authors.

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
	"compress/gzip"
	"io"
	"strings"
)

// DecompressTerraformStateSecret returns the Terraform state held in data, the
// gzip-compressed value of a Kubernetes backend's state Secret.
func DecompressTerraformStateSecret(data string) ([]byte, error) {
	reader, err := gzip.NewReader(strings.NewReader(data))
	if err != nil {
		return nil, err
	}
	state, err := io.ReadAll(reader)
	if err != nil {
		return nil, err
	}
	if err := reader.Close(); err != nil {
		return nil, err
	}
	return state, nil
}
