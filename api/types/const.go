package types

const (
	// WorkingVolumeMountPath is the mount path for working volume
	WorkingVolumeMountPath = "/data"
	// InputTFConfigurationVolumeName is the volume name for input Terraform Configuration
	InputTFConfigurationVolumeName = "tf-input-configuration"
	// BackendVolumeName is the volume name for Terraform backend
	BackendVolumeName = "tf-backend"
	// InputTFConfigurationVolumeMountPath is the volume mount path for input Terraform Configuration
	InputTFConfigurationVolumeMountPath = "/opt/tf-configuration"
	// BackendVolumeMountPath is the volume mount path for Terraform backend
	BackendVolumeMountPath = "/opt/tf-backend"

	GitCredsKnownHosts = "known_hosts"
	// Terraform credentials
	TerraformCredentials = "credentials.tfrc.json"
	// Terraform Registry Configuration
	TerraformRegistryConfig = ".terraformrc"
)

const (
	// TFInputConfigMapName is the CM name for Terraform Input Configuration
	TFInputConfigMapName = "tf-%s"
	// TFVariableSecret is the Secret name for variables, including credentials from Provider
	TFVariableSecret = "variable-%s"
)

// TerraformExecutionType is the type for Terraform execution
type TerraformExecutionType string

const (
	// TerraformApply is the name to mark `terraform apply`
	TerraformApply TerraformExecutionType = "apply"
	// TerraformDestroy is the name to mark `terraform destroy`
	TerraformDestroy TerraformExecutionType = "destroy"
)

const (
	// ClusterRoleName is the name of the ClusterRole for Terraform Job
	ClusterRoleName = "tf-executor-clusterrole"
	// ServiceAccountName is the name of the ServiceAccount for Terraform Job
	ServiceAccountName = "tf-executor-service-account"
)

const (
	DefaultNamespace = "default"
	// TerraformContainerName is the name of the container that executes terraform in the pod
	TerraformContainerName     = "terraform-executor"
	TerraformInitContainerName = "terraform-init"
	// GitAuthConfigVolumeName is the volume name for git auth configurtaion
	GitAuthConfigVolumeName = "git-auth-configuration"
	// GitAuthConfigVolumeMountPath is the volume mount path for git auth configurtaion
	GitAuthConfigVolumeMountPath = "/root/.ssh"
	// TerraformCredentialsConfigVolumeName is the volume name for terraform auth configurtaion
	TerraformCredentialsConfigVolumeName = "terraform-credentials-configuration"
	// TerraformCredentialsConfigVolumeMountPath is the volume mount path for terraform auth configurtaion
	TerraformCredentialsConfigVolumeMountPath = "/root/.terraform.d"
	// TerraformRCConfigVolumeName is the volume name of the terraform registry configuration
	TerraformRCConfigVolumeName = "terraform-rc-configuration"
	// TerraformRCConfigVolumeMountPath is the volume mount path for registry configuration
	TerraformRCConfigVolumeMountPath = "/root"
	// TerraformCredentialsHelperConfigVolumeName is the volume name for terraform auth configurtaion
	TerraformCredentialsHelperConfigVolumeName = "terraform-credentials-helper-configuration"
	// TerraformCredentialsHelperConfigVolumeMountPath is the volume mount path for terraform auth configurtaion
	TerraformCredentialsHelperConfigVolumeMountPath = "/root/.terraform.d/plugins"
)
