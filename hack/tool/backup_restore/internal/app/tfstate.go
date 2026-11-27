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

package app

import (
	"bytes"
	"compress/gzip"
	"io"
	"os"
	"path/filepath"

	"github.com/oam-dev/terraform-controller/controllers/configuration/backend"
)

func getSecretName(k *backend.K8SBackend) string {
	return "tfstate-default-" + k.SecretSuffix
}

// compressedTFState returns the Terraform state file at path, gzip-compressed
// as the Kubernetes backend stores it in its state Secret.
func compressedTFState(path string) ([]byte, error) {
	state, err := os.Open(filepath.Clean(path))
	if err != nil {
		return nil, err
	}
	defer state.Close() //nolint:errcheck // read only

	var compressed bytes.Buffer
	writer := gzip.NewWriter(&compressed)
	if _, err := io.Copy(writer, state); err != nil {
		return nil, err
	}
	if err := writer.Close(); err != nil {
		return nil, err
	}
	return compressed.Bytes(), nil
}
