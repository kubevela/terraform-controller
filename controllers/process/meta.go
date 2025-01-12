package process

import (
	"github.com/oam-dev/terraform-controller/api/types"
	crossplane "github.com/oam-dev/terraform-controller/api/types/crossplane-runtime"
	"github.com/oam-dev/terraform-controller/api/v1beta2"
	tfcfg "github.com/oam-dev/terraform-controller/controllers/configuration"
	"github.com/oam-dev/terraform-controller/controllers/configuration/backend"
	"github.com/pkg/errors"
	v1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// LegacySubResources if user specify ControllerNamespace when re-staring controller, there are some sub-resources like Secret
// and ConfigMap that are in the namespace of the Configuration. We need to GC these sub-resources when Configuration is deleted.
type LegacySubResources struct {
	// Namespace is the namespace of the Configuration, also the namespace of the sub-resources.
	Namespace           string
	ApplyJobName        string
	DestroyJobName      string
	ConfigurationCMName string
	VariableSecretName  string
}

// TFConfigurationMeta is all the metadata of a Configuration
type TFConfigurationMeta struct {
	Name                                         string
	Namespace                                    string
	ControllerNamespace                          string
	ConfigurationType                            types.ConfigurationType
	CompleteConfiguration                        string
	Git                                          types.Git
	ConfigurationChanged                         bool
	EnvChanged                                   bool
	ConfigurationCMName                          string
	ApplyJobName                                 string
	DestroyJobName                               string
	Envs                                         []v1.EnvVar
	ProviderReference                            *crossplane.Reference
	VariableSecretName                           string
	VariableSecretData                           map[string][]byte
	DeleteResource                               bool
	Region                                       string
	Credentials                                  map[string]string
	JobEnv                                       map[string]interface{}
	GitCredentialsSecretReference                *v1.SecretReference
	TerraformCredentialsSecretReference          *v1.SecretReference
	TerraformRCConfigMapReference                *v1.SecretReference
	TerraformCredentialsHelperConfigMapReference *v1.SecretReference

	Backend backend.Backend
	// JobNodeSelector Expose the node selector of job to the controller level
	JobNodeSelector map[string]string

	// TerraformImage is the Terraform image which can run `terraform init/plan/apply`
	TerraformImage string
	BusyboxImage   string
	GitImage       string

	// JobAuthSecret is the secret name for pulling image in the Terraform job
	JobAuthSecret string

	// BackoffLimit specifies the number of retries to mark the Job as failed
	BackoffLimit int32

	// ResourceQuota series Variables are for Setting Compute Resources required by this container
	ResourceQuota types.ResourceQuota

	LegacySubResources    LegacySubResources
	ControllerNSSpecified bool

	K8sClient client.Client
}

// TFState is Terraform State
type TFState struct {
	Outputs map[string]TfStateProperty `json:"outputs"`
}

// TfStateProperty is the tf state property for an output
type TfStateProperty struct {
	Value interface{} `json:"value,omitempty"`
	Type  interface{} `json:"type,omitempty"`
}

// ToProperty converts TfStateProperty type to Property
func (tp *TfStateProperty) ToProperty() (v1beta2.Property, error) {
	var (
		property v1beta2.Property
		err      error
	)
	sv, err := tfcfg.Interface2String(tp.Value)
	if err != nil {
		return property, errors.Wrapf(err, "failed to convert value %s of terraform state outputs to string", tp.Value)
	}
	property = v1beta2.Property{
		Value: sv,
	}
	return property, err
}
