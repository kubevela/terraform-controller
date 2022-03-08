package provider

import (
	"github.com/pkg/errors"
	"gopkg.in/yaml.v2"
	"k8s.io/klog/v2"
)

const (
	envAWSAccessKeyID     = "AWS_ACCESS_KEY_ID"
	envAWSSecretAccessKey = "AWS_SECRET_ACCESS_KEY"
	envAWSDefaultRegion   = "AWS_DEFAULT_REGION"
	envAWSSessionToken    = "AWS_SESSION_TOKEN"
)

// AWSCredentials are credentials for AWS
type AWSCredentials struct {
	AWSAccessKeyID     string `yaml:"awsAccessKeyID"`
	AWSSecretAccessKey string `yaml:"awsSecretAccessKey"`
	AWSSessionToken    string `yaml:"awsSessionToken"`
}

func getAWSCredentials(secretData []byte, name, namespace, region string) (map[string]string, error) {
	var ak AWSCredentials
	if err := yaml.Unmarshal(secretData, &ak); err != nil {
		klog.ErrorS(err, errConvertCredentials, "Name", name, "Namespace", namespace)
		return nil, errors.Wrap(err, errConvertCredentials)
	}
	return map[string]string{
		envAWSAccessKeyID:     ak.AWSAccessKeyID,
		envAWSSecretAccessKey: ak.AWSSecretAccessKey,
		envAWSSessionToken:    ak.AWSSessionToken,
		envAWSDefaultRegion:   region,
	}, nil
}
