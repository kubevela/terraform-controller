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
	crossplanetypes "github.com/oam-dev/terraform-controller/api/types/crossplane-runtime"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +kubebuilder:storageversion
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="STATE",type="string",JSONPath=".status.state"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"

// Backend is the Schema for the backend API.
type Backend struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec BackendSpec `json:"spec,omitempty"`
}

// BackendSpec defines the desired state of Backend
type BackendSpec struct {
	// Backend contains all options supported by the Terraform backend configuration.
	Backend BackendSpecInnerMain `json:"backend,omitempty"`
}

// BackendSpecInnerMain defines all options supported by the Terraform backend configuration.
type BackendSpecInnerMain struct {
	// Remote is needed for the Terraform `remote` backend type.
	Remote RemoteBackendConf `json:"remote,omitempty"`
	//
	// Artifactory is needed for the Terraform `artifactory` backend type.
	Artifactory ArtifactoryBackendConf `json:"artifactory,omitempty"`

	// Azurerm is needed for the Terraform `azurerm` backend type.
	Azurerm AzurermBackendConf `json:"azurerm,omitempty"`

	// Consul is needed for the Terraform `consul` backend type.
	Consul ConsulBackendConf `json:"consul,omitempty"`

	// Consul is needed for the Terraform `consul` backend type.
	COS COSBackendConf `json:"cos,omitempty"`

	// ETCD is needed for the Terraform `etcd` backend type.
	ETCD ETCDBackendConf `json:"etcd,omitempty"`

	// ETCDV3 is needed for the Terraform `etcdv3` backend type.
	ETCDV3 ETCDV3BackendConf `json:"etcdv3,omitempty"`

	// GCS is needed for the Terraform `gcs` backend type.
	GCS GCSBackendConf `json:"gcs,omitempty"`

	// HTTP is needed for the Terraform `http` backend type.
	HTTP HTTPBackendConf `json:"http,omitempty"`

	// Kubernetes is needed for the Terraform `kubernetes` backend type.
	Kubernetes KubernetesBackendConf `json:"kubernetes,omitempty"`

	// Manta is needed for the Terraform `manta` backend type.
	Manta MantaBackendConf `json:"manta,omitempty"`

	// OSS is needed for the Terraform `oss` backend type.
	OSS OSSBackendConf `json:"oss,omitempty"`

	// PG is needed for the Terraform `pg` backend type.
	PG PGBackendConf `json:"pg,omitempty"`

	// s3 is needed for the Terraform `s3` backend type.
	S3 S3BackendConf `json:"s3,omitempty"`

	// Swift is needed for the Terraform `swift` backend type.
	Swift SwiftBackendConf `json:"swift,omitempty"`
}

// RemoteBackendConf defines all options supported by the Terraform `remote` backend type.
// You can refer to [the Terraform documentation](https://www.terraform.io/language/settings/backends/remote) for the usage of each option.
type RemoteBackendConf struct {
	Hostname     string                      `json:"hostname,omitempty" hcl:"hostname"`
	Organization string                      `json:"organization,omitempty" hcl:"organization"`
	Token        string                      `json:"token,omitempty" hcl:"token"`
	Workspaces   RemoteBackendConfWorkspaces `json:"workspaces,omitempty" hcl:"workspaces,block"`
}

type RemoteBackendConfWorkspaces struct {
	Name   string `json:"name,omitempty" hcl:"name"`
	Prefix string `json:"prefix,omitempty" hcl:"prefix"`
}

// ArtifactoryBackendConf defines all options supported by the Terraform `artifactory` backend type.
// You can refer to [the Terraform documentation](https://www.terraform.io/language/settings/backends/remote) for the usage of each option.
type ArtifactoryBackendConf struct {
	Username string `json:"username,omitempty" hcl:"username"`
	Password string `json:"password,omitempty" hcl:"password"`
	URL      string `json:"url,omitempty" hcl:"url"`
	Repo     string `json:"repo,omitempty" hcl:"repo"`
	Subpath  string `json:"subpath,omitempty" hcl:"subpath"`
}

