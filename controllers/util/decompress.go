package util

import (
	"bytes"
	"compress/gzip"
)

// DecompressTerraformStateSecret decompress the data of Terraform backend state secret
// Modified based on Hashicorp code base https://github.com/hashicorp/terraform/blob/fabdf0bea1fa2bf6a9d56cc3ea0f28242bf5e812/backend/remote-state/kubernetes/client.go#L355
// Licensed under Mozilla Public License 2.0
func DecompressTerraformStateSecret(data string) ([]byte, error) {
	b := new(bytes.Buffer)
	gz, err := gzip.NewReader(bytes.NewReader([]byte(data)))
	if err != nil {
		return nil, err
	}
	if _, err := b.ReadFrom(gz); err != nil {
		return nil, err
	}
	if err := gz.Close(); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}
