package configuration

import (
	"context"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/pkg/errors"
	v1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	apitypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"

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
func RenderConfiguration(configuration *v1beta1.Configuration, terraformBackendNamespace string, configurationType types.ConfigurationType) (string, error) {
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
	backendTF, err := RenderTemplate(configuration.Spec.Backend, terraformBackendNamespace)
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

// CompareTwoContainerEnvs compares two slices of v1.EnvVar
func CompareTwoContainerEnvs(s1 []v1.EnvVar, s2 []v1.EnvVar) bool {
	less := func(env1 v1.EnvVar, env2 v1.EnvVar) bool {
		return env1.Name < env2.Name
	}
	return cmp.Diff(s1, s2, cmpopts.SortSlices(less)) == ""
}

// SetRegion will set the region for Configuration
func SetRegion(ctx context.Context, k8sClient client.Client, namespace, name string, providerObj *v1beta1.Provider) (string, error) {
	configuration, err := Get(ctx, k8sClient, apitypes.NamespacedName{Namespace: namespace, Name: name})
	if err != nil {
		return "", errors.Wrap(err, "failed to get configuration")
	}
	if configuration.Spec.Region != "" {
		return configuration.Spec.Region, nil
	}

	configuration.Spec.Region = providerObj.Spec.Region
	return providerObj.Spec.Region, Update(ctx, k8sClient, &configuration)
}

// Update will update the Configuration
func Update(ctx context.Context, k8sClient client.Client, configuration *v1beta1.Configuration) error {
	return k8sClient.Update(ctx, configuration)
}

// Get will get the Configuration
func Get(ctx context.Context, k8sClient client.Client, namespacedName apitypes.NamespacedName) (v1beta1.Configuration, error) {
	configuration := &v1beta1.Configuration{}
	if err := k8sClient.Get(ctx, namespacedName, configuration); err != nil {
		if kerrors.IsNotFound(err) {
			klog.ErrorS(err, "unable to fetch Configuration", "NamespacedName", namespacedName)
		}
		return *configuration, err
	}
	return *configuration, nil
}
