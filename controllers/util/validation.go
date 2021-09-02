package util

import (
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/pkg/errors"
	v1 "k8s.io/api/core/v1"

	"github.com/oam-dev/terraform-controller/api/v1beta1"
)

// ConfigurationType is the type for Terraform Configuration
type ConfigurationType string

const (
	// ConfigurationJSON is the json type Configuration
	ConfigurationJSON ConfigurationType = "JSON"
	// ConfigurationHCL is the HCL type Configuration
	ConfigurationHCL ConfigurationType = "HCL"
)

// ValidConfiguration will validate a Configuration
func ValidConfiguration(configuration *v1beta1.Configuration, controllerNamespace string) (ConfigurationType, string, error) {
	json := configuration.Spec.JSON
	hcl := configuration.Spec.HCL
	switch {
	case json == "" && hcl == "":
		return "", "", errors.New("spec.JSON or spec.HCL should be set")
	case json != "" && hcl != "":
		return "", "", errors.New("spec.JSON and spec.HCL cloud not be set at the same time")
	case json != "":
		return ConfigurationJSON, json, nil
	case hcl != "":
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
		backendTF, err := renderTemplate(configuration.Spec.Backend, controllerNamespace)
		if err != nil {
			return "", "", errors.Wrap(err, "failed to prepare Terraform backend configuration")
		}
		return ConfigurationHCL, hcl + "\n" + backendTF, nil
	}
	return "", "", errors.New("unknown issue")
}

// CompareTwoContainerEnvs compares two slices of v1.EnvVar
func CompareTwoContainerEnvs(s1 []v1.EnvVar, s2 []v1.EnvVar) bool {
	less := func(env1 v1.EnvVar, env2 v1.EnvVar) bool {
		return env1.Name < env2.Name
	}
	return cmp.Diff(s1, s2, cmpopts.SortSlices(less)) == ""
}
