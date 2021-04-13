package util

import (
	"context"

	"github.com/ghodss/yaml"
	"github.com/pkg/errors"
	v1 "k8s.io/api/core/v1"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/oam-dev/terraform-controller/api/v1beta1"
)

const (
	// ProviderName is the name of Provider object
	ProviderName = "default"
	// ProviderNamespace is the namespace of Provider object
	ProviderNamespace = "default"
)

type CloudProvider string

const (
	Alibaba CloudProvider = "alibaba"
	AWS     CloudProvider = "aws"
)

const (
	EnvAlicloudAcessKey  = "ALICLOUD_ACCESS_KEY"
	EnvAlicloudSecretKey = "ALICLOUD_SECRET_KEY"
	EnvAlicloudRegion    = "ALICLOUD_REGION"

	EnvAWSAccessKeyID     = "AWS_ACCESS_KEY_ID"
	EnvAWSSecretAccessKey = "AWS_SECRET_ACCESS_KEY"
	EnvAWSDefaultRegion   = "AWS_DEFAULT_REGION"
)

type AlibabaCloudCredentials struct {
	AccessKeyID     string `yaml:"accessKeyID"`
	AccessKeySecret string `yaml:"accessKeySecret"`
}

type AWSCredentials struct {
	AWSAccessKeyID     string `yaml:"awsAccessKeyID"`
	AWSSecretAccessKey string `yaml:"awsSecretAccessKey"`
}

func GetProviderCredentials(ctx context.Context, k8sClient client.Client) (map[string]string, error) {
	var provider v1beta1.Provider
	if err := k8sClient.Get(ctx, client.ObjectKey{Name: ProviderName, Namespace: ProviderNamespace}, &provider); err != nil {
		errMsg := "failed to get Provider object"
		klog.ErrorS(err, errMsg, "Name", ProviderName)
		return nil, errors.Wrap(err, errMsg)
	}

	region := provider.Spec.Region
	switch provider.Spec.Credentials.Source {
	case "Secret":
		var secret v1.Secret
		secretRef := provider.Spec.Credentials.SecretRef
		if err := k8sClient.Get(ctx, client.ObjectKey{Name: secretRef.Name, Namespace: secretRef.Namespace}, &secret); err != nil {
			errMsg := "failed to get the Secret from Provider"
			klog.ErrorS(err, errMsg, "Name", secretRef.Name, "Namespace", secretRef.Namespace)
			return nil, errors.Wrap(err, errMsg)
		}
		switch provider.Spec.Provider {
		case string(Alibaba):
			var ak AlibabaCloudCredentials
			if err := yaml.Unmarshal(secret.Data[secretRef.Key], &ak); err != nil {
				errMsg := "failed to convert the credentials of Secret from Provider"
				klog.ErrorS(err, errMsg, "Name", secretRef.Name, "Namespace", secretRef.Namespace)
				return nil, errors.Wrap(err, errMsg)
			}
			return map[string]string{
				EnvAlicloudAcessKey:  ak.AccessKeyID,
				EnvAlicloudSecretKey: ak.AccessKeySecret,
				EnvAlicloudRegion:    region,
			}, nil
		case string(AWS):
			var ak AWSCredentials
			if err := yaml.Unmarshal(secret.Data[secretRef.Key], &ak); err != nil {
				errMsg := "failed to convert the credentials of Secret from Provider"
				klog.ErrorS(err, errMsg, "Name", secretRef.Name, "Namespace", secretRef.Namespace)
				return nil, errors.Wrap(err, errMsg)
			}
			return map[string]string{
				EnvAWSAccessKeyID:     ak.AWSAccessKeyID,
				EnvAWSSecretAccessKey: ak.AWSSecretAccessKey,
				EnvAWSDefaultRegion:   region,
			}, nil

		}
	default:
		errMsg := "the credentials type is not supported."
		err := errors.New(errMsg)
		klog.ErrorS(err, "", "CredentialType", provider.Spec.Credentials.Source)
		return nil, err
	}
	return nil, nil
}
