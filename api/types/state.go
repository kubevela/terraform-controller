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
	Authorizing                          ConfigurationState = "Authorizing"
	ProviderNotFound                     ConfigurationState = "ProviderNotFound"
	ProviderNotReady                     ConfigurationState = "ProviderNotReady"
	ConfigurationStaticCheckFailed       ConfigurationState = "ConfigurationSpecNotValid"
	Available                            ConfigurationState = "Available"
	ConfigurationProvisioningAndChecking ConfigurationState = "ProvisioningAndChecking"
	ConfigurationDestroying              ConfigurationState = "Destroying"
	ConfigurationApplyFailed             ConfigurationState = "ApplyFailed"
	ConfigurationDestroyFailed           ConfigurationState = "DestroyFailed"
	ConfigurationReloading               ConfigurationState = "ConfigurationReloading"
	GeneratingOutputs                    ConfigurationState = "GeneratingTerraformOutputs"
	InvalidRegion                        ConfigurationState = "InvalidRegion"
	TerraformInitError                   ConfigurationState = "TerraformInitError"
)

// Stage is the Terraform stage
type Stage string

const (
	TerraformInit  Stage = "TerraformInit"
	TerraformApply Stage = "TerraformApply"
)

const (
	// MessageDestroyJobNotCompleted is the message when Configuration deletion isn't completed
	MessageDestroyJobNotCompleted = "Configuration deletion isn't completed"
	// MessageApplyJobNotCompleted is the message when cloud resources are not created completed
	MessageApplyJobNotCompleted = "cloud resources are not created completed"
	// MessageCloudResourceProvisioningAndChecking is the message when cloud resource is being provisioned
	MessageCloudResourceProvisioningAndChecking = "Cloud resources are being provisioned and provisioning status is checking..."
	// ErrUpdateTerraformApplyJob means hitting  an issue to update Terraform apply job
	ErrUpdateTerraformApplyJob = "Hit an issue to update Terraform apply job"
	// MessageCloudResourceDeployed means Cloud resources are deployed and ready to use
	MessageCloudResourceDeployed = "Cloud resources are deployed and ready to use"
	// MessageCloudResourceDestroying is the message when cloud resource is being destroyed
	MessageCloudResourceDestroying = "Cloud resources is being destroyed..."
	// ErrProviderNotFound means provider not found
	ErrProviderNotFound = "provider not found"
	// ErrProviderNotReady means provider object is not ready
	ErrProviderNotReady = "Provider is not ready"
	// ConfigurationReloadingAsHCLChanged means Configuration changed and needs reloading
	ConfigurationReloadingAsHCLChanged = "Configuration's HCL has changed, and starts reloading"
	// ConfigurationReloadingAsVariableChanged means Configuration changed and needs reloading
	ConfigurationReloadingAsVariableChanged = "Configuration's variable has changed, and starts reloading"
	// ErrGenerateOutputs means error to generate outputs
	ErrGenerateOutputs = "Hit an issue to generate outputs"
)

// ProviderState is the type for Provider state
type ProviderState string

const (
	// ProviderIsReady is the `ready` state
	ProviderIsReady ProviderState = "ready"
	// ProviderIsNotReady marks the state of a Provider is not ready
	ProviderIsNotReady ProviderState = "ProviderNotReady"
)
