package types

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

// GitRef specifies the git reference
type GitRef struct {
	Branch string `json:"branch,omitempty"`
	Tag    string `json:"tag,omitempty"`
	Commit string `json:"commit,omitempty"`
}