// AzurermBackendConf defines all options supported by the Terraform `azurerm` backend type.
// You can refer to [the Terraform documentation](https://www.terraform.io/language/settings/backends/remote) for the usage of each option.
type AzurermBackendConf struct {
	StorageAccountName string `json:"storage_account_name,omitempty" hcl:"storage_account_name"`
	ContainerName      string `json:"container_name,omitempty" hcl:"container_name"`
	Key                string `json:"key,omitempty" hcl:"key"`
	Environment        string `json:"environment,omitempty" hcl:"environment"`
	Endpoint           string `json:"endpoint,omitempty" hcl:"endpoint"`
	Snapshot           bool   `json:"snapshot,omitempty" hcl:"snapshot"`

	ResourceGroupName string `json:"resource_group_name,omitempty" hcl:"resource_group_name"`
	MSIEndpoint       string `json:"msi_endpoint,omitempty" hcl:"msi_endpoint"`
	SubscriptionID    string `json:"subscription_id,omitempty" hcl:"subscription_id"`
	TenantID          string `json:"tenant_id,omitempty" hcl:"tenant_id"`
	UseMicrosoftGraph bool   `json:"use_microsoft_graph,omitempty" hcl:"use_microsoft_graph"`
	UseMSI            bool   `json:"use_msi,omitempty" hcl:"use_msi"`

	SASToken string `json:"sas_token,omitempty" hcl:"sas_token"`

	AccessKey string `json:"access_key,omitempty" hcl:"access_key"`

	UseAzureadAuth string `json:"use_azuread_auth,omitempty" hcl:"use_azuread_auth"`

	ClientID                  string `json:"client_id,omitempty" hcl:"client_id"`
	ClientCertificatePassword string `json:"client_certificate_password,omitempty" hcl:"client_certificate_password"`
	// ClientCertificateSecret is a reference to a secret containing the client certificate
	// It's used to replace the `client_certificate_path` in native hcl backend configuration as we cannot use local file paths in the kubernetes cluster.
	ClientCertificateSecret crossplanetypes.CredentialsSource `json:"client_certificate_secret,omitempty" hcl:"client_certificate_secret"`
	ClientSecret            string                            `json:"client_secret,omitempty" hcl:"client_secret"`
}

// ConsulBackendConf defines all options supported by the Terraform `consul` backend type.
// You can refer to [the Terraform documentation](https://www.terraform.io/language/settings/backends/consul) for the usage of each option.
type ConsulBackendConf struct {
	Path        string `json:"path,omitempty" hcl:"path"`
	AccessToken string `json:"access_token,omitempty" hcl:"access_token"`
	Address     string `json:"address,omitempty" hcl:"address"`
	Schema      string `json:"schema,omitempty" hcl:"schema"`
	Datacenter  string `json:"datacenter,omitempty" hcl:"datacenter"`
	HTTPAuth    string `json:"http_auth,omitempty" hcl:"http_auth"`
	Gzip        bool   `json:"gzip,omitempty" hcl:"gzip"`
	Lock        bool   `json:"lock,omitempty" hcl:"lock"`
	// CAFileSecret is a reference to a secret containing the CA certificate
	// It's used to replace the `ca_file` in native hcl backend configuration as we cannot use local file paths in the kubernetes cluster.
	CAFileSecret crossplanetypes.CredentialsSource `json:"ca_file_secret,omitempty" hcl:"ca_file_secret"`
	// CertFileSecret is a reference to a secret containing the client certificate
	// It's used to replace the `cert_file` in native hcl backend configuration as we cannot use local file paths in the kubernetes cluster.
	CertFileSecret crossplanetypes.CredentialsSource `json:"cert_file_secret,omitempty" hcl:"cert_file_secret"`
	// KeyFileSecret is a reference to a secret containing the client key
	// It's used to replace the `key_file` in native hcl backend configuration as we cannot use local file paths in the kubernetes cluster.
	KeyFileSecret crossplanetypes.CredentialsSource `json:"key_file_secret,omitempty" hcl:"key_file_secret"`
}

