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
	"github.com/oam-dev/terraform-controller/api/types"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	crossplanetypes "github.com/oam-dev/terraform-controller/api/types/crossplane-runtime"
)

// ProviderSpec defines the desired state of Provider.
type ProviderSpec struct {
	// Provider is the cloud service provider, like `alibaba`
	Provider string `json:"provider"`

	// Region is cloud provider's region
	Region string `json:"region,omitempty"`

	// Credentials required to authenticate to this provider.
	Credentials ProviderCredentials `json:"credentials"`
}

// ProviderCredentials required to authenticate.
type ProviderCredentials struct {
	// Source of the provider credentials.
	// +kubebuilder:validation:Enum=None;Secret;InjectedIdentity;Environment;Filesystem
	Source crossplanetypes.CredentialsSource `json:"source"`

	// A SecretRef is a reference to a secret key that contains the credentials
	// that must be used to connect to the provider.
	// +optional
	SecretRef *crossplanetypes.SecretKeySelector `json:"secretRef,omitempty"`
}

// ProviderStatus defines the observed state of Provider.
type ProviderStatus struct {
	State   types.ProviderState `json:"state,omitempty"`
	Message string              `json:"message,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="STATE",type="string",JSONPath=".status.state"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"

// Provider is the Schema for the providers API.
type Provider struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ProviderSpec   `json:"spec,omitempty"`
	Status ProviderStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ProviderList contains a list of Provider.
type ProviderList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Provider `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Provider{}, &ProviderList{})
}
