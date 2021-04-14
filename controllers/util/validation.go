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

func ValidConfiguration(configuration v1beta1.Configuration) (ConfigurationType, string, error) {
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
		return ConfigurationHCL, hcl, nil
	}
	return "", "", errors.New("unknown issue")
}
