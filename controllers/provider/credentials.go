package provider

import (
	"context"

	"github.com/aliyun/alibaba-cloud-sdk-go/services/sts"
	"github.com/ghodss/yaml"
	"github.com/pkg/errors"
	v1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/oam-dev/terraform-controller/api/v1beta1"
)

const (
	// DefaultName is the name of Provider object
	DefaultName = "default"
	// DefaultNamespace is the namespace of Provider object
	DefaultNamespace = "default"
)

// CloudProvider is a type for mark a Cloud Provider
type CloudProvider string

const (
	alibaba CloudProvider = "alibaba"
	aws     CloudProvider = "aws"
	gcp     CloudProvider = "gcp"
	qcloud  CloudProvider = "tencent"
	azure   CloudProvider = "azure"
	vsphere CloudProvider = "vsphere"
	ec      CloudProvider = "ec"
	ucloud  CloudProvider = "ucloud"
	custom  CloudProvider = "custom"
	baidu   CloudProvider = "baidu"
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

	envQCloudSecretID  = "TENCENTCLOUD_SECRET_ID"
	envQCloudSecretKey = "TENCENTCLOUD_SECRET_KEY"
	envQCloudRegion    = "TENCENTCLOUD_REGION"

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

// TencentCloudCredentials are credentials for Tencent Cloud
type TencentCloudCredentials struct {
	SecretID  string `yaml:"secretID"`
	SecretKey string `yaml:"secretKey"`
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

// CustomCredentials are credentials for custom (you self)
type CustomCredentials map[string]string

// GetProviderCredentials gets provider credentials by cloud provider name
func GetProviderCredentials(ctx context.Context, k8sClient client.Client, provider *v1beta1.Provider, region string) (map[string]string, error) {
	switch provider.Spec.Credentials.Source {
	case "Secret":
		var secret v1.Secret
		secretRef := provider.Spec.Credentials.SecretRef
		name := secretRef.Name
		namespace := secretRef.Namespace
		if err := k8sClient.Get(ctx, client.ObjectKey{Name: name, Namespace: namespace}, &secret); err != nil {
			errMsg := "failed to get the Secret from Provider"
			klog.ErrorS(err, errMsg, "Name", name, "Namespace", namespace)
			return nil, errors.Wrap(err, errMsg)
		}
		secretData, ok := secret.Data[secretRef.Key]
		if !ok {
			return nil, errors.Errorf("in the provider %s, the key %s not found in the referenced secret %s", provider.Name, secretRef.Key, name)
		}
		switch provider.Spec.Provider {
		case string(alibaba):
			var ak AlibabaCloudCredentials
			if err := yaml.Unmarshal(secret.Data[secretRef.Key], &ak); err != nil {
				klog.ErrorS(err, errConvertCredentials, "Name", name, "Namespace", namespace)
				return nil, errors.Wrap(err, errConvertCredentials)
			}
			if err := checkAlibabaCloudCredentials(region, ak.AccessKeyID, ak.AccessKeySecret, ak.SecurityToken); err != nil {
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
			if err := yaml.Unmarshal(secretData, &ak); err != nil {
				klog.ErrorS(err, errConvertCredentials, "Name", name, "Namespace", namespace)
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
		case string(gcp):
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
		case string(qcloud):
			var cred TencentCloudCredentials
			if err := yaml.Unmarshal(secretData, &cred); err != nil {
				klog.ErrorS(err, errConvertCredentials, "Name", name, "Namespace", namespace)
				return nil, errors.Wrap(err, errConvertCredentials)
			}
			return map[string]string{
				envQCloudSecretID:  cred.SecretID,
				envQCloudSecretKey: cred.SecretKey,
				envQCloudRegion:    region,
			}, nil
		case string(azure):
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
		case string(vsphere):
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
		case string(ec):
			var ak ECCredentials
			if err := yaml.Unmarshal(secretData, &ak); err != nil {
				klog.ErrorS(err, errConvertCredentials, "Name", name, "Namespace", namespace)
				return nil, errors.Wrap(err, errConvertCredentials)
			}
			return map[string]string{
				envECApiKey: ak.ECApiKey,
			}, nil
		case string(custom):
			var ck = make(CustomCredentials)
			if err := yaml.Unmarshal(secretData, &ck); err != nil {
				klog.ErrorS(err, errConvertCredentials, "Name", name, "Namespace", namespace)
				return nil, errors.Wrap(err, errConvertCredentials)
			}
			return ck, nil
		case string(baidu):
			return getBaiduCloudCredentials(secretData, name, namespace, region)
		default:
			errMsg := "unsupported provider"
			klog.InfoS(errMsg, "Provider", provider.Spec.Provider)
			return nil, errors.New(errMsg)
		}
	default:
		errMsg := "the credentials type is not supported."
		err := errors.New(errMsg)
		klog.ErrorS(err, "", "CredentialType", provider.Spec.Credentials.Source)
		return nil, err
	}
}

// GetProviderFromConfiguration gets provider object from Configuration
// Returns:
// 1) (nil, err): hit an issue to find the provider
// 2) (nil, nil): provider not found
// 3) (provider, nil): provider found
func GetProviderFromConfiguration(ctx context.Context, k8sClient client.Client, namespace, name string) (*v1beta1.Provider, error) {
	var provider = &v1beta1.Provider{}
	if err := k8sClient.Get(ctx, client.ObjectKey{Name: name, Namespace: namespace}, provider); err != nil {
		if kerrors.IsNotFound(err) {
			return nil, nil
		}
		errMsg := "failed to get Provider object"
		klog.ErrorS(err, errMsg, "Name", name)
		return nil, errors.Wrap(err, errMsg)
	}
	return provider, nil
}

// checkAlibabaCloudProvider checks if the credentials from the provider are valid
func checkAlibabaCloudCredentials(region string, accessKeyID, accessKeySecret, stsToken string) error {
	var (
		client *sts.Client
		err    error
	)
	if stsToken != "" {
		client, err = sts.NewClientWithStsToken(region, accessKeyID, accessKeySecret, stsToken)
	} else {
		client, err = sts.NewClientWithAccessKey(region, accessKeyID, accessKeySecret)
	}
	if err != nil {
		return err
	}
	request := sts.CreateGetCallerIdentityRequest()
	request.Scheme = "https"

	_, err = client.GetCallerIdentity(request)
	if err != nil {
		errMsg := "Alibaba Cloud credentials are invalid"
		klog.ErrorS(err, errMsg)
		return errors.Wrap(err, errMsg)
	}
	return nil
}
