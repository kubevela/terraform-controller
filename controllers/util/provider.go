package util

import (
	"context"
	"fmt"

	"github.com/aliyun/alibaba-cloud-sdk-go/services/ram"
	"github.com/ghodss/yaml"
	"github.com/pkg/errors"
	v1 "k8s.io/api/core/v1"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/oam-dev/terraform-controller/api/types"
	"github.com/oam-dev/terraform-controller/api/v1beta1"
)

const (
	// ProviderDefaultName is the name of Provider object
	ProviderDefaultName = "default"
	// ProviderDefaultNamespace is the namespace of Provider object
	ProviderDefaultNamespace = "default"
)

// CloudProvider is a type for mark a Cloud Provider
type CloudProvider string

const (
	alibaba CloudProvider = "alibaba"
	aws     CloudProvider = "aws"
	gcp     CloudProvider = "gcp"
	azure   CloudProvider = "azure"
	vsphere CloudProvider = "vsphere"
	ec      CloudProvider = "ec"
	ucloud  CloudProvider = "ucloud"
)

const (
	envAlicloudAcessKey  = "ALICLOUD_ACCESS_KEY"
	envAlicloudSecretKey = "ALICLOUD_SECRET_KEY"
	envAlicloudRegion    = "ALICLOUD_REGION"
	envAliCloudStsToken  = "ALICLOUD_SECURITY_TOKEN"

	envUCloudPrivateKey = "UCLOUD_PRIVATE_KEY"
	envUCloudProjectID  = "UCLOUD_PROJECT_ID"
	envUCloudPublicKey  = "UCLOUD_PUBLIC_KEY"
	envUCloudRegion     = "UCLOUD_REGION"

	envAWSAccessKeyID     = "AWS_ACCESS_KEY_ID"
	envAWSSecretAccessKey = "AWS_SECRET_ACCESS_KEY"
	envAWSDefaultRegion   = "AWS_DEFAULT_REGION"
	envAWSSessionToken    = "AWS_SESSION_TOKEN"

	envGCPCredentialsJSON = "GOOGLE_CREDENTIALS"
	envGCPRegion          = "GOOGLE_REGION"
	envGCPProject         = "GOOGLE_PROJECT"

	envARMClientID       = "ARM_CLIENT_ID"
	envARMClientSecret   = "ARM_CLIENT_SECRET"
	envARMSubscriptionID = "ARM_SUBSCRIPTION_ID"
	envARMTenantID       = "ARM_TENANT_ID"

	envVSphereUser               = "VSPHERE_USER"
	envVSpherePassword           = "VSPHERE_PASSWORD"
	envVSphereServer             = "VSPHERE_SERVER"
	envVSphereAllowUnverifiedSSL = "VSPHERE_ALLOW_UNVERIFIED_SSL"

	errConvertCredentials = "failed to convert the credentials of Secret from Provider"
	errCredentialValid    = "Credentials are not valid"

	envECApiKey = "EC_API_KEY"
)

// AlibabaCloudCredentials are credentials for Alibaba Cloud
type AlibabaCloudCredentials struct {
	AccessKeyID     string `yaml:"accessKeyID"`
	AccessKeySecret string `yaml:"accessKeySecret"`
	SecurityToken   string `yaml:"securityToken"`
}

// UCloudCredentials are credentials for UCloud
type UCloudCredentials struct {
	PublicKey  string `yaml:"publicKey"`
	PrivateKey string `yaml:"privateKey"`
	Region     string `yaml:"region"`
	ProjectID  string `yaml:"projectID"`
}

// AWSCredentials are credentials for AWS
type AWSCredentials struct {
	AWSAccessKeyID     string `yaml:"awsAccessKeyID"`
	AWSSecretAccessKey string `yaml:"awsSecretAccessKey"`
	AWSSessionToken    string `yaml:"awsSessionToken"`
}

// GCPCredentials are credentials for GCP
type GCPCredentials struct {
	GCPCredentialsJSON string `yaml:"gcpCredentialsJSON"`
	GCPProject         string `yaml:"gcpProject"`
}

