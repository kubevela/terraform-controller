package provider

import (
	"github.com/pkg/errors"
	"gopkg.in/yaml.v2"
	"k8s.io/klog/v2"
)

const (
	envQCloudSecretID  = "TENCENTCLOUD_SECRET_ID"
	envQCloudSecretKey = "TENCENTCLOUD_SECRET_KEY"
	envQCloudRegion    = "TENCENTCLOUD_REGION"
)

// TencentCloudCredentials are credentials for Tencent Cloud
type TencentCloudCredentials struct {
	SecretID  string `yaml:"secretID"`
	SecretKey string `yaml:"secretKey"`
}

func getTencentCloudCredentials(secretData []byte, name, namespace, region string) (map[string]string, error) {
	var ak TencentCloudCredentials
	if err := yaml.Unmarshal(secretData, &ak); err != nil {
		klog.ErrorS(err, errConvertCredentials, "Name", name, "Namespace", namespace)
		return nil, errors.Wrap(err, errConvertCredentials)
	}
	return map[string]string{
		envQCloudSecretID:  ak.SecretID,
		envQCloudSecretKey: ak.SecretKey,
		envQCloudRegion:    region,
	}, nil
}
