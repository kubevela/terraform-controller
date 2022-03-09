package provider

import (
	"github.com/pkg/errors"
	"gopkg.in/yaml.v2"
	"k8s.io/klog/v2"
)

const (
	envECApiKey = "EC_API_KEY"
)

// ECCredentials are credentials for Elastic CLoud
type ECCredentials struct {
	ECApiKey string `yaml:"ecApiKey"`
}

func getECCloudCredentials(secretData []byte, name, namespace string) (map[string]string, error) {
	var ak ECCredentials
	if err := yaml.Unmarshal(secretData, &ak); err != nil {
		klog.ErrorS(err, errConvertCredentials, "Name", name, "Namespace", namespace)
		return nil, errors.Wrap(err, errConvertCredentials)
	}
	return map[string]string{
		envECApiKey: ak.ECApiKey,
	}, nil
}