// AzureCredentials are credentials for Azure
type AzureCredentials struct {
	ARMClientID       string `yaml:"armClientID"`
	ARMClientSecret   string `yaml:"armClientSecret"`
	ARMSubscriptionID string `yaml:"armSubscriptionID"`
	ARMTenantID       string `yaml:"armTenantID"`
}

// VSphereCredentials are credentials for VSphere
type VSphereCredentials struct {
	VSphereUser               string `yaml:"vSphereUser"`
	VSpherePassword           string `yaml:"vSpherePassword"`
	VSphereServer             string `yaml:"vSphereServer"`
	VSphereAllowUnverifiedSSL string `yaml:"vSphereAllowUnverifiedSSL,omitempty"`
}

// ECCredentials are credentials for Elastic CLoud
type ECCredentials struct {
	ECApiKey string `yaml:"ecApiKey"`
}

// GetProviderCredentials gets provider credentials by cloud provider name
func GetProviderCredentials(ctx context.Context, k8sClient client.Client, providerNamespace, providerName string) (map[string]string, error) {
	provider, err := GetProviderFromConfiguration(ctx, k8sClient, providerNamespace, providerName)
	if err != nil {
		return nil, err
	}

	if provider.Status.State != types.ProviderIsReady {
		err := fmt.Errorf("provider is not ready: %s/%s", provider.Namespace, provider.Name)
		klog.ErrorS(err, "failed to get credential")
		return nil, err
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
		case string(alibaba):
			var ak AlibabaCloudCredentials
			if err := yaml.Unmarshal(secret.Data[secretRef.Key], &ak); err != nil {
				klog.ErrorS(err, errConvertCredentials, "Name", secretRef.Name, "Namespace", secretRef.Namespace)
				return nil, errors.Wrap(err, errConvertCredentials)
			}
			if err := checkAlibabaCloudCredentials(region, ak.AccessKeyID, ak.AccessKeySecret); err != nil {
				klog.ErrorS(err, errCredentialValid)
				return nil, errors.Wrap(err, errCredentialValid)
			}
			return map[string]string{
				envAlicloudAcessKey:  ak.AccessKeyID,
				envAlicloudSecretKey: ak.AccessKeySecret,
				envAlicloudRegion:    region,
				envAliCloudStsToken:  ak.SecurityToken,
			}, nil
		case string(ucloud):
			var ak UCloudCredentials
			if err := yaml.Unmarshal(secret.Data[secretRef.Key], &ak); err != nil {
				klog.ErrorS(err, errConvertCredentials, "Name", secretRef.Name, "Namespace", secretRef.Namespace)
				return nil, errors.Wrap(err, errConvertCredentials)
			}
			return map[string]string{
				envUCloudPublicKey:  ak.PublicKey,
				envUCloudPrivateKey: ak.PrivateKey,
				envUCloudRegion:     ak.Region,
				envUCloudProjectID:  ak.ProjectID,
			}, nil
		case string(aws):
			var ak AWSCredentials
			if err := yaml.Unmarshal(secret.Data[secretRef.Key], &ak); err != nil {
				klog.ErrorS(err, errConvertCredentials, "Name", secretRef.Name, "Namespace", secretRef.Namespace)
				return nil, errors.Wrap(err, errConvertCredentials)
			}
			return map[string]string{
				envAWSAccessKeyID:     ak.AWSAccessKeyID,
				envAWSSecretAccessKey: ak.AWSSecretAccessKey,
				envAWSSessionToken:    ak.AWSSessionToken,
				envAWSDefaultRegion:   region,
			}, nil
		case string(gcp):
			var ak GCPCredentials
			if err := yaml.Unmarshal(secret.Data[secretRef.Key], &ak); err != nil {
				klog.ErrorS(err, errConvertCredentials, "Name", secretRef.Name, "Namespace", secretRef.Namespace)
				return nil, errors.Wrap(err, errConvertCredentials)
			}
			return map[string]string{
				envGCPCredentialsJSON: ak.GCPCredentialsJSON,
				envGCPProject:         ak.GCPProject,
				envGCPRegion:          region,
			}, nil
		case string(azure):
			var cred AzureCredentials
			if err := yaml.Unmarshal(secret.Data[secretRef.Key], &cred); err != nil {
				klog.ErrorS(err, errConvertCredentials, "Name", secretRef.Name, "Namespace", secretRef.Namespace)
				return nil, errors.Wrap(err, errConvertCredentials)
			}
			return map[string]string{
				envARMClientID:       cred.ARMClientID,
				envARMClientSecret:   cred.ARMClientSecret,
				envARMSubscriptionID: cred.ARMSubscriptionID,
				envARMTenantID:       cred.ARMTenantID,
			}, nil
		case string(vsphere):
			var cred VSphereCredentials
			if err := yaml.Unmarshal(secret.Data[secretRef.Key], &cred); err != nil {
				klog.ErrorS(err, errConvertCredentials, "Name", secretRef.Name, "Namespace", secretRef.Namespace)
				return nil, errors.Wrap(err, errConvertCredentials)
			}
			return map[string]string{
				envVSphereUser:               cred.VSphereUser,
				envVSpherePassword:           cred.VSpherePassword,
				envVSphereServer:             cred.VSphereServer,
				envVSphereAllowUnverifiedSSL: cred.VSphereAllowUnverifiedSSL,
			}, nil
		case string(ec):
			var ak ECCredentials
			if err := yaml.Unmarshal(secret.Data[secretRef.Key], &ak); err != nil {
				klog.ErrorS(err, errConvertCredentials, "Name", secretRef.Name, "Namespace", secretRef.Namespace)
				return nil, errors.Wrap(err, errConvertCredentials)
			}
			return map[string]string{
				envECApiKey: ak.ECApiKey,
			}, nil
		default:
			errMsg := "unsupported provider"
			klog.ErrorS(err, errMsg, "Provider", provider.Spec.Provider)
			return nil, errors.Wrap(err, errMsg)
		}
	default:
		errMsg := "the credentials type is not supported."
		err := errors.New(errMsg)
		klog.ErrorS(err, "", "CredentialType", provider.Spec.Credentials.Source)
		return nil, err
	}
}

