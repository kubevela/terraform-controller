package provider

import (
	"github.com/pkg/errors"
	"gopkg.in/yaml.v2"
	"k8s.io/klog/v2"
)

const (
	envVSphereUser               = "VSPHERE_USER"
	envVSpherePassword           = "VSPHERE_PASSWORD"
	envVSphereServer             = "VSPHERE_SERVER"
	envVSphereAllowUnverifiedSSL = "VSPHERE_ALLOW_UNVERIFIED_SSL"
)

// VSphereCredentials are credentials for VSphere
type VSphereCredentials struct {
	VSphereUser               string `yaml:"vSphereUser"`
	VSpherePassword           string `yaml:"vSpherePassword"`
	VSphereServer             string `yaml:"vSphereServer"`
	VSphereAllowUnverifiedSSL string `yaml:"vSphereAllowUnverifiedSSL,omitempty"`
}

func getVSphereCredentials(secretData []byte, name, namespace string) (map[string]string, error) {
	var cred VSphereCredentials
	if err := yaml.Unmarshal(secretData, &cred); err != nil {
		klog.ErrorS(err, errConvertCredentials, "Name", name, "Namespace", namespace)
		return nil, errors.Wrap(err, errConvertCredentials)
	}
	return map[string]string{
		envVSphereUser:               cred.VSphereUser,
		envVSpherePassword:           cred.VSpherePassword,
		envVSphereServer:             cred.VSphereServer,
		envVSphereAllowUnverifiedSSL: cred.VSphereAllowUnverifiedSSL,
	}, nil
}
