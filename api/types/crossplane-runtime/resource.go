/*
Copyright 2019 The Crossplane Authors.

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

package types

// A CredentialsSource is a source from which provider credentials may be
// acquired.
type CredentialsSource string

const (
	// CredentialsSourceNone indicates that a provider does not require
	// credentials.
	CredentialsSourceNone CredentialsSource = "None"

	// CredentialsSourceSecret indicates that a provider should acquire
	// credentials from a secret.
	CredentialsSourceSecret CredentialsSource = "Secret"

	// CredentialsSourceInjectedIdentity indicates that a provider should use
	// credentials via its (pod's) identity; i.e. via IRSA for AWS,
	// Workload Identity for GCP, Pod Identity for Azure, or in-cluster
	// authentication for the Kubernetes API.
	CredentialsSourceInjectedIdentity CredentialsSource = "InjectedIdentity"
)

// A SecretKeySelector is a reference to a secret key in an arbitrary namespace.
type SecretKeySelector struct {
	SecretReference `json:",inline"`

	// The key to select.
	Key string `json:"key"`
}

// A SecretReference is a reference to a secret in an arbitrary namespace.
type SecretReference struct {
	// Name of the secret.
	Name string `json:"name"`

	// Namespace of the secret.
	Namespace string `json:"namespace,omitempty"`
}
