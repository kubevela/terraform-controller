package provider

import (
	"github.com/pkg/errors"
	"gopkg.in/yaml.v2"
	"k8s.io/klog/v2"
)

const (
	envARMClientID       = "ARM_CLIENT_ID"
	envARMClientSecret   = "ARM_CLIENT_SECRET"
	envARMSubscriptionID = "ARM_SUBSCRIPTION_ID"
	envARMTenantID       = "ARM_TENANT_ID"
)

// AzureCredentials are credentials for Azure
type AzureCredentials struct {
	ARMClientID       string `yaml:"armClientID"`
	ARMClientSecret   string `yaml:"armClientSecret"`
	ARMSubscriptionID string `yaml:"armSubscriptionID"`
	ARMTenantID       string `yaml:"armTenantID"`
}

func getAzureCredentials(secretData []byte, name, namespace string) (map[string]string, error) {
	var cred AzureCredentials
	if err := yaml.Unmarshal(secretData, &cred); err != nil {
		klog.ErrorS(err, errConvertCredentials, "Name", name, "Namespace", namespace)
		return nil, errors.Wrap(err, errConvertCredentials)
	}
	return map[string]string{
		envARMClientID:       cred.ARMClientID,
		envARMClientSecret:   cred.ARMClientSecret,
		envARMSubscriptionID: cred.ARMSubscriptionID,
		envARMTenantID:       cred.ARMTenantID,
	}, nil
}
