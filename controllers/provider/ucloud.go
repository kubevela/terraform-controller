package provider

import (
	"github.com/pkg/errors"
	"gopkg.in/yaml.v2"
	"k8s.io/klog/v2"
)

const (
	envUCloudPrivateKey = "UCLOUD_PRIVATE_KEY"
	envUCloudProjectID  = "UCLOUD_PROJECT_ID"
	envUCloudPublicKey  = "UCLOUD_PUBLIC_KEY"
	envUCloudRegion     = "UCLOUD_REGION"
)

// UCloudCredentials are credentials for UCloud
type UCloudCredentials struct {
	PublicKey  string `yaml:"publicKey"`
	PrivateKey string `yaml:"privateKey"`
	Region     string `yaml:"region"`
	ProjectID  string `yaml:"projectID"`
}

func getUCloudCredentials(secretData []byte, name, namespace string) (map[string]string, error) {
	var ak UCloudCredentials
	if err := yaml.Unmarshal(secretData, &ak); err != nil {
		klog.ErrorS(err, errConvertCredentials, "Name", name, "Namespace", namespace)
		return nil, errors.Wrap(err, errConvertCredentials)
	}
	return map[string]string{
		envUCloudPublicKey:  ak.PublicKey,
		envUCloudPrivateKey: ak.PrivateKey,
		envUCloudRegion:     ak.Region,
		envUCloudProjectID:  ak.ProjectID,
	}, nil
}
