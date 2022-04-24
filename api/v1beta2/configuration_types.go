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

	// BackendReference specifies the reference to Backend
	BackendReference *types.Reference `json:"backendRef,omitempty"`

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
	// Deprecated: use the `type` and `config` instead
	SecretSuffix string `json:"secretSuffix,omitempty"`
	// InClusterConfig Used to authenticate to the cluster from inside a pod. Only `true` is allowed
	// Deprecated: use the `type` and `config` instead
	InClusterConfig bool `json:"inClusterConfig,omitempty"`

	// HCL allows users to use raw hcl code to specify their Terraform backend configuration
	HCL string `json:"hcl,omitempty"`

	// Type specifies the Terraform backend type, for example, "kubernetes", "s3", etc
	Type string `json:"type,omitempty"`
	// Config is the detail configurations of the Terraform backend,
	// and it represents the key-value pairs inside the terraform backend block in the hcl files
	Config *runtime.RawExtension `json:"config,omitempty"`
	// Workspace is used only when the users use the "remote" backend type
	Workspace *runtime.RawExtension `json:"workspace,omitempty"`
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
