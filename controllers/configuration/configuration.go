package configuration

import (
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/pkg/errors"
	v1 "k8s.io/api/core/v1"
	"k8s.io/klog/v2"

	"github.com/oam-dev/terraform-controller/api/types"
	"github.com/oam-dev/terraform-controller/api/v1beta1"
)

// ValidConfigurationObject will validate a Configuration
func ValidConfigurationObject(configuration *v1beta1.Configuration) (types.ConfigurationType, error) {
	json := configuration.Spec.JSON
	hcl := configuration.Spec.HCL
	remote := configuration.Spec.Remote
	switch {
	case json == "" && hcl == "" && remote == "":
		return "", errors.New("spec.JSON, spec.HCL or spec.Remote should be set")
	case json != "" && hcl != "", json != "" && remote != "", hcl != "" && remote != "":
		return "", errors.New("spec.JSON, spec.HCL and/or spec.Remote cloud not be set at the same time")
	case json != "":
		return types.ConfigurationJSON, nil
	case hcl != "":
		return types.ConfigurationHCL, nil
	case remote != "":
		return types.ConfigurationRemote, nil
	}
	return "", nil
}

// RenderConfiguration will compose the Terraform configuration with hcl/json and backend
func RenderConfiguration(configuration *v1beta1.Configuration, controllerNamespace string, configurationType types.ConfigurationType) (string, error) {
	if configuration.Spec.Backend != nil {
		if configuration.Spec.Backend.SecretSuffix == "" {
			configuration.Spec.Backend.SecretSuffix = configuration.Name
		}
		configuration.Spec.Backend.InClusterConfig = true
	} else {
		configuration.Spec.Backend = &v1beta1.Backend{
			SecretSuffix:    configuration.Name,
			InClusterConfig: true,
		}
	}
	backendTF, err := RenderTemplate(configuration.Spec.Backend, controllerNamespace)
	if err != nil {
		return "", errors.Wrap(err, "failed to prepare Terraform backend configuration")
	}

	switch configurationType {
	case types.ConfigurationJSON:
		return configuration.Spec.JSON, nil
	case types.ConfigurationHCL:
		completedConfiguration := configuration.Spec.HCL
		completedConfiguration += "\n" + backendTF
		return completedConfiguration, nil
	case types.ConfigurationRemote:
		return backendTF, nil
	default:
		return "", errors.New("Unsupported Configuration Type")
	}
}

// CheckWhetherConfigurationChanges will check whether configuration is changed
func CheckWhetherConfigurationChanges(configurationType types.ConfigurationType, cm *v1.ConfigMap, completedConfiguration string) (bool, error) {
	var configurationChanged bool
	switch configurationType {
	case types.ConfigurationJSON:
		return configurationChanged, nil
	case types.ConfigurationHCL:
		if cm != nil {
			configurationChanged = cm.Data[types.TerraformHCLConfigurationName] != completedConfiguration
			if configurationChanged {
				klog.InfoS("Configuration HCL changed", "ConfigMap", cm.Data[types.TerraformHCLConfigurationName],
					"RenderedCompletedConfiguration", completedConfiguration)
			}
		} else {
			// If the ConfigMap doesn't exist, we can surely say the configuration hcl/json changed
			configurationChanged = true
		}

		return configurationChanged, nil
	case types.ConfigurationRemote:
		return cm.Name == "", nil
	}

	return configurationChanged, errors.New("unknown issue")
}

// CompareTwoContainerEnvs compares two slices of v1.EnvVar
func CompareTwoContainerEnvs(s1 []v1.EnvVar, s2 []v1.EnvVar) bool {
	less := func(env1 v1.EnvVar, env2 v1.EnvVar) bool {
		return env1.Name < env2.Name
	}
	return cmp.Diff(s1, s2, cmpopts.SortSlices(less)) == ""
}
