package provider

import (
	"github.com/pkg/errors"
	"gopkg.in/yaml.v2"
	"k8s.io/klog/v2"
)

const (
	envHuaWeiCloudRegion    = "HW_REGION_NAME"
	envHuaWeiCloudAccessKey = "HW_ACCESS_KEY"
	envHuaWeiCloudSecretKey = "HW_SECRET_KEY"
)

// HuaWeiCloudCredentials are credentials for Huawei Cloud
type HuaWeiCloudCredentials struct {
	AccessKey string `yaml:"accessKey"`
	SecretKey string `yaml:"secretKey"`
}

func getHuaWeiCloudCredentials(secretData []byte, name, namespace, region string) (map[string]string, error) {
	var hwc HuaWeiCloudCredentials
	if err := yaml.Unmarshal(secretData, &hwc); err != nil {
		klog.ErrorS(err, errConvertCredentials, "Name", name, "Namespace", namespace)
		return nil, errors.Wrap(err, errConvertCredentials)
	}
	return map[string]string{
		envHuaWeiCloudAccessKey: hwc.AccessKey,
		envHuaWeiCloudSecretKey: hwc.SecretKey,
		envHuaWeiCloudRegion:    region,
	}, nil
}
