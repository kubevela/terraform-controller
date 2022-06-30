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
	"os"

	"github.com/oam-dev/terraform-controller/controllers/configuration/backend"
)

func getSecretName(k *backend.K8SBackend) string {
	return "tfstate-default-" + k.SecretSuffix
}

// Modified based on Hashicorp code base https://github.com/hashicorp/terraform/blob/fabdf0bea1fa2bf6a9d56cc3ea0f28242bf5e812/backend/remote-state/kubernetes/client.go#L343
// Licensed under Mozilla Public License 2.0
func compressedTFState(path string) ([]byte, error) {
	srcBytes, err := os.ReadFile(path)
	var buf bytes.Buffer
	writer := gzip.NewWriter(&buf)
	_, err = writer.Write(srcBytes)
	if err != nil {
		return nil, err
	}
	if err := writer.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
