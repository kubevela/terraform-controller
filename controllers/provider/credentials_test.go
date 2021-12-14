package provider

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestCheckAlibabaCloudCredentials(t *testing.T) {
	type credentials struct {
		AccessKeyID     string
		AccessKeySecret string
		SecurityToken   string
		Region          string
	}

	type args struct {
		credentials credentials
	}
	tests := []struct {
		name string
		args args
	}{
		{
			name: "Check AlibabaCloud credentials",
			args: args{
				credentials: credentials{
					AccessKeyID:     "aaaa",
					AccessKeySecret: "bbbbb",
					Region:          "cn-hangzhou",
				},
			},
		},
		{
			name: "Check AlibabaCloud credentials with sts token",
			args: args{
				credentials: credentials{
					AccessKeyID:     "STS.aaaa",
					AccessKeySecret: "bbbbb",
					SecurityToken:   "ccc",
					Region:          "cn-hangzhou",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cred := tt.args.credentials
			err := checkAlibabaCloudCredentials(cred.Region, cred.AccessKeyID, cred.AccessKeySecret, cred.SecurityToken)
			assert.NotNil(t, err)
		})
	}

}
