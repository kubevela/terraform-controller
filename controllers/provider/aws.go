package provider

import (
	"os"

	"github.com/pkg/errors"
	"gopkg.in/yaml.v2"
	"k8s.io/klog/v2"
)

const (
	envAWSAccessKeyID          = "AWS_ACCESS_KEY_ID"
	envAWSSecretAccessKey      = "AWS_SECRET_ACCESS_KEY"
	envAWSDefaultRegion        = "AWS_DEFAULT_REGION"
	envAWSProvider             = "AWS_PROVIDER"
	envAWSSessionToken         = "AWS_SESSION_TOKEN"
	envAWSRoleArn              = "AWS_ROLE_ARN"
	envAWSWebIdentityTokenFile = "AWS_WEB_IDENTITY_TOKEN_FILE"
)

// AWSCredentials are credentials for AWS
type AWSCredentials struct {
	AWSAccessKeyID     string `yaml:"awsAccessKeyID"`
	AWSSecretAccessKey string `yaml:"awsSecretAccessKey"`
	AWSSessionToken    string `yaml:"awsSessionToken"`
}

type AWSCredentialsInjected struct {
	AWSRoleArn              string `env:"AWS_ROLE_ARN"`
	AWSWebIdentityTokenFile string `env:"AWS_WEB_IDENTITY_TOKEN_FILE"`
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

func getAWSCredentialsInjectedIdentity(region string) (map[string]string, error) {
	//var ak AWSCredentialsInjected
	// opts := &ak.Options{Environment: map[string]string{
	// 	"AWS_ROLE_ARN":                "arn:aws:iam::001122334455:role/terrafrom-controller-sa",
	// 	"AWS_WEB_IDENTITY_TOKEN_FILE": "/var/run/secrets/eks.amazonaws.com/serviceaccount/token",
	// }}
	arn := os.Getenv("AWS_ROLE_ARN")
	token := os.Getenv("AWS_WEB_IDENTITY_TOKEN_FILE")

	return map[string]string{
		// envAWSRoleArn:              ak.AWSRoleArn,              //os.Getenv("AWS_ROLE_ARN"),
		// envAWSWebIdentityTokenFile: ak.AWSWebIdentityTokenFile, //os.Getenv("AWS_WEB_IDENTITY_TOKEN_FILE"),
		envAWSRoleArn:              arn,
		envAWSWebIdentityTokenFile: token,
		envAWSDefaultRegion:        region,
	}, nil
}
