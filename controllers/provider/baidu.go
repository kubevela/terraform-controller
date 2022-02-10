package provider

import (
	"github.com/ghodss/yaml"
	"github.com/pkg/errors"
	v1 "k8s.io/api/core/v1"
	"k8s.io/klog/v2"

	crossplanetypes "github.com/oam-dev/terraform-controller/api/types/crossplane-runtime"
)

const baidu CloudProvider = "baidu"

const (
	envBaiduAccessKey = "BAIDUCLOUD_ACCESS_KEY"
	envBaiduSecretKey = "BAIDUCLOUD_SECRET_KEY"
	envBaiduRegion    = "BAIDUCLOUD_REGION"
)

// BaiduCloudCredentials are credentials for Baidu Cloud
type BaiduCloudCredentials struct {
	KeyBaiduAccessKey string `yaml:"baiducloud_access_key"`
	KeyBaiduSecretKey string `yaml:"baiducloud_secret_key"`
}

func getBaiduCloudCredentials(secret v1.Secret, secretRef *crossplanetypes.SecretKeySelector, region string) (map[string]string, error) {
	var ak BaiduCloudCredentials
	if err := yaml.Unmarshal(secret.Data[secretRef.Key], &ak); err != nil {
		klog.ErrorS(err, errConvertCredentials, "Name", secretRef.Name, "Namespace", secretRef.Namespace)
		return nil, errors.Wrap(err, errConvertCredentials)
	}
	return map[string]string{
		envBaiduAccessKey: ak.KeyBaiduAccessKey,
		envBaiduSecretKey: ak.KeyBaiduSecretKey,
		envBaiduRegion:    region,
	}, nil
}
