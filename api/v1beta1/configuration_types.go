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

package v1beta1

import (
	state "github.com/oam-dev/terraform-controller/api/types"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	types "github.com/oam-dev/terraform-controller/api/types/crossplane-runtime"
)

// ConfigurationSpec defines the desired state of Configuration
type ConfigurationSpec struct {
	// JSON is the Terraform JSON syntax configuration
	JSON string `json:"JSON,omitempty"`
	// HCL is the Terraform HCL type configuration
	HCL string `json:"hcl,omitempty"`
	// +kubebuilder:pruning:PreserveUnknownFields
	Variable *runtime.RawExtension `json:"variable,omitempty"`

	// Backend stores the state in a Kubernetes secret with locking done using a Lease resource.
	// TODO(zzxwill) If a backend exists in HCL/JSON, this can be optional. Currently, if Backend is not set by users, it
	// still will set by the controller, ignoring the settings in HCL/JSON backend
	Backend *Backend `json:"backend,omitempty"`

	// WriteConnectionSecretToReference specifies the namespace and name of a
	// Secret to which any connection details for this managed resource should
	// be written. Connection details frequently include the endpoint, username,
	// and password required to connect to the managed resource.
	// +optional
	WriteConnectionSecretToReference *types.SecretReference `json:"writeConnectionSecretToRef,omitempty"`

	// ProviderReference specifies the reference to Provider
	ProviderReference *types.Reference `json:"providerRef,omitempty"`
}

// ConfigurationStatus defines the observed state of Configuration
type ConfigurationStatus struct {
	State   state.ResourceState `json:"state,omitempty"`
	Message string              `json:"message,omitempty"`
	Outputs map[string]Property `json:"outputs,omitempty"`
}

type Property struct {
	Value string `json:"value,omitempty"`
	Type  string `json:"type,omitempty"`
}

// Backend stores the state in a Kubernetes secret with locking done using a Lease resource.
type Backend struct {
	// SecretSuffix used when creating secrets. Secrets will be named in the format: tfstate-{workspace}-{secretSuffix}
	SecretSuffix string `json:"secretSuffix,omitempty"`
	// InClusterConfig Used to authenticate to the cluster from inside a pod. Only `true` is allowed
	InClusterConfig bool `json:"inClusterConfig,omitempty"`
}

// +kubebuilder:object:root=true

// Configuration is the Schema for the configurations API
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="STATE",type="string",JSONPath=".status.state"
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
