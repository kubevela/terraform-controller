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

// A ConfigurationState represents the status of a resource
type ConfigurationState string

// Reasons a resource is or is not ready.
const (
	ProviderNotReady                     ConfigurationState = "ProviderNotReady"
	ProviderReady                        ConfigurationState = "ProviderIsReady"
	ConfigurationStaticChecking          ConfigurationState = "SpecChecking"
	ConfigurationSyntaxError             ConfigurationState = "SyntaxError"
	ConfigurationSyntaxGood              ConfigurationState = "SyntaxGood"
	Available                            ConfigurationState = "Available"
	ConfigurationProvisioningAndChecking ConfigurationState = "ProvisioningAndChecking"
	ConfigurationDestroying              ConfigurationState = "Destroying"
)

// ProviderState is the type for Provider state
type ProviderState string

const (
	// ProviderIsReady is the `ready` state
	ProviderIsReady ProviderState = "ready"
	// ProviderIsInitializing marks the state of a Provider is initializing
	ProviderIsInitializing ProviderState = "initializing"
)
