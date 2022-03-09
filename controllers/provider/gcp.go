package provider

import (
	"github.com/pkg/errors"
	"gopkg.in/yaml.v2"
	"k8s.io/klog/v2"
)

const (
	envGCPCredentialsJSON = "GOOGLE_CREDENTIALS"
	envGCPRegion          = "GOOGLE_REGION"
	envGCPProject         = "GOOGLE_PROJECT"
)

// GCPCredentials are credentials for GCP
type GCPCredentials struct {
	GCPCredentialsJSON string `yaml:"gcpCredentialsJSON"`
	GCPProject         string `yaml:"gcpProject"`
}

func getGCPCredentials(secretData []byte, name, namespace, region string) (map[string]string, error) {
	var ak GCPCredentials
	if err := yaml.Unmarshal(secretData, &ak); err != nil {
		klog.ErrorS(err, errConvertCredentials, "Name", name, "Namespace", namespace)
		return nil, errors.Wrap(err, errConvertCredentials)
	}
	return map[string]string{
		envGCPCredentialsJSON: ak.GCPCredentialsJSON,
		envGCPProject:         ak.GCPProject,
		envGCPRegion:          region,
	}, nil
}
