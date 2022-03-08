package provider

import (
	"github.com/pkg/errors"
	"gopkg.in/yaml.v2"
	"k8s.io/klog/v2"
)

const (
	envBaiduAccessKey = "BAIDUCLOUD_ACCESS_KEY"
	envBaiduSecretKey = "BAIDUCLOUD_SECRET_KEY"
	envBaiduRegion    = "BAIDUCLOUD_REGION"
)

// BaiduCloudCredentials are credentials for Baidu Cloud
type BaiduCloudCredentials struct {
	KeyBaiduAccessKey string `yaml:"accessKey"`
	KeyBaiduSecretKey string `yaml:"secretKey"`
}

func getBaiduCloudCredentials(secretData []byte, name, namespace, region string) (map[string]string, error) {
	var ak BaiduCloudCredentials
	if err := yaml.Unmarshal(secretData, &ak); err != nil {
		klog.ErrorS(err, errConvertCredentials, "Name", name, "Namespace", namespace)
		return nil, errors.Wrap(err, errConvertCredentials)
	}
	return map[string]string{
		envBaiduAccessKey: ak.KeyBaiduAccessKey,
		envBaiduSecretKey: ak.KeyBaiduSecretKey,
		envBaiduRegion:    region,
	}, nil
}