// COSBackendConf defines all options supported by the Terraform `cos` backend type.
// You can refer to [the Terraform documentation](https://www.terraform.io/language/settings/backends/cos) for the usage of each option.
type COSBackendConf struct {
	SecretID  string `json:"secret_id,omitempty" hcl:"secret_id"`
	SecretKey string `json:"secret_key,omitempty" hcl:"secret_key"`
	Region    string `json:"region,omitempty" hcl:"region"`
	Bucket    string `json:"bucket,omitempty" hcl:"bucket"`
	Prefix    string `json:"prefix,omitempty" hcl:"prefix"`
	Key       string `json:"key,omitempty" hcl:"key"`
	Encrypt   string `json:"encrypt,omitempty" hcl:"encrypt"`
	ACL       string `json:"acl,omitempty" hcl:"acl"`
}

// ETCDBackendConf defines all options supported by the Terraform `etcd` backend type.
// You can refer to [the Terraform documentation](https://www.terraform.io/language/settings/backends/etcd) for the usage of each option.
type ETCDBackendConf struct {
	Path      string `json:"path,omitempty" hcl:"path"`
	Endpoints string `json:"endpoints,omitempty" hcl:"endpoints"`
	Username  string `json:"username,omitempty" hcl:"username"`
	Password  string `json:"password,omitempty" hcl:"password"`
}

// ETCDV3BackendConf defines all options supported by the Terraform `etcdv3` backend type.
// You can refer to [the Terraform documentation](https://www.terraform.io/language/settings/backends/etcdv3) for the usage of each option.
type ETCDV3BackendConf struct {
	Endpoints string `json:"endpoints,omitempty" hcl:"endpoints"`
	Username  string `json:"username,omitempty" hcl:"username"`
	Password  string `json:"password,omitempty" hcl:"password"`
	Prefix    string `json:"prefix,omitempty" hcl:"prefix"`
	Lock      bool   `json:"lock,omitempty" hcl:"lock"`
	// CacertSecret is a reference to a secret containing the PEM-encoded CA bundle with which to verify certificates of TLS-enabled etcd servers.
	// It's used to replace the `cacert_path` in native hcl backend configuration as we cannot use local file paths in the kubernetes cluster.
	CacertSecret crossplanetypes.CredentialsSource `json:"cacert_secret,omitempty" hcl:"cacert_secret"`
	// CertSecret is a reference to a secret containing the PEM-encoded certificate to provide to etcd for secure client identification.
	// It's used to replace the `cert_path` in native hcl backend configuration as we cannot use local file paths in the kubernetes cluster.
	CertSecret crossplanetypes.CredentialsSource `json:"cert_secret,omitempty" hcl:"cert_secret"`
	// KeySecret is a reference to a secret containing the PEM-encoded key to provide to etcd for secure client identification.
	// It's used to replace the `key_path` in native hcl backend configuration as we cannot use local file paths in the kubernetes cluster.
	KeySecret       crossplanetypes.CredentialsSource `json:"key_secret,omitempty" hcl:"key_secret"`
	MAXRequestBytes int64                             `json:"max_request_bytes,omitempty" hcl:"max_request_bytes"`
}

// GCSBackendConf defines all options supported by the Terraform `gcs` backend type.
// You can refer to [the Terraform documentation](https://www.terraform.io/language/settings/backends/gcs) for the usage of each option.
type GCSBackendConf struct {
	Bucket string `json:"bucket,omitempty"`
	// CredentialsSecret is a reference to a secret containing Google Cloud Platform account credentials in JSON format.
	// It's used to replace the `credentials` in native hcl backend configuration as we cannot use local file paths in the kubernetes cluster.
	CredentialsSecret                  crossplanetypes.CredentialsSource `json:"credentials_secret,omitempty" hcl:"credentials_secret"`
	ImpersonateServiceAccount          string                            `json:"impersonate_service_account,omitempty" hcl:"impersonate_service_account"`
	ImpersonateServiceAccountDelegates string                            `json:"impersonate_service_account_delegates,omitempty" hcl:"impersonate_service_account_delegates"`
	AccessToken                        string                            `json:"access_token,omitempty" hcl:"access_token"`
	Prefix                             string                            `json:"prefix,omitempty" hcl:"prefix"`
	EncryptionKey                      string                            `json:"encryption_key,omitempty" hcl:"encryption_key"`
}

