package types

import "k8s.io/apimachinery/pkg/api/resource"

const (
	// TerraformHCLConfigurationName is the file name for Terraform hcl Configuration
	TerraformHCLConfigurationName = "main.tf"
)

// ConfigurationType is the type for Terraform Configuration
type ConfigurationType string

const (
	// ConfigurationHCL is the HCL type Configuration
	ConfigurationHCL ConfigurationType = "HCL"
	// ConfigurationRemote means HCL stores in a remote git repository
	ConfigurationRemote ConfigurationType = "Remote"
)

// Git represents Git configuration.
type Git struct {
	URL  string
	Path string
	Ref  GitRef
}

// GitRef specifies the git reference
type GitRef struct {
	Branch string `json:"branch,omitempty"`
	Tag    string `json:"tag,omitempty"`
	Commit string `json:"commit,omitempty"`
}

// ResourceQuota represents resource quotas for terraform jobs.
type ResourceQuota struct {
	ResourcesLimitsCPU              string
	ResourcesLimitsCPUQuantity      resource.Quantity
	ResourcesLimitsMemory           string
	ResourcesLimitsMemoryQuantity   resource.Quantity
	ResourcesRequestsCPU            string
	ResourcesRequestsCPUQuantity    resource.Quantity
	ResourcesRequestsMemory         string
	ResourcesRequestsMemoryQuantity resource.Quantity
}
