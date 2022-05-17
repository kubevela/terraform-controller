/*
Copyright 2021 The KubeVela Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1beta2

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	state "github.com/oam-dev/terraform-controller/api/types"
	types "github.com/oam-dev/terraform-controller/api/types/crossplane-runtime"
)

// ConfigurationSpec defines the desired state of Configuration
type ConfigurationSpec struct {
	// HCL is the Terraform HCL type configuration
	HCL string `json:"hcl,omitempty"`

	// Remote is a git repo which contains hcl files. Currently, only public git repos are supported.
	Remote string `json:"remote,omitempty"`

	// +kubebuilder:pruning:PreserveUnknownFields
	Variable *runtime.RawExtension `json:"variable,omitempty"`

	// Backend describes the Terraform backend configuration.
	// This field is needed if the users use a git repo to provide the hcl files and
	// want to use their custom Terraform backend (instead of the default kubernetes backend type).
	// Notice: the content in this field will **override** the backend configuration in the inline hcl code or
	// in the hcl files in the git repo.
	Backend *Backend `json:"backend,omitempty"`

	// Path is the sub-directory of remote git repository.
	Path string `json:"path,omitempty"`

	// WriteConnectionSecretToReference specifies the namespace and name of a
	// Secret to which any connection details for this managed resource should
	// be written. Connection details frequently include the endpoint, username,
	// and password required to connect to the managed resource.
	// +optional
	WriteConnectionSecretToReference *types.SecretReference `json:"writeConnectionSecretToRef,omitempty"`

	// ProviderReference specifies the reference to Provider
	ProviderReference *types.Reference `json:"providerRef,omitempty"`

	// InlineCredentials specifies the credentials in spec.HCl field as below.
	//	provider "aws" {
	//		region     = "us-west-2"
	//		access_key = "my-access-key"
	//		secret_key = "my-secret-key"
	//	}
	// Or indicates a Terraform module or configuration don't need credentials at all, like provider `random`
	InlineCredentials bool `json:"inlineCredentials,omitempty"`

	// DeleteResource will determine whether provisioned cloud resources will be deleted when CR is deleted
	// +kubebuilder:default:=true
	DeleteResource *bool `json:"deleteResource,omitempty"`

	// Region is cloud provider's region. It will override the region in the region field of ProviderReference
	Region string `json:"customRegion,omitempty"`

	// ForceDelete will force delete Configuration no matter which state it is or whether it has provisioned some resources
	// It will help delete Configuration in unexpected cases.
	ForceDelete *bool `json:"forceDelete,omitempty"`
}

// ConfigurationStatus defines the observed state of Configuration
type ConfigurationStatus struct {
	// observedGeneration is the most recent generation observed for this Configuration. It corresponds to the
	// Configuration's generation, which is updated on mutation by the API Server.
	// If ObservedGeneration equals Generation, and State is Available, the value of Outputs is latest
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	Apply   ConfigurationApplyStatus   `json:"apply,omitempty"`
	Destroy ConfigurationDestroyStatus `json:"destroy,omitempty"`
}

// ConfigurationApplyStatus is the status for Configuration apply
type ConfigurationApplyStatus struct {
	State   state.ConfigurationState `json:"state,omitempty"`
	Message string                   `json:"message,omitempty"`
	Outputs map[string]Property      `json:"outputs,omitempty"`
}

// ConfigurationDestroyStatus is the status for Configuration destroy
type ConfigurationDestroyStatus struct {
	State   state.ConfigurationState `json:"state,omitempty"`
	Message string                   `json:"message,omitempty"`
}

// Property is the property for an output
type Property struct {
	Value string `json:"value,omitempty"`
}

// Backend describes the Terraform backend configuration
type Backend struct {
	// SecretSuffix used when creating secrets. Secrets will be named in the format: tfstate-{workspace}-{secretSuffix}
	SecretSuffix string `json:"secretSuffix,omitempty"`
	// InClusterConfig Used to authenticate to the cluster from inside a pod. Only `true` is allowed
	InClusterConfig bool `json:"inClusterConfig,omitempty"`

	// Inline allows users to use raw hcl code to specify their Terraform backend
	Inline string `json:"inline,omitempty"`

	// BackendType indicates which backend type to use. This field is needed for custom backend configuration.
	BackendType string `json:"backend_type,omitempty"`

	// Remote is needed for the Terraform `remote` backend type.
	Remote *RemoteBackendConf `json:"remote,omitempty"`

	// Artifactory is needed for the Terraform `artifactory` backend type.
	Artifactory *ArtifactoryBackendConf `json:"artifactory,omitempty"`

	// Azurerm is needed for the Terraform `azurerm` backend type.
	Azurerm *AzurermBackendConf `json:"azurerm,omitempty"`

	// Consul is needed for the Terraform `consul` backend type.
	Consul *ConsulBackendConf `json:"consul,omitempty"`

	// Consul is needed for the Terraform `consul` backend type.
	COS *COSBackendConf `json:"cos,omitempty"`

	// ETCD is needed for the Terraform `etcd` backend type.
	ETCD *ETCDBackendConf `json:"etcd,omitempty"`

	// ETCDV3 is needed for the Terraform `etcdv3` backend type.
	ETCDV3 *ETCDV3BackendConf `json:"etcdv3,omitempty"`

	// GCS is needed for the Terraform `gcs` backend type.
	GCS *GCSBackendConf `json:"gcs,omitempty"`

	// HTTP is needed for the Terraform `http` backend type.
	HTTP *HTTPBackendConf `json:"http,omitempty"`

	// Kubernetes is needed for the Terraform `kubernetes` backend type.
	Kubernetes *KubernetesBackendConf `json:"kubernetes,omitempty"`

	// Manta is needed for the Terraform `manta` backend type.
	Manta *MantaBackendConf `json:"manta,omitempty"`

	// OSS is needed for the Terraform `oss` backend type.
	OSS *OSSBackendConf `json:"oss,omitempty"`

	// PG is needed for the Terraform `pg` backend type.
	PG *PGBackendConf `json:"pg,omitempty"`

	// s3 is needed for the Terraform `s3` backend type.
	S3 *S3BackendConf `json:"s3,omitempty"`

	// Swift is needed for the Terraform `swift` backend type.
	Swift *SwiftBackendConf `json:"swift,omitempty"`
}

// CurrentNSSecretSelector is used to specify the key in a secret in the current namespace.
type CurrentNSSecretSelector struct {
	// Name is the name of the secret
	Name string `json:"name"`
	// Key is the key selected in the secret
	Key string `json:"key"`
}

// RemoteBackendConf defines all options supported by the Terraform `remote` backend type.
// You can refer to https://www.terraform.io/language/settings/backends/remote for the usage of each option.
type RemoteBackendConf struct {
	Hostname     *string                     `json:"hostname,omitempty" hcl:"hostname"`
	Organization *string                     `json:"organization,omitempty" hcl:"organization"`
	Token        *string                     `json:"token,omitempty" hcl:"token"`
	Workspaces   RemoteBackendConfWorkspaces `json:"workspaces" hcl:"workspaces,block"`
}

// RemoteBackendConfWorkspaces defines the `Workspaces` struct in RemoteBackendConf
type RemoteBackendConfWorkspaces struct {
	Name   *string `json:"name,omitempty" hcl:"name"`
	Prefix *string `json:"prefix,omitempty" hcl:"prefix"`
}

// ArtifactoryBackendConf defines all options supported by the Terraform `artifactory` backend type.
// You can refer to https://www.terraform.io/language/settings/backends/artifactory for the usage of each option.
type ArtifactoryBackendConf struct {
	Username string `json:"username" hcl:"username"`
	Password string `json:"password" hcl:"password"`
	URL      string `json:"url" hcl:"url"`
	Repo     string `json:"repo" hcl:"repo"`
	Subpath  string `json:"subpath" hcl:"subpath"`
}

// AzurermBackendConf defines all options supported by the Terraform `azurerm` backend type.
// You can refer to https://www.terraform.io/language/settings/backends/azurerm for the usage of each option.
type AzurermBackendConf struct {
	StorageAccountName        string  `json:"storage_account_name" hcl:"storage_account_name"`
	ContainerName             string  `json:"container_name" hcl:"container_name"`
	Key                       string  `json:"key" hcl:"key"`
	Environment               *string `json:"environment,omitempty" hcl:"environment"`
	Endpoint                  *string `json:"endpoint,omitempty" hcl:"endpoint"`
	Snapshot                  *bool   `json:"snapshot,omitempty" hcl:"snapshot"`
	ResourceGroupName         *string `json:"resource_group_name,omitempty" hcl:"resource_group_name"`
	MSIEndpoint               *string `json:"msi_endpoint,omitempty" hcl:"msi_endpoint"`
	SubscriptionID            *string `json:"subscription_id,omitempty" hcl:"subscription_id"`
	TenantID                  *string `json:"tenant_id,omitempty" hcl:"tenant_id"`
	UseMicrosoftGraph         *bool   `json:"use_microsoft_graph,omitempty" hcl:"use_microsoft_graph"`
	UseMSI                    *bool   `json:"use_msi,omitempty" hcl:"use_msi"`
	SASToken                  *string `json:"sas_token,omitempty" hcl:"sas_token"`
	AccessKey                 *string `json:"access_key,omitempty" hcl:"access_key"`
	UseAzureadAuth            *bool   `json:"use_azuread_auth,omitempty" hcl:"use_azuread_auth"`
	ClientID                  *string `json:"client_id,omitempty" hcl:"client_id"`
	ClientCertificatePassword *string `json:"client_certificate_password,omitempty" hcl:"client_certificate_password"`
	// ClientCertificateSecret is a reference to a secret containing the client certificate
	// It's used to replace the `client_certificate_path` in native hcl backend configuration as we cannot use local file paths in the kubernetes cluster.
	ClientCertificateSecret *CurrentNSSecretSelector `json:"client_certificate_secret,omitempty" hcl:"ClientCertificateSecret,block"`
	ClientSecret            *string                  `json:"client_secret,omitempty" hcl:"client_secret"`
}

// ConsulBackendConf defines all options supported by the Terraform `consul` backend type.
// You can refer to https://www.terraform.io/language/settings/backends/consul for the usage of each option.
type ConsulBackendConf struct {
	Path        string  `json:"path" hcl:"path"`
	AccessToken string  `json:"access_token" hcl:"access_token"`
	Address     *string `json:"address,omitempty" hcl:"address"`
	Schema      *string `json:"schema,omitempty" hcl:"schema"`
	Datacenter  *string `json:"datacenter,omitempty" hcl:"datacenter"`
	HTTPAuth    *string `json:"http_auth,omitempty" hcl:"http_auth"`
	Gzip        *bool   `json:"gzip,omitempty" hcl:"gzip"`
	Lock        *bool   `json:"lock,omitempty" hcl:"lock"`
	// CAFileSecret is a reference to a secret containing the CA certificate
	// It's used to replace the `ca_file` in native hcl backend configuration as we cannot use local file paths in the kubernetes cluster.
	CAFileSecret *CurrentNSSecretSelector `json:"ca_file_secret,omitempty" hcl:"CAFileSecret,block"`
	// CertFileSecret is a reference to a secret containing the client certificate
	// It's used to replace the `cert_file` in native hcl backend configuration as we cannot use local file paths in the kubernetes cluster.
	CertFileSecret *CurrentNSSecretSelector `json:"cert_file_secret,omitempty" hcl:"CertFileSecret,block"`
	// KeyFileSecret is a reference to a secret containing the client key
	// It's used to replace the `key_file` in native hcl backend configuration as we cannot use local file paths in the kubernetes cluster.
	KeyFileSecret *CurrentNSSecretSelector `json:"key_file_secret,omitempty" hcl:"KeyFileSecret,block"`
}

// COSBackendConf defines all options supported by the Terraform `cos` backend type.
// You can refer to https://www.terraform.io/language/settings/backends/cos for the usage of each option.
type COSBackendConf struct {
	SecretID  *string `json:"secret_id,omitempty" hcl:"secret_id"`
	SecretKey *string `json:"secret_key,omitempty" hcl:"secret_key"`
	Region    *string `json:"region,omitempty" hcl:"region"`
	Bucket    string  `json:"bucket" hcl:"bucket"`
	Prefix    *string `json:"prefix,omitempty" hcl:"prefix"`
	Key       *string `json:"key,omitempty" hcl:"key"`
	Encrypt   *string `json:"encrypt,omitempty" hcl:"encrypt"`
	ACL       *string `json:"acl,omitempty" hcl:"acl"`
}

// ETCDBackendConf defines all options supported by the Terraform `etcd` backend type.
// You can refer to https://www.terraform.io/language/settings/backends/etcd for the usage of each option.
type ETCDBackendConf struct {
	Path      string  `json:"path" hcl:"path"`
	Endpoints string  `json:"endpoints" hcl:"endpoints"`
	Username  *string `json:"username,omitempty" hcl:"username"`
	Password  *string `json:"password,omitempty" hcl:"password"`
}

// ETCDV3BackendConf defines all options supported by the Terraform `etcdv3` backend type.
// You can refer to https://www.terraform.io/language/settings/backends/etcdv3 for the usage of each option.
type ETCDV3BackendConf struct {
	Endpoints string  `json:"endpoints" hcl:"endpoints"`
	Username  *string `json:"username,omitempty" hcl:"username"`
	Password  *string `json:"password,omitempty" hcl:"password"`
	Prefix    *string `json:"prefix,omitempty" hcl:"prefix"`
	Lock      *bool   `json:"lock,omitempty" hcl:"lock"`
	// CacertSecret is a reference to a secret containing the PEM-encoded CA bundle with which to verify certificates of TLS-enabled etcd servers.
	// It's used to replace the `cacert_path` in native hcl backend configuration as we cannot use local file paths in the kubernetes cluster.
	CacertSecret *CurrentNSSecretSelector `json:"cacert_secret,omitempty" hcl:"CacertSecret,block"`
	// CertSecret is a reference to a secret containing the PEM-encoded certificate to provide to etcd for secure client identification.
	// It's used to replace the `cert_path` in native hcl backend configuration as we cannot use local file paths in the kubernetes cluster.
	CertSecret *CurrentNSSecretSelector `json:"cert_secret,omitempty" hcl:"CertSecret,block"`
	// KeySecret is a reference to a secret containing the PEM-encoded key to provide to etcd for secure client identification.
	// It's used to replace the `key_path` in native hcl backend configuration as we cannot use local file paths in the kubernetes cluster.
	KeySecret       *CurrentNSSecretSelector `json:"key_secret,omitempty" hcl:"KeySecret,block"`
	MAXRequestBytes *int64                   `json:"max_request_bytes,omitempty" hcl:"max_request_bytes"`
}

// GCSBackendConf defines all options supported by the Terraform `gcs` backend type.
// You can refer to https://www.terraform.io/language/settings/backends/gcs for the usage of each option.
type GCSBackendConf struct {
	Bucket string `json:"bucket"`
	// CredentialsSecret is a reference to a secret containing Google Cloud Platform account credentials in JSON format.
	// It's used to replace the `credentials` in native hcl backend configuration as we cannot use local file paths in the kubernetes cluster.
	CredentialsSecret                  *CurrentNSSecretSelector `json:"credentials_secret,omitempty" hcl:"CredentialsSecret,block"`
	ImpersonateServiceAccount          *string                  `json:"impersonate_service_account,omitempty" hcl:"impersonate_service_account"`
	ImpersonateServiceAccountDelegates *string                  `json:"impersonate_service_account_delegates,omitempty" hcl:"impersonate_service_account_delegates"`
	AccessToken                        *string                  `json:"access_token,omitempty" hcl:"access_token"`
	Prefix                             *string                  `json:"prefix,omitempty" hcl:"prefix"`
	EncryptionKey                      *string                  `json:"encryption_key,omitempty" hcl:"encryption_key"`
}

// HTTPBackendConf defines all options supported by the Terraform `http` backend type.
// You can refer to https://www.terraform.io/language/settings/backends/http for the usage of each option.
type HTTPBackendConf struct {
	Address              string  `json:"address" hcl:"address"`
	UpdateMethod         *string `json:"update_method,omitempty" hcl:"update_method"`
	LockAddress          *string `json:"lock_address,omitempty" hcl:"lock_address"`
	LockMethod           *string `json:"lock_method,omitempty" hcl:"lock_method"`
	UnlockAddress        *string `json:"unlock_address,omitempty" hcl:"unlock_address"`
	UnlockMethod         *string `json:"unlock_method,omitempty" hcl:"unlock_method"`
	Username             *string `json:"username,omitempty" hcl:"username"`
	Password             *string `json:"password,omitempty" hcl:"password"`
	SkipCertVerification *bool   `json:"skip_cert_verification,omitempty" hcl:"skip_cert_verification"`
	RetryMax             *int    `json:"retry_max,omitempty" hcl:"retry_max"`
	RetryWaitMin         *int    `json:"retry_wait_min,omitempty" hcl:"retry_wait_min"`
	RetryWaitMax         *int    `json:"retry_wait_max,omitempty" hcl:"retry_wait_max"`
}

// KubernetesBackendConf defines all options supported by the Terraform `kubernetes` backend type.
// You can refer to https://www.terraform.io/language/settings/backends/kubernetes for the usage of each option.
type KubernetesBackendConf struct {
	SecretSuffix         string             `json:"secret_suffix" hcl:"secret_suffix"`
	Labels               *map[string]string `json:"labels,omitempty" hcl:"labels"`
	Namespace            *string            `json:"namespace,omitempty" hcl:"namespace"`
	InClusterConfig      *bool              `json:"in_cluster_config,omitempty" hcl:"in_cluster_config"`
	LoadConfigFile       *bool              `json:"load_config_file,omitempty" hcl:"load_config_file"`
	Host                 *string            `json:"host,omitempty" hcl:"host"`
	Username             *string            `json:"username,omitempty" hcl:"username"`
	Password             *string            `json:"password,omitempty" hcl:"password"`
	Insecure             *bool              `json:"insecure,omitempty" hcl:"insecure"`
	ClientCertificate    *string            `json:"client_certificate,omitempty" hcl:"client_certificate"`
	ClientKey            *string            `json:"client_key,omitempty" hcl:"client_key"`
	ClusterCACertificate *string            `json:"cluster_ca_certificate,omitempty" hcl:"cluster_ca_certificate"`
	// ConfigSecret is a reference to a secret containing the kubeconfig file.
	// It's used to replace the `config_path` in native hcl backend configuration as we cannot use local file paths in the kubernetes cluster.
	ConfigSecret          *CurrentNSSecretSelector   `json:"config_secret,omitempty" hcl:"ConfigSecret,block"`
	ConfigContext         *string                    `json:"config_context,omitempty" hcl:"config_context"`
	ConfigContextAuthInfo *string                    `json:"config_context_auth_info,omitempty" hcl:"config_context_auth_info"`
	ConfigContextCluster  *string                    `json:"config_context_cluster,omitempty" hcl:"config_context_cluster"`
	Token                 *string                    `json:"token,omitempty" hcl:"token"`
	Exec                  *KubernetesBackendConfExec `json:"exec,omitempty" hcl:"exec,block"`
}

// KubernetesBackendConfExec defines the `Exec` struct in the KubernetesBackendConf
type KubernetesBackendConfExec struct {
	APIVersion string            `json:"api_version,omitempty" hcl:"api_version"`
	Command    string            `json:"command,omitempty" hcl:"command"`
	ENV        map[string]string `json:"env,omitempty" hcl:"env"`
	Args       []string          `json:"args,omitempty" hcl:"args"`
}

// MantaBackendConf defines all options supported by the Terraform `manta` backend type.
// You can refer to https://www.terraform.io/language/settings/backends/manta for the usage of each option.
type MantaBackendConf struct {
	Account               string  `json:"account" hcl:"account"`
	User                  *string `json:"user,omitempty" hcl:"user"`
	URL                   *string `json:"url,omitempty" hcl:"url"`
	KeyMaterial           *string `json:"key_material,omitempty" hcl:"key_material"`
	KeyID                 string  `json:"key_id" hcl:"key_id"`
	InsecureSkipTLSVerify *bool   `json:"insecure_skip_tls_verify,omitempty" hcl:"insecure_skip_tls_verify"`
	Path                  string  `json:"path" hcl:"path"`
	ObjectName            *string `json:"object_name,omitempty" hcl:"object_name"`
}

// OSSBackendConf defines all options supported by the Terraform `oss` backend type.
// You can refer to https://www.terraform.io/language/settings/backends/oss for the usage of each option.
type OSSBackendConf struct {
	AccessKey          *string                   `json:"access_key,omitempty" hcl:"access_key"`
	SecretKey          *string                   `json:"secret_key,omitempty" hcl:"secret_key"`
	SecurityToken      *string                   `json:"security_token,omitempty" hcl:"security_token"`
	ECSRoleName        *string                   `json:"ecs_role_name,omitempty" hcl:"ecs_role_name"`
	Region             *string                   `json:"region,omitempty" hcl:"region"`
	TablestoreEndpoint *string                   `json:"tablestore_endpoint,omitempty" hcl:"tablestore_endpoint"`
	Endpoint           *string                   `json:"endpoint,omitempty" hcl:"endpoint"`
	Bucket             string                    `json:"bucket" hcl:"bucket"`
	Prefix             *string                   `json:"prefix,omitempty" hcl:"prefix"`
	Key                *string                   `json:"key,omitempty" hcl:"key"`
	TablestoreTable    *string                   `json:"tablestore_table,omitempty" hcl:"tablestore_table"`
	Encrypt            *bool                     `json:"encrypt,omitempty" hcl:"encrypt"`
	ACL                *string                   `json:"acl,omitempty" hcl:"acl"`
	AssumeRole         *OSSBackendConfAssumeRole `json:"assume_role,omitempty" hcl:"assume_role,block"`
	// SharedCredentialsSecret is a reference to a secret that contains the shared credentials file.
	// It's used to replace the `shared_credentials_file` in native hcl backend configuration as we cannot use local file paths in the kubernetes cluster.
	SharedCredentialsSecret *CurrentNSSecretSelector `json:"shared_credentials_secret,omitempty" hcl:"SharedCredentialsSecret,block"`
	Profile                 *string                  `json:"profile,omitempty" hcl:"profile"`
}

// OSSBackendConfAssumeRole defines the `AssumeRole` struct in the OSSBackendConf
type OSSBackendConfAssumeRole struct {
	RoleArn           *string `json:"role_arn,omitempty" hcl:"role_arn"`
	SessionName       *string `json:"session_name,omitempty" hcl:"session_name"`
	Policy            *string `json:"policy,omitempty" hcl:"policy"`
	SessionExpiration *int    `json:"session_expiration,omitempty" hcl:"session_expiration"`
}

// PGBackendConf defines all options supported by the Terraform `pg` backend type.
// You can refer to https://www.terraform.io/language/settings/backends/pg for the usage of each option.
type PGBackendConf struct {
	ConnStr            string  `json:"conn_str" hcl:"conn_str"`
	SchemaNam          *string `json:"schema_name,omitempty" hcl:"schema_name"`
	SkipSchemaCreation *bool   `json:"skip_schema_creation,omitempty" hcl:"skip_schema_creation"`
	SkipTableCreation  *bool   `json:"skip_table_creation,omitempty" hcl:"skip_table_creation"`
	SkipIndexCreation  *bool   `json:"skip_index_creation,omitempty" hcl:"skip_index_creation"`
}

// S3BackendConf defines all options supported by the Terraform `s3` backend type.
// You can refer to https://www.terraform.io/language/settings/backends/s3 for the usage of each option.
type S3BackendConf struct {
	Bucket           string  `json:"bucket" hcl:"bucket"`
	Key              string  `json:"key" hcl:"key"`
	Region           string  `json:"region" hcl:"region"`
	DynamodbEndpoint *string `json:"dynamodb_endpoint,omitempty" hcl:"dynamodb_endpoint"`
	DynamodbTable    *string `json:"dynamodb_table,omitempty" hcl:"dynamodb_table"`
	Endpoint         *string `json:"endpoint,omitempty" hcl:"endpoint"`
	IAMEndpoint      *string `json:"iam_endpoint,omitempty" hcl:"iam_endpoint"`
	STSEndpoint      *string `json:"sts_endpoint,omitempty" hcl:"sts_endpoint"`
	Encrypt          *bool   `json:"encrypt,omitempty" hcl:"encrypt"`
	ACL              *string `json:"acl,omitempty" hcl:"acl"`
	AccessKey        *string `json:"access_key,omitempty" hcl:"access_key"`
	SecretKey        *string `json:"secret_key,omitempty" hcl:"secret_key"`
	KMSKeyID         *string `json:"kms_key_id,omitempty" hcl:"kms_key_id"`
	// SharedCredentialsSecret is a reference to a secret that contains the AWS shared credentials file.
	// It's used to replace the `shared_credentials_file` in native hcl backend configuration as we cannot use local file paths in the kubernetes cluster.
	SharedCredentialsSecret     *CurrentNSSecretSelector `json:"shared_credentials_secret,omitempty" hcl:"SharedCredentialsSecret,block"`
	Profile                     *string                  `json:"profile,omitempty" hcl:"profile"`
	Token                       *string                  `json:"token,omitempty" hcl:"token"`
	SkipCredentialsValidation   *bool                    `json:"skip_credentials_validation,omitempty" hcl:"skip_credentials_validation"`
	SkipRegionValidation        *bool                    `json:"skip_region_validation,omitempty" hcl:"skip_region_validation"`
	SkipMetadataAPICheck        *bool                    `json:"skip_metadata_api_check,omitempty" hcl:"skip_metadata_api_check"`
	SSECustomerKey              *string                  `json:"sse_customer_key,omitempty" hcl:"sse_customer_key"`
	RoleARN                     *string                  `json:"role_arn,omitempty" hcl:"role_arn"`
	SessionName                 *string                  `json:"session_name,omitempty" hcl:"session_name"`
	ExternalID                  *string                  `json:"external_id,omitempty" hcl:"external_id"`
	AssumeRoleDurationSeconds   *int                     `json:"assume_role_duration_seconds,omitempty" hcl:"assume_role_duration_seconds"`
	AssumeRolePolicy            *string                  `json:"assume_role_policy,omitempty" hcl:"assume_role_policy"`
	AssumeRolePolicyArns        *[]string                `json:"assume_role_policy_arns,omitempty" hcl:"assume_role_policy_arns"`
	AssumeRoleTags              *map[string]string       `json:"assume_role_tags,omitempty" hcl:"assume_role_tags"`
	AssumeRoleTransitiveTagKeys *[]string                `json:"assume_role_transitive_tag_keys,omitempty" hcl:"assume_role_transitive_tag_keys"`
	WorkspaceKeyPrefix          *string                  `json:"workspace_key_prefix,omitempty" hcl:"workspace_key_prefix"`
	ForcePathStyle              *string                  `json:"force_path_style,omitempty" hcl:"force_path_style"`
	MAXRetries                  *int                     `json:"max_retries,omitempty" hcl:"max_retries"`
}

// SwiftBackendConf defines all options supported by the Terraform `swift` backend type.
// You can refer to https://www.terraform.io/language/settings/backends/swift for the usage of each option.
type SwiftBackendConf struct {
	AuthURL                     *string `json:"auth_url,omitempty" hcl:"auth_url"`
	RegionName                  *string `json:"region_name,omitempty" hcl:"region_name"`
	UserName                    *string `json:"user_name,omitempty" hcl:"user_name"`
	UserID                      *string `json:"user_id,omitempty" hcl:"user_id"`
	ApplicationCredentialID     *string `json:"application_credential_id,omitempty" hcl:"application_credential_id"`
	ApplicationCredentialName   *string `json:"application_credential_name,omitempty" hcl:"application_credential_name"`
	ApplicationCredentialSecret *string `json:"application_credential_secret,omitempty" hcl:"application_credential_secret"`
	TenantID                    *string `json:"tenant_id,omitempty" hcl:"tenant_id"`
	TenantName                  *string `json:"tenant_name,omitempty" hcl:"tenant_name"`
	Password                    *string `json:"password,omitempty" hcl:"password"`
	Token                       *string `json:"token,omitempty" hcl:"token"`
	USerDomainName              *string `json:"user_domain_name,omitempty" hcl:"user_domain_name"`
	UserDomainID                *string `json:"user_domain_id,omitempty" hcl:"user_domain_id"`
	ProjectDomainName           *string `json:"project_domain_name,omitempty" hcl:"project_domain_name"`
	ProjectDomainID             *string `json:"project_domain_id,omitempty" hcl:"project_domain_id"`
	DomainID                    *string `json:"domain_id,omitempty" hcl:"domain_id"`
	DomainName                  *string `json:"domain_name,omitempty" hcl:"domain_name"`
	DefaultDomain               *string `json:"default_domain,omitempty" hcl:"default_domain"`
	Insecure                    *bool   `json:"insecure,omitempty" hcl:"insecure"`
	EndpointType                *string `json:"endpoint_type,omitempty" hcl:"endpoint_type"`
	// CacertSecret is a reference to a secret that contains a custom CA certificate when communicating over SSL.
	// It's used to replace the `cacert_file` in native hcl backend configuration as we cannot use local file paths in the kubernetes cluster.
	CacertSecret *CurrentNSSecretSelector `json:"cacert_file_secret,omitempty" hcl:"CacertSecret,block"`
	// CertSecret is a reference to a secret that contains certificate file for SSL client authentication.
	// It's used to replace the `cert` in native hcl backend configuration as we cannot use local file paths in the kubernetes cluster.
	CertSecret *CurrentNSSecretSelector `json:"cert_file_secret,omitempty" hcl:"CertSecret,block"`
	// KeySecret is a reference to a secret that contains client private key file for SSL client authentication.
	// It's used to replace the `key` in native hcl backend configuration as we cannot use local file paths in the kubernetes cluster.
	KeySecret            *CurrentNSSecretSelector `json:"key_file_secret,omitempty" hcl:"KeySecret,block"`
	Swauth               *bool                    `json:"swauth,omitempty" hcl:"swauth"`
	AllowReauth          *bool                    `json:"allow_reauth,omitempty" hcl:"allow_reauth"`
	Cloud                *string                  `json:"cloud,omitempty" hcl:"cloud"`
	MAXRetries           *int                     `json:"max_retries,omitempty" hcl:"max_retries"`
	DisableNoCacheHeader *bool                    `json:"disable_no_cache_header,omitempty" hcl:"disable_no_cache_header"`
	// Deprecated: Use `container` instead.
	Path             *string `json:"path,omitempty" hcl:"path"`
	Container        *string `json:"container" hcl:"container"`
	ArchivePath      *string `json:"archive_path,omitempty" hcl:"archive_path"`
	ArchiveContainer *string `json:"archive_container,omitempty" hcl:"archive_container"`
	ExpireAfter      *string `json:"expire_after,omitempty" hcl:"expire_after"`
	Lock             *bool   `json:"lock,omitempty" hcl:"lock"`
	StateName        *string `json:"state_name,omitempty" hcl:"state_name"`
}

// +kubebuilder:object:root=true

// Configuration is the Schema for the configurations API
//+kubebuilder:storageversion
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="STATE",type="string",JSONPath=".status.apply.state"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
type Configuration struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ConfigurationSpec   `json:"spec,omitempty"`
	Status ConfigurationStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ConfigurationList contains a list of Configuration
type ConfigurationList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Configuration `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Configuration{}, &ConfigurationList{})
}
