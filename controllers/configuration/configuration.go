package configuration

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/oam-dev/terraform-controller/controllers/util"
	"github.com/pkg/errors"
	v1 "k8s.io/api/core/v1"

	"github.com/oam-dev/terraform-controller/api/types"
	"github.com/oam-dev/terraform-controller/api/v1beta1"
)

// ValidConfigurationObject will validate a Configuration
func ValidConfigurationObject(configuration *v1beta1.Configuration) (types.ConfigurationType, error) {
	json := configuration.Spec.JSON
	hcl := configuration.Spec.HCL
	switch {
	case json == "" && hcl == "":
		return "", errors.New("spec.JSON or spec.HCL should be set")
	case json != "" && hcl != "":
		return "", errors.New("spec.JSON and spec.HCL cloud not be set at the same time")
	case json != "":
		return types.ConfigurationJSON, nil
	case hcl != "":
		return types.ConfigurationHCL, nil
	}
	return "", nil
}

// ComposeConfiguration will compose the Terraform configuration with hcl/json and backend
// and will also check whether configuration is changed
func ComposeConfiguration(configuration *v1beta1.Configuration, controllerNamespace string,
	configurationType types.ConfigurationType, cm *v1.ConfigMap) (string, bool, error) {
	var configurationChanged bool
	switch configurationType {
	case types.ConfigurationJSON:
		return configuration.Spec.JSON, configurationChanged, nil
	case types.ConfigurationHCL:
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
		backendTF, err := util.RenderTemplate(configuration.Spec.Backend, controllerNamespace)
		if err != nil {
			return "", configurationChanged, errors.Wrap(err, "failed to prepare Terraform backend configuration")
		}

		completedConfiguration := configuration.Spec.HCL + "\n" + backendTF

		if cm != nil {
			configurationChanged = cm.Data[types.TerraformHCLConfigurationName] != completedConfiguration
		} else {
			// If the ConfigMap doesn't exist, we can surely say the configuration hcl/json changed
			configurationChanged = true
		}

		return completedConfiguration, configurationChanged, nil

	}

	return "", configurationChanged, errors.New("unknown issue")
}

// CompareTwoContainerEnvs compares two slices of v1.EnvVar
func CompareTwoContainerEnvs(s1 []v1.EnvVar, s2 []v1.EnvVar) bool {
	less := func(env1 v1.EnvVar, env2 v1.EnvVar) bool {
		return env1.Name < env2.Name
	}
	return cmp.Diff(s1, s2, cmpopts.SortSlices(less)) == ""
}

// checkTerraformSyntax checks the syntax error for a HCL/JSON configuration
func checkTerraformSyntax(configuration string) error {
	abs, _ := filepath.Abs(".")
	var terraform = "terraform/darwin/terraform"
	switch runtime.GOOS {
	case "linux":
		terraform = "terraform/linux/terraform"
	case "windows":
		terraform = "terraform/windows/terraform.exe"
	}
	terraform = filepath.Join(abs, "controllers", "configuration", terraform)
	dir, _ := os.MkdirTemp(".", "tf-validate-")
	defer os.RemoveAll(dir) //nolint:errcheck
	tfFile := fmt.Sprintf("%s/main.tf", dir)
	if err := os.WriteFile(tfFile, []byte(configuration), 0400); err != nil {
		return err
	}
	cmd := fmt.Sprintf("cd %s && %s init && %s validate", dir, terraform, terraform)
	output, _ := exec.Command("bash", "-c", cmd).CombinedOutput() //nolint:gosec
	if strings.Contains(string(output), "Success!") {
		return nil
	}
	return errors.New(string(output))
}

// CheckConfigurationSyntax checks the syntax of Configuration
func CheckConfigurationSyntax(configuration *v1beta1.Configuration, configurationType types.ConfigurationType) error {
	var template string
	switch configurationType {
	case types.ConfigurationHCL:
		template = configuration.Spec.HCL
	case types.ConfigurationJSON:
		template = configuration.Spec.JSON
	}
	return checkTerraformSyntax(template)
}