// HTTPBackendConf defines all options supported by the Terraform `http` backend type.
// You can refer to [the Terraform documentation](https://www.terraform.io/language/settings/backends/http) for the usage of each option.
type HTTPBackendConf struct {
	Address              string `json:"address,omitempty" hcl:"address"`
	UpdateMethod         string `json:"update_method,omitempty" hcl:"update_method"`
	LockAddress          string `json:"lock_address,omitempty" hcl:"lock_address"`
	LockMethod           string `json:"lock_method,omitempty" hcl:"lock_method"`
	UnlockAddress        string `json:"unlock_address,omitempty" hcl:"unlock_address"`
	UnlockMethod         string `json:"unlock_method,omitempty" hcl:"unlock_method"`
	Username             string `json:"username,omitempty" hcl:"username"`
	Password             string `json:"password,omitempty" hcl:"password"`
	SkipCertVerification bool   `json:"skip_cert_verification,omitempty" hcl:"skip_cert_verification"`
	RetryMax             int    `json:"retry_max,omitempty" hcl:"retry_max"`
	RetryWaitMin         int    `json:"retry_wait_min,omitempty" hcl:"retry_wait_min"`
	RetryWaitMax         int    `json:"retry_wait_max,omitempty" hcl:"retry_wait_max"`
}

// KubernetesBackendConf defines all options supported by the Terraform `kubernetes` backend type.
// You can refer to [the Terraform documentation](https://www.terraform.io/language/settings/backends/kubernetes) for the usage of each option.
type KubernetesBackendConf struct {
	SecretSuffix         string            `json:"secret_suffix,omitempty" hcl:"secret_suffix"`
	Labels               map[string]string `json:"labels,omitempty" hcl:"labels"`
	Namespace            string            `json:"namespace,omitempty" hcl:"namespace"`
	InClusterConfig      bool              `json:"in_cluster_config,omitempty" hcl:"in_cluster_config"`
	LoadConfigFile       bool              `json:"load_config_file,omitempty" hcl:"load_config_file"`
	Host                 string            `json:"host,omitempty" hcl:"host"`
	Username             string            `json:"username,omitempty" hcl:"username"`
	Password             string            `json:"password,omitempty" hcl:"password"`
	Insecure             string            `json:"insecure,omitempty" hcl:"insecure"`
	ClientCertificate    string            `json:"client_certificate,omitempty" hcl:"client_certificate"`
	ClientKey            string            `json:"client_key,omitempty" hcl:"client_key"`
	ClusterCACertificate string            `json:"cluster_ca_certificate,omitempty" hcl:"cluster_ca_certificate"`
	// ConfigSecret is a reference to a secret containing the kubeconfig file.
	// It's used to replace the `config_path` in native hcl backend configuration as we cannot use local file paths in the kubernetes cluster.
	ConfigSecret crossplanetypes.CredentialsSource `json:"config_secret,omitempty" hcl:"config_secret"`
	// ConfigSecrets is a list of references to secrets containing the kubeconfig file.
	// It's used to replace the `config_paths` in native hcl backend configuration as we cannot use local file paths in the kubernetes cluster.
	ConfigSecrets         []crossplanetypes.CredentialsSource `json:"config_secrets,omitempty" hcl:"config_secrets"`
	ConfigContext         string                              `json:"config_context,omitempty" hcl:"config_context"`
	ConfigContextAuthInfo string                              `json:"config_context_auth_info,omitempty" hcl:"config_context_auth_info"`
	ConfigContextCluster  string                              `json:"config_context_cluster,omitempty" hcl:"config_context_cluster"`
	Token                 string                              `json:"token,omitempty" hcl:"token"`
	Exec                  KubernetesBackendConfExec           `json:"exec,omitempty" hcl:"exec"`
}

