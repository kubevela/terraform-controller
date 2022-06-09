package provider

import (
	"github.com/pkg/errors"
	"gopkg.in/yaml.v2"
	"k8s.io/klog/v2"
)

const (
	// EnvAWSAccessKeyID is the name of the AWS_ACCESS_KEY_ID env
	EnvAWSAccessKeyID = "AWS_ACCESS_KEY_ID"
	// EnvAWSSecretAccessKey is the name of the AWS_SECRET_ACCESS_KEY env
	EnvAWSSecretAccessKey = "AWS_SECRET_ACCESS_KEY"
	// EnvAWSDefaultRegion is the name of the AWS_DEFAULT_REGION env
	EnvAWSDefaultRegion = "AWS_DEFAULT_REGION"
	// EnvAWSSessionToken is the name of the AWS_SESSION_TOKEN env
	EnvAWSSessionToken = "AWS_SESSION_TOKEN"
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
		EnvAWSAccessKeyID:     ak.AWSAccessKeyID,
		EnvAWSSecretAccessKey: ak.AWSSecretAccessKey,
		EnvAWSSessionToken:    ak.AWSSessionToken,
		EnvAWSDefaultRegion:   region,
	}, nil
}
