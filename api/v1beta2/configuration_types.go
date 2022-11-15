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
	"k8s.io/api/core/v1"
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
	// +kubebuilder:pruning:PreserveUnknownFields
	JobEnv *runtime.RawExtension `json:"JobEnv,omitempty"`
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

	// GitCredentialsReference specifies the reference to the secret containing the git credentials
	GitCredentialsReference *v1.SecretReference `json:"gitCredentialsReference,omitempty"`
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
	// Region is the region for the cloud resources created by this Configuration. If spec.region is not empty, it's the
	// value of it. Otherwise, it's the value of spec.providerReference.region.
	Region string `json:"region,omitempty"`
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
	// +kubebuilder:validation:Enum=kubernetes;s3
	BackendType string `json:"backendType,omitempty"`

	// Kubernetes is needed for the Terraform `kubernetes` backend type.
	Kubernetes *KubernetesBackendConf `json:"kubernetes,omitempty"`

	// S3 is needed for the Terraform `s3` backend type.
	S3 *S3BackendConf `json:"s3,omitempty"`
}

// KubernetesBackendConf defines all options supported by the Terraform `kubernetes` backend type.
// You can refer to https://www.terraform.io/language/settings/backends/kubernetes for the usage of each option.
type KubernetesBackendConf struct {
	SecretSuffix string  `json:"secret_suffix" hcl:"secret_suffix"`
	Namespace    *string `json:"namespace,omitempty" hcl:"namespace"`
}

// S3BackendConf defines all options supported by the Terraform `s3` backend type.
// You can refer to https://www.terraform.io/language/settings/backends/s3 for the usage of each option.
type S3BackendConf struct {
	// Region is optional, default to the AWS_DEFAULT_REGION in the credentials of the provider
	Region *string `json:"region,omitempty" hcl:"region"`
	Bucket string  `json:"bucket" hcl:"bucket"`
	Key    string  `json:"key" hcl:"key"`
}

// +kubebuilder:object:root=true

// Configuration is the Schema for the configurations API
// +kubebuilder:storageversion
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
