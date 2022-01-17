package util

import (
	"encoding/base64"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDecompressTerraformStateSecret(t *testing.T) {
	type args struct {
		data string
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
				data: "H4sIAAAAAAAA/0SMwa7CIBBF9/0KMutH80ArDb9ijKHDYEhqMQO4afrvBly4POfc3H0QAt7EOaYNrDj/NS7E7ELi5/1XQI3/o4beM3F0K1ihO65xI/egNsLThLPRWi6agkR/CVIppaSZJrfgbBx6//1ItbxqyWDFfnTBlFNlpKaut+EYPgEAAP//xUXpvZsAAAA=",
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
	}

	for _, tt := range testcases {
		t.Run(tt.name, func(t *testing.T) {
			state, err := base64.StdEncoding.DecodeString(tt.args.data)
			assert.NoError(t, err)
			got, err := DecompressTerraformStateSecret(string(state))
			if tt.want.errMsg != "" {
				assert.Contains(t, err.Error(), tt.want.errMsg)
			} else {
				assert.Equal(t, tt.want.raw, string(got))
			}
		})
	}
}