type KubernetesBackendConfExec struct {
	APIVersion string            `json:"api_version,omitempty" hcl:"api_version"`
	Command    string            `json:"command,omitempty" hcl:"command"`
	ENV        map[string]string `json:"env,omitempty" hcl:"env"`
	Args       []string          `json:"args,omitempty" hcl:"args"`
}

// MantaBackendConf defines all options supported by the Terraform `manta` backend type.
// You can refer to [the Terraform documentation](https://www.terraform.io/docs/backends/types/manta) for the usage of each option.
type MantaBackendConf struct {
	Account               string `json:"account,omitempty" hcl:"account"`
	User                  string `json:"user,omitempty" hcl:"user"`
	URL                   string `json:"url,omitempty" hcl:"url"`
	KeyMaterial           string `json:"key_material,omitempty" hcl:"key_material"`
	KeyID                 string `json:"key_id,omitempty" hcl:"key_id"`
	InsecureSkipTLSVerify bool   `json:"insecure_skip_tls_verify,omitempty" hcl:"insecure_skip_tls_verify"`
	Path                  string `json:"path,omitempty" hcl:"path"`
	ObjectName            string `json:"object_name,omitempty" hcl:"object_name"`
}

// OSSBackendConf defines all options supported by the Terraform `oss` backend type.
// You can refer to [the Terraform documentation](https://www.terraform.io/docs/backends/types/oss) for the usage of each option.
type OSSBackendConf struct {
	AccessKey          string                   `json:"access_key,omitempty" hcl:"access_key"`
	SecretKey          string                   `json:"secret_key,omitempty" hcl:"secret_key"`
	SecurityToken      string                   `json:"security_token,omitempty" hcl:"security_token"`
	ECSRoleName        string                   `json:"ecs_role_name,omitempty" hcl:"ecs_role_name"`
	Region             string                   `json:"region,omitempty" hcl:"region"`
	TablestoreEndpoint string                   `json:"tablestore_endpoint,omitempty" hcl:"tablestore_endpoint"`
	Endpoint           string                   `json:"endpoint,omitempty" hcl:"endpoint"`
	Bucket             string                   `json:"bucket,omitempty" hcl:"bucket"`
	Prefix             string                   `json:"prefix,omitempty" hcl:"prefix"`
	Key                string                   `json:"key,omitempty" hcl:"key"`
	TablestoreTable    string                   `json:"tablestore_table,omitempty" hcl:"tablestore_table"`
	Encrypt            bool                     `json:"encrypt,omitempty" hcl:"encrypt"`
	ACL                string                   `json:"acl,omitempty" hcl:"acl"`
	AssumeRole         OSSBackendConfAssumeRole `json:"assume_role,omitempty" hcl:"assume_role"`
	// SharedCredentialsSecret is a reference to a secret that contains the shared credentials file.
	// It's used to replace the `shared_credentials_file` in native hcl backend configuration as we cannot use local file paths in the kubernetes cluster.
	SharedCredentialsSecret crossplanetypes.CredentialsSource `json:"shared_credentials_secret,omitempty" hcl:"shared_credentials_secret"`
	Profile                 string                            `json:"profile,omitempty" hcl:"profile"`
}

type OSSBackendConfAssumeRole struct {
	RoleArn           string `json:"role_arn,omitempty" hcl:"role_arn"`
	SessionName       string `json:"session_name,omitempty" hcl:"session_name"`
	Policy            string `json:"policy,omitempty" hcl:"policy"`
	SessionExpiration int    `json:"session_expiration,omitempty" hcl:"session_expiration"`
}

// PGBackendConf defines all options supported by the Terraform `pg` backend type.
// You can refer to [the Terraform documentation](https://www.terraform.io/docs/backends/types/pg) for the usage of each option.
type PGBackendConf struct {
	ConnStr            string `json:"conn_str,omitempty" hcl:"conn_str"`
	SchemaNam          string `json:"schema_name,omitempty" hcl:"schema_name"`
	SkipSchemaCreation bool   `json:"skip_schema_creation,omitempty" hcl:"skip_schema_creation"`
	SkipTableCreation  bool   `json:"skip_table_creation,omitempty" hcl:"skip_table_creation"`
	SkipIndexCreation  bool   `json:"skip_index_creation,omitempty" hcl:"skip_index_creation"`
}

