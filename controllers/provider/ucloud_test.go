package provider

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetUCloudCredentials(t *testing.T) {
	tests := []struct {
		name       string
		ns         string
		secretData []byte
		region     string
		wanted     map[string]string
	}{
		{
			name:       "ucloud1",
			ns:         "qa",
			region:     "cn-bj2",
			secretData: []byte("publicKey: xxxx1\nprivateKey: xxxx2\nregion: test1\nprojectID: test1"),
			wanted: map[string]string{
				envUCloudPrivateKey: "xxxx2",
				envUCloudProjectID:  "test1",
				envUCloudPublicKey:  "xxxx1",
				envUCloudRegion:     "cn-bj2",
			},
		},
		{
			name:       "ucloud1",
			ns:         "qa",
			region:     "",
			secretData: []byte("publicKey: xxxx1\nprivateKey: xxxx2\nregion: test1\nprojectID: test1"),
			wanted: map[string]string{
				envUCloudPrivateKey: "xxxx2",
				envUCloudProjectID:  "test1",
				envUCloudPublicKey:  "xxxx1",
				envUCloudRegion:     "",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m, err := getUCloudCredentials(tt.secretData, tt.name, tt.ns, tt.region)
			assert.Nil(t, err)
			if !reflect.DeepEqual(tt.wanted, m) {
				t.Errorf("getUCloudCredentials got = %v, wanted %v", m, tt.wanted)
			}
		})
	}
}
