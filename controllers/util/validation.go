package util

import (
	"github.com/pkg/errors"

	"github.com/oam-dev/terraform-controller/api/v1beta1"
)

type ConfigurationType string

const (
	ConfigurationJSON ConfigurationType = "JSON"
	ConfigurationHCL  ConfigurationType = "HCL"
)

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
