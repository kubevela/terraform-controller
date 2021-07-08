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
	GCP     CloudProvider = "gcp"
	Azure   CloudProvider = "azure"
	VSphere CloudProvider = "vsphere"
)

const (
	EnvAlicloudAcessKey  = "ALICLOUD_ACCESS_KEY"
	EnvAlicloudSecretKey = "ALICLOUD_SECRET_KEY"
	EnvAlicloudRegion    = "ALICLOUD_REGION"
	EnvAliCloudStsToken  = "ALICLOUD_SECURITY_TOKEN"

	EnvAWSAccessKeyID     = "AWS_ACCESS_KEY_ID"
	EnvAWSSecretAccessKey = "AWS_SECRET_ACCESS_KEY"
	EnvAWSDefaultRegion   = "AWS_DEFAULT_REGION"

	EnvGCPCredentialsJSON = "GOOGLE_CREDENTIALS"
	EnvGCPRegion          = "GOOGLE_REGION"
	EnvGCPProject         = "GOOGLE_PROJECT"

	EnvARMClientID       = "ARM_CLIENT_ID"
	EnvARMClientSecret   = "ARM_CLIENT_SECRET"
	EnvARMSubscriptionID = "ARM_SUBSCRIPTION_ID"
	EnvARMTenantID       = "ARM_TENANT_ID"

	EnvVSphereUser               = "VSPHERE_USER"
	EnvVSpherePassword           = "VSPHERE_PASSWORD"
	EnvVSphereServer             = "VSPHERE_SERVER"
	EnvVSphereAllowUnverifiedSSL = "VSPHERE_ALLOW_UNVERIFIED_SSL"
	errConvertCredentials        = "failed to convert the credentials of Secret from Provider"
)

type AlibabaCloudCredentials struct {
	AccessKeyID     string `yaml:"accessKeyID"`
	AccessKeySecret string `yaml:"accessKeySecret"`
	SecurityToken   string `yaml:"securityToken"`
}

type AWSCredentials struct {
	AWSAccessKeyID     string `yaml:"awsAccessKeyID"`
	AWSSecretAccessKey string `yaml:"awsSecretAccessKey"`
}

type GCPCredentials struct {
	GCPCredentialsJSON string `yaml:"gcpCredentialsJSON"`
	GCPProject         string `yaml:"gcpProject"`
}

type AzureCredentials struct {
	ARMClientID       string `yaml:"armClientID"`
	ARMClientSecret   string `yaml:"armClientSecret"`
	ARMSubscriptionID string `yaml:"armSubscriptionID"`
	ARMTenantID       string `yaml:"armTenantID"`
}

type VSphereCredentials struct {
	VSphereUser               string `yaml:"vSphereUser"`
	VSpherePassword           string `yaml:"vSpherePassword"`
	VSphereServer             string `yaml:"vSphereServer"`
	VSphereAllowUnverifiedSSL string `yaml:"vSphereAllowUnverifiedSSL,omitempty"`
}

func GetProviderCredentials(ctx context.Context, k8sClient client.Client, namespace, providerName string) (map[string]string, error) {
	var provider v1beta1.Provider
	if providerName != "" {
		if err := k8sClient.Get(ctx, client.ObjectKey{Name: providerName, Namespace: namespace}, &provider); err != nil {
			errMsg := "failed to get Provider object"
			klog.ErrorS(err, errMsg, "Name", providerName)
			return nil, errors.Wrap(err, errMsg)
		}
	} else {
		if err := k8sClient.Get(ctx, client.ObjectKey{Name: ProviderName, Namespace: ProviderNamespace}, &provider); err != nil {
			errMsg := "failed to get Provider object"
			klog.ErrorS(err, errMsg, "Name", ProviderName)
			return nil, errors.Wrap(err, errMsg)
		}
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
				klog.ErrorS(err, errConvertCredentials, "Name", secretRef.Name, "Namespace", secretRef.Namespace)
				return nil, errors.Wrap(err, errConvertCredentials)
			}
			return map[string]string{
				EnvAlicloudAcessKey:  ak.AccessKeyID,
				EnvAlicloudSecretKey: ak.AccessKeySecret,
				EnvAlicloudRegion:    region,
				EnvAliCloudStsToken:  ak.SecurityToken,
			}, nil
		case string(AWS):
			var ak AWSCredentials
			if err := yaml.Unmarshal(secret.Data[secretRef.Key], &ak); err != nil {
				klog.ErrorS(err, errConvertCredentials, "Name", secretRef.Name, "Namespace", secretRef.Namespace)
				return nil, errors.Wrap(err, errConvertCredentials)
			}
			return map[string]string{
				EnvAWSAccessKeyID:     ak.AWSAccessKeyID,
				EnvAWSSecretAccessKey: ak.AWSSecretAccessKey,
				EnvAWSDefaultRegion:   region,
			}, nil
		case string(GCP):
			var ak GCPCredentials
			if err := yaml.Unmarshal(secret.Data[secretRef.Key], &ak); err != nil {
				klog.ErrorS(err, errConvertCredentials, "Name", secretRef.Name, "Namespace", secretRef.Namespace)
				return nil, errors.Wrap(err, errConvertCredentials)
			}
			return map[string]string{
				EnvGCPCredentialsJSON: ak.GCPCredentialsJSON,
				EnvGCPProject:         ak.GCPProject,
				EnvGCPRegion:          region,
			}, nil
		case string(Azure):
			var cred AzureCredentials
			if err := yaml.Unmarshal(secret.Data[secretRef.Key], &cred); err != nil {
				klog.ErrorS(err, errConvertCredentials, "Name", secretRef.Name, "Namespace", secretRef.Namespace)
				return nil, errors.Wrap(err, errConvertCredentials)
			}
			return map[string]string{
				EnvARMClientID:       cred.ARMClientID,
				EnvARMClientSecret:   cred.ARMClientSecret,
				EnvARMSubscriptionID: cred.ARMSubscriptionID,
				EnvARMTenantID:       cred.ARMTenantID,
			}, nil
		case string(VSphere):
			var cred VSphereCredentials
			if err := yaml.Unmarshal(secret.Data[secretRef.Key], &cred); err != nil {
				klog.ErrorS(err, errConvertCredentials, "Name", secretRef.Name, "Namespace", secretRef.Namespace)
				return nil, errors.Wrap(err, errConvertCredentials)
			}
			return map[string]string{
				EnvVSphereUser:               cred.VSphereUser,
				EnvVSpherePassword:           cred.VSpherePassword,
				EnvVSphereServer:             cred.VSphereServer,
				EnvVSphereAllowUnverifiedSSL: cred.VSphereAllowUnverifiedSSL,
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
