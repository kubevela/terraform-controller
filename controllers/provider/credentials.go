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
	tencent CloudProvider = "tencent"
	azure   CloudProvider = "azure"
	vsphere CloudProvider = "vsphere"
	ec      CloudProvider = "ec"
	ucloud  CloudProvider = "ucloud"
	custom  CloudProvider = "custom"
	baidu   CloudProvider = "baidu"
	huawei  CloudProvider = "huawei"
)

const (
	envAlicloudAcessKey  = "ALICLOUD_ACCESS_KEY"
	envAlicloudSecretKey = "ALICLOUD_SECRET_KEY"
	envAlicloudRegion    = "ALICLOUD_REGION"
	envAliCloudStsToken  = "ALICLOUD_SECURITY_TOKEN"

	errConvertCredentials     = "failed to convert the credentials of Secret from Provider"
	errCredentialValid        = "Credentials are not valid"
	ErrCredentialNotRetrieved = "Credentials are not retrieved from referenced Provider"
)

// AlibabaCloudCredentials are credentials for Alibaba Cloud
type AlibabaCloudCredentials struct {
	AccessKeyID     string `yaml:"accessKeyID" json:"accessKeyID,omitempty"`
	AccessKeySecret string `yaml:"accessKeySecret" json:"accessKeySecret,omitempty"`
	SecurityToken   string `yaml:"securityToken" json:"securityToken,omitempty"`
}

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
			return getUCloudCredentials(secretData, name, namespace, region)
		case string(aws):
			return getAWSCredentials(secretData, name, namespace, region)
		case string(gcp):
			return getGCPCredentials(secretData, name, namespace, region)
		case string(tencent):
			return getTencentCloudCredentials(secretData, name, namespace, region)
		case string(azure):
			return getAzureCredentials(secretData, name, namespace)
		case string(vsphere):
			return getVSphereCredentials(secretData, name, namespace)
		case string(ec):
			return getECCloudCredentials(secretData, name, namespace)
		case string(custom):
			return getCustomCredentials(secretData, name, namespace)
		case string(baidu):
			return getBaiduCloudCredentials(secretData, name, namespace, region)
		case string(huawei):
			return getHuaWeiCloudCredentials(secretData, name, namespace, region)
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