// S3BackendConf defines all options supported by the Terraform `s3` backend type.
// You can refer to [the Terraform documentation](https://www.terraform.io/docs/backends/types/s3) for the usage of each option.
type S3BackendConf struct {
	Bucket           string `json:"bucket,omitempty" hcl:"bucket"`
	Key              string `json:"key,omitempty" hcl:"key"`
	Region           string `json:"region,omitempty" hcl:"region"`
	DynamodbEndpoint string `json:"dynamodb_endpoint,omitempty" hcl:"dynamodb_endpoint"`
	DynamodbTable    string `json:"dynamodb_table,omitempty" hcl:"dynamodb_table"`
	Endpoint         string `json:"endpoint,omitempty" hcl:"endpoint"`
	IAMEndpoint      string `json:"iam_endpoint,omitempty" hcl:"iam_endpoint"`
	STSEndpoint      string `json:"sts_endpoint,omitempty" hcl:"sts_endpoint"`
	Encrypt          bool   `json:"encrypt,omitempty" hcl:"encrypt"`
	ACL              string `json:"acl,omitempty" hcl:"acl"`
	AccessKey        string `json:"access_key,omitempty" hcl:"access_key"`
	SecretKey        string `json:"secret_key,omitempty" hcl:"secret_key"`
	KMSKeyID         string `json:"kms_key_id,omitempty" hcl:"kms_key_id"`
	// SharedCredentialsSecret is a reference to a secret that contains the AWS shared credentials file.
	// It's used to replace the `shared_credentials_file` in native hcl backend configuration as we cannot use local file paths in the kubernetes cluster.
	SharedCredentialsSecret     crossplanetypes.CredentialsSource `json:"shared_credentials_secret,omitempty" hcl:"shared_credentials_secret"`
	Profile                     string                            `json:"profile,omitempty" hcl:"profile"`
	Token                       string                            `json:"token,omitempty" hcl:"token"`
	SkipCredentialsValidation   bool                              `json:"skip_credentials_validation,omitempty" hcl:"skip_credentials_validation"`
	SkipRegionValidation        bool                              `json:"skip_region_validation,omitempty" hcl:"skip_region_validation"`
	SkipMetadataAPICheck        bool                              `json:"skip_metadata_api_check,omitempty" hcl:"skip_metadata_api_check"`
	SSECustomerKey              string                            `json:"sse_customer_key,omitempty" hcl:"sse_customer_key"`
	RoleARN                     string                            `json:"role_arn,omitempty" hcl:"role_arn"`
	SessionName                 string                            `json:"session_name,omitempty" hcl:"session_name"`
	ExternalID                  string                            `json:"external_id,omitempty" hcl:"external_id"`
	AssumeRoleDurationSeconds   int                               `json:"assume_role_duration_seconds,omitempty" hcl:"assume_role_duration_seconds"`
	AssumeRolePolicy            string                            `json:"assume_role_policy,omitempty" hcl:"assume_role_policy"`
	AssumeRolePolicyArns        []string                          `json:"assume_role_policy_arns,omitempty" hcl:"assume_role_policy_arns"`
	AssumeRoleTags              map[string]string                 `json:"assume_role_tags,omitempty" hcl:"assume_role_tags"`
	AssumeRoleTransitiveTagKeys []string                          `json:"assume_role_transitive_tag_keys,omitempty" hcl:"assume_role_transitive_tag_keys"`
	WorkspaceKeyPrefix          string                            `json:"workspace_key_prefix,omitempty" hcl:"workspace_key_prefix"`
	ForcePathStyle              string                            `json:"force_path_style,omitempty" hcl:"force_path_style"`
	MAXRetries                  int                               `json:"max_retries,omitempty" hcl:"max_retries"`
}

