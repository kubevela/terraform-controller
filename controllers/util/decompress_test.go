package util

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDecompressTerraformStateSecret(t *testing.T) {
	type args struct {
		data       string
		needDecode bool
	}
	type want struct {
		raw    string
		errMsg string
	}
	testcases := []struct {
		name string
		args args
		want want
	}{
		{
			name: "decompress terraform state secret",
			args: args{
				data:       "H4sIAAAAAAAA/0SMwa7CIBBF9/0KMutH80ArDb9ijKHDYEhqMQO4afrvBly4POfc3H0QAt7EOaYNrDj/NS7E7ELi5/1XQI3/o4beM3F0K1ihO65xI/egNsLThLPRWi6agkR/CVIppaSZJrfgbBx6//1ItbxqyWDFfnTBlFNlpKaut+EYPgEAAP//xUXpvZsAAAA=",
				needDecode: true,
			},
			want: want{
				raw: `{
  "version": 4,
  "terraform_version": "1.0.2",
  "serial": 2,
  "lineage": "c35c8722-b2ef-cd6f-1111-755abc87acdd",
  "outputs": {},
  "resources": []
}
`,
			},
		},
		{
			name: "bad data",
			args: args{
				data: "abc",
			},
			want: want{
				errMsg: "EOF",
			},
		},
	}

	for _, tt := range testcases {
		t.Run(tt.name, func(t *testing.T) {
			if tt.args.needDecode {
				state, err := base64.StdEncoding.DecodeString(tt.args.data)
				assert.NoError(t, err)
				tt.args.data = string(state)
			}
			got, err := DecompressTerraformStateSecret(tt.args.data)
			if tt.want.errMsg != "" || err != nil {
				assert.Contains(t, err.Error(), tt.want.errMsg)
			} else {
				assert.Equal(t, tt.want.raw, string(got))
			}
		})
	}
}

// gzipped compresses state with the standard library, as Terraform's
// Kubernetes backend does before storing it.
func gzipped(t *testing.T, state string) []byte {
	var compressed bytes.Buffer
	writer := gzip.NewWriter(&compressed)
	_, err := writer.Write([]byte(state))
	assert.NoError(t, err)
	assert.NoError(t, writer.Close())
	return compressed.Bytes()
}

func TestDecompressTerraformStateSecretRoundTrip(t *testing.T) {
	for name, state := range map[string]string{
		"a state":        `{"version": 4, "serial": 7, "resources": []}`,
		"an empty state": "",
	} {
		t.Run(name, func(t *testing.T) {
			got, err := DecompressTerraformStateSecret(string(gzipped(t, state)))
			assert.NoError(t, err)
			assert.Equal(t, state, string(got))
		})
	}
}

func TestDecompressTerraformStateSecretRejectsATruncatedSecret(t *testing.T) {
	compressed := gzipped(t, `{"version": 4, "serial": 7, "resources": []}`)

	_, err := DecompressTerraformStateSecret(string(compressed[:len(compressed)-6]))

	assert.Error(t, err)
}
