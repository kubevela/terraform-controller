package configuration

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/pkg/errors"
	v1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	apitypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/oam-dev/terraform-controller/api/types"
	crossplane "github.com/oam-dev/terraform-controller/api/types/crossplane-runtime"
	"github.com/oam-dev/terraform-controller/api/v1beta1"
	"github.com/oam-dev/terraform-controller/controllers/provider"
)

const (
	// GithubPrefix is the constant of GitHub domain
	GithubPrefix = "https://github.com/"
	// GithubKubeVelaContribPrefix is the prefix of GitHub repository of kubevela-contrib
	GithubKubeVelaContribPrefix = "https://github.com/kubevela-contrib"
	// GiteeTerraformSourceOrg is the Gitee organization of Terraform source
	GiteeTerraformSourceOrg = "https://gitee.com/kubevela-terraform-source"
	// GiteePrefix is the constant of Gitee domain
	GiteePrefix = "https://gitee.com/"
)

const errGitHubBlockedNotBoolean = "the value of githubBlocked is not a boolean"

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

// IsDeletable will check whether the Configuration can be deleted immediately
// If deletable, it means no external cloud resources are provisioned
func IsDeletable(ctx context.Context, k8sClient client.Client, configuration *v1beta1.Configuration) (bool, error) {
	providerRef := GetProviderNamespacedName(*configuration)
	providerObj, err := provider.GetProviderFromConfiguration(ctx, k8sClient, providerRef.Namespace, providerRef.Name)
	if err != nil {
		return false, err
	}
	// allow Configuration to delete when the Provider doesn't exist or is not ready, which means external cloud resources are
	// not provisioned at all
	if providerObj == nil || providerObj.Status.State == types.ProviderIsNotReady {
		return true, nil
	}

	if configuration.Status.Apply.State == types.ConfigurationProvisioningAndChecking {
		warning := fmt.Sprintf("Destroy could not complete and needs to wait for Provision to complete first: %s", types.MessageCloudResourceProvisioningAndChecking)
		klog.Warning(warning)
		return false, errors.New(warning)
	}

	return false, nil
}

// ReplaceTerraformSource will replace the Terraform source from GitHub to Gitee
func ReplaceTerraformSource(remote string, githubBlockedStr string) string {
	klog.InfoS("Whether GitHub is blocked", "githubBlocked", githubBlockedStr)
	githubBlocked, err := strconv.ParseBool(githubBlockedStr)
	if err != nil {
		klog.Warningf(errGitHubBlockedNotBoolean, err)
		return remote
	}
	klog.InfoS("Parsed GITHUB_BLOCKED env", "githubBlocked", githubBlocked)

	if !githubBlocked {
		return remote
	}

	if remote == "" {
		return ""
	}
	if strings.HasPrefix(remote, GithubPrefix) {
		var repo string
		if strings.HasPrefix(remote, GithubKubeVelaContribPrefix) {
			repo = strings.Replace(remote, GithubPrefix, GiteePrefix, 1)
		} else {
			tmp := strings.Split(strings.Replace(remote, GithubPrefix, "", 1), "/")
			if len(tmp) == 2 {
				repo = GiteeTerraformSourceOrg + "/" + tmp[1]
			}
		}
		klog.InfoS("New remote git", "Gitee", repo)
		return repo
	}
	return remote
}

// GetProviderNamespacedName will get the provider namespaced name
func GetProviderNamespacedName(configuration v1beta1.Configuration) *crossplane.Reference {
	if configuration.Spec.ProviderReference != nil {
		return configuration.Spec.ProviderReference
	}
	return &crossplane.Reference{
		Name:      provider.DefaultName,
		Namespace: provider.DefaultNamespace,
	}
}