// ValidateProviderCredentials validates provider credentials by cloud provider name
func ValidateProviderCredentials(ctx context.Context, k8sClient client.Client, provider *v1beta1.Provider) error {
	switch provider.Spec.Credentials.Source {
	case "Secret":
		var secret v1.Secret
		secretRef := provider.Spec.Credentials.SecretRef
		if err := k8sClient.Get(ctx, client.ObjectKey{Name: secretRef.Name, Namespace: secretRef.Namespace}, &secret); err != nil {
			errMsg := "failed to get the Secret from Provider"
			klog.ErrorS(err, errMsg, "Name", secretRef.Name, "Namespace", secretRef.Namespace)
			return errors.Wrap(err, errMsg)
		}
	default:
		errMsg := "the credentials type is not supported."
		err := errors.New(errMsg)
		klog.ErrorS(err, "", "CredentialType", provider.Spec.Credentials.Source)
		return err
	}
	return nil
}

// GetProviderFromConfiguration gets provider object from Configuration
func GetProviderFromConfiguration(ctx context.Context, k8sClient client.Client, namespace, providerName string) (*v1beta1.Provider, error) {
	var provider = &v1beta1.Provider{}
	if err := k8sClient.Get(ctx, client.ObjectKey{Name: providerName, Namespace: namespace}, provider); err != nil {
		errMsg := "failed to get Provider object"
		klog.ErrorS(err, errMsg, "Name", providerName)
		return nil, errors.Wrap(err, errMsg)
	}
	return provider, nil
}

// checkAlibabaCloudProvider checks if the credentials from the provider are valid
func checkAlibabaCloudCredentials(region string, accessKeyID, accessKeySecret string) error {
	client, err := ram.NewClientWithAccessKey(region, accessKeyID, accessKeySecret)
	if err != nil {
		return err
	}
	request := ram.CreateGetPasswordPolicyRequest()
	request.Scheme = "https"

	_, err = client.GetPasswordPolicy(request)
	if err != nil {
		errMsg := "Alibaba Cloud credentials are not valid"
		klog.ErrorS(err, errMsg)
		return errors.Wrap(err, errMsg)
	}
	return nil
}
