package types

const (
	// TerraformJSONConfigurationName is the file name for Terraform json Configuration
	TerraformJSONConfigurationName = "main.tf.json"
	// TerraformHCLConfigurationName is the file name for Terraform hcl Configuration
	TerraformHCLConfigurationName = "main.tf"
)

// ConfigurationType is the type for Terraform Configuration
type ConfigurationType string

const (
	// ConfigurationJSON is the json type Configuration
	ConfigurationJSON ConfigurationType = "JSON"
	// ConfigurationHCL is the HCL type Configuration
	ConfigurationHCL ConfigurationType = "HCL"
)
