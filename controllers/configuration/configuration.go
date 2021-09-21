package configuration

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/pkg/errors"
	v1 "k8s.io/api/core/v1"
	"k8s.io/klog/v2"

	"github.com/oam-dev/terraform-controller/api/types"
	"github.com/oam-dev/terraform-controller/api/v1beta1"
	git "github.com/oam-dev/terraform-controller/controllers/gitrepo"
	"github.com/oam-dev/terraform-controller/controllers/util"
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
	switch configurationType {
	case types.ConfigurationJSON:
		return configuration.Spec.JSON, nil
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
			return "", errors.Wrap(err, "failed to prepare Terraform backend configuration")
		}

		completedConfiguration := configuration.Spec.HCL + "\n" + backendTF
		return completedConfiguration, nil

	}

	return "", errors.New("unknown issue")
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

// checkTerraformSyntax checks the syntax error for a HCL/JSON configuration
func checkTerraformSyntax(name, configuration string) error {
	klog.InfoS("About to check the syntax issue", "configuration", configuration)
	dir, osErr := os.MkdirTemp("", fmt.Sprintf("tf-validate-%s-", name))
	if osErr != nil {
		klog.ErrorS(osErr, "Failed to create folder", "Dir", dir)
		return osErr
	}
	klog.InfoS("Validate dir", "Dir", dir)
	defer os.RemoveAll(dir) //nolint:errcheck
	tfFile := fmt.Sprintf("%s/main.tf", dir)
	if err := os.WriteFile(tfFile, []byte(configuration), 0777); err != nil { //nolint
		klog.ErrorS(err, "Failed to write Configuration hcl to main.tf", "HCL", configuration)
		return err
	}
	if err := os.Chdir(dir); err != nil {
		klog.ErrorS(err, "Failed to change dir", "dir", dir)
		return err
	}

	var (
		output []byte
		err    error
	)
	output, err = exec.Command("terraform", "init").CombinedOutput()
	if err != nil {
		klog.ErrorS(err, "The command execution isn't successful", "cmd", "terraform init", "output", string(output))
	} else {
		output, err = exec.Command("terraform", "validate").CombinedOutput()
		if err != nil {
			klog.ErrorS(err, "The command execution isn't successful", "cmd", "terraform validate", "output", string(output))
		}
	}
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
	case types.ConfigurationRemote:
		dir, err := os.MkdirTemp("", fmt.Sprintf("tf-remote-%s-", configuration.Name))
		if err != nil {
			klog.ErrorS(err, "Failed to create folder", "Dir", dir)
			return err
		}
		defer os.RemoveAll(dir) //nolint:errcheck
		if err := git.Clone(dir, configuration.Spec.Remote); err != nil {
			return err
		}

	}
	return checkTerraformSyntax(configuration.Name, template)
}
