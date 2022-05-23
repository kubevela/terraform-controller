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
	// This field is needed if the users use a git repo to provide the hcl files or
	// want to use their custom Terraform backend (instead of the default kubernetes backend type).
	// Notice: This field may cause two backend blocks in the final Terraform module and make the executor job failed.
	// So, please make sure that there are no backend configurations in your inline hcl code or the git repo.
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

// BackendType is an enum type for the Terraform backend types
// +kubebuilder:validation:Enum=remote;artifactory;azurerm;consul;cos;etcd;etcdv3;gcs;http;kubernetes;manta;oso;pg;s3;swift
type BackendType string

// Backend describes the Terraform backend configuration
type Backend struct {
	// SecretSuffix used when creating secrets. Secrets will be named in the format: tfstate-{workspace}-{secretSuffix}
	SecretSuffix string `json:"secretSuffix,omitempty"`
	// InClusterConfig Used to authenticate to the cluster from inside a pod. Only `true` is allowed
	InClusterConfig bool `json:"inClusterConfig,omitempty"`

	// Inline allows users to use raw hcl code to specify their Terraform backend
	Inline string `json:"inline,omitempty"`

	// BackendType indicates which backend type to use. This field is needed for custom backend configuration.
	BackendType BackendType `json:"backendType,omitempty"`

	// Kubernetes is needed for the Terraform `kubernetes` backend type.
	Kubernetes *KubernetesBackendConf `json:"kubernetes,omitempty"`
}

// CurrentNSSecretSelector is used to specify the key in a secret in the current namespace.
type CurrentNSSecretSelector struct {
	// Name is the name of the secret
	Name string `json:"name"`
	// Key is the key selected in the secret
	Key string `json:"key"`
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