// SwiftBackendConf defines all options supported by the Terraform `swift` backedn type.
// You can refer to [the Terraform documentation](https://www.terraform.io/docs/backends/types/swift) for the usage of each option.
type SwiftBackendConf struct {
	AuthURL                     string `json:"auth_url,omitempty" hcl:"auth_url"`
	RegionName                  string `json:"region_name,omitempty" hcl:"region_name"`
	UserName                    string `json:"user_name,omitempty" hcl:"user_name"`
	UserID                      string `json:"user_id,omitempty" hcl:"user_id"`
	ApplicationCredentialID     string `json:"application_credential_id,omitempty" hcl:"application_credential_id"`
	ApplicationCredentialName   string `json:"application_credential_name,omitempty" hcl:"application_credential_name"`
	ApplicationCredentialSecret string `json:"application_credential_secret,omitempty" hcl:"application_credential_secret"`
	TenantID                    string `json:"tenant_id,omitempty" hcl:"tenant_id"`
	TenantName                  string `json:"tenant_name,omitempty" hcl:"tenant_name"`
	Password                    string `json:"password,omitempty" hcl:"password"`
	Token                       string `json:"token,omitempty" hcl:"token"`
	USerDomainName              string `json:"user_domain_name,omitempty" hcl:"user_domain_name"`
	UserDomainID                string `json:"user_domain_id,omitempty" hcl:"user_domain_id"`
	ProjectDomainName           string `json:"project_domain_name,omitempty" hcl:"project_domain_name"`
	ProjectDomainID             string `json:"project_domain_id,omitempty" hcl:"project_domain_id"`
	DomainID                    string `json:"domain_id,omitempty" hcl:"domain_id"`
	DomainName                  string `json:"domain_name,omitempty" hcl:"domain_name"`
	DefaultDomain               string `json:"default_domain,omitempty" hcl:"default_domain"`
	Insecure                    bool   `json:"insecure,omitempty" hcl:"insecure"`
	EndpointType                string `json:"endpoint_type,omitempty" hcl:"endpoint_type"`
	// CacertSecret is a reference to a secret that contains a custom CA certificate when communicating over SSL.
	// It's used to replace the `cacert_file` in native hcl backend configuration as we cannot use local file paths in the kubernetes cluster.
	CacertSecret crossplanetypes.CredentialsSource `json:"cacert_file_secret,omitempty" hcl:"cacert_file_secret"`
	// CertSecret is a reference to a secret that contains certificate file for SSL client authentication.
	// It's used to replace the `cert` in native hcl backend configuration as we cannot use local file paths in the kubernetes cluster.
	CertSecret crossplanetypes.CredentialsSource `json:"cert_file_secret,omitempty" hcl:"cert_file_secret"`
	// KeySecret is a reference to a secret that contains client private key file for SSL client authentication.
	// It's used to replace the `key` in native hcl backend configuration as we cannot use local file paths in the kubernetes cluster.
	Key                  crossplanetypes.CredentialsSource `json:"key_file_secret,omitempty" hcl:"key_file_secret"`
	Swauth               bool                              `json:"swauth,omitempty" hcl:"swauth"`
	AllowReauth          bool                              `json:"allow_reauth,omitempty" hcl:"allow_reauth"`
	Cloud                string                            `json:"cloud,omitempty" hcl:"cloud"`
	MAXRetries           int                               `json:"max_retries,omitempty" hcl:"max_retries"`
	DisableNoCacheHeader bool                              `json:"disable_no_cache_header,omitempty" hcl:"disable_no_cache_header"`
	// Deprecated: Use `container` instead.
	Path             string `json:"path,omitempty" hcl:"path"`
	Container        string `json:"container,omitempty" hcl:"container"`
	ArchivePath      string `json:"archive_path,omitempty" hcl:"archive_path"`
	ArchiveContainer string `json:"archive_container,omitempty" hcl:"archive_container"`
	ExpireAfter      string `json:"expire_after,omitempty" hcl:"expire_after"`
	Lock             bool   `json:"lock,omitempty" hcl:"lock"`
	StateName        string `json:"state_name,omitempty" hcl:"state_name"`
}
