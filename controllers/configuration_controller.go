/*
Copyright 2021 The KubeVela Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controllers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"time"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	batchv1 "k8s.io/api/batch/v1"
	v1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/util/feature"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/oam-dev/terraform-controller/api/types"
	crossplane "github.com/oam-dev/terraform-controller/api/types/crossplane-runtime"
	"github.com/oam-dev/terraform-controller/api/v1beta1"
	"github.com/oam-dev/terraform-controller/api/v1beta2"
	tfcfg "github.com/oam-dev/terraform-controller/controllers/configuration"
	"github.com/oam-dev/terraform-controller/controllers/configuration/backend"
	"github.com/oam-dev/terraform-controller/controllers/features"
	"github.com/oam-dev/terraform-controller/controllers/provider"
	"github.com/oam-dev/terraform-controller/controllers/terraform"
)

const (
	defaultNamespace = "default"
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
	// terraformContainerName is the name of the container that executes the terraform in the pod
	terraformContainerName     = "terraform-executor"
	terraformInitContainerName = "terraform-init"
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
	configurationFinalizer = "configuration.finalizers.terraform-controller"
	// ClusterRoleName is the name of the ClusterRole for Terraform Job
	ClusterRoleName = "tf-executor-clusterrole"
	// ServiceAccountName is the name of the ServiceAccount for Terraform Job
	ServiceAccountName = "tf-executor-service-account"
)

const (
	GitCredsKnownHosts = "known_hosts"
	// Terraform credentials
	TerraformCredentials = "credentials.tfrc.json"
	// Terraform Registry Configuration
	TerraformRegistryConfig = ".terraformrc"
)

// ConfigurationReconciler reconciles a Configuration object.
type ConfigurationReconciler struct {
	client.Client
	Log                 logr.Logger
	ControllerNamespace string
	ProviderName        string
	Scheme              *runtime.Scheme
}

// +kubebuilder:rbac:groups=terraform.core.oam.dev,resources=configurations,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=terraform.core.oam.dev,resources=configurations/status,verbs=get;update;patch

// Reconcile will reconcile periodically
func (r *ConfigurationReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	klog.InfoS("reconciling Terraform Configuration...", "NamespacedName", req.NamespacedName)

	configuration, err := tfcfg.Get(ctx, r.Client, req.NamespacedName)
	if err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	meta := initTFConfigurationMeta(req, configuration, r.Client)
	if r.ControllerNamespace != "" {
		uid := string(configuration.GetUID())
		// @step: since we are using a single namespace to run these, we must ensure the names
		// are unique across the namespace
		meta.KeepLegacySubResourceMetas()
		meta.ApplyJobName = uid + "-" + string(TerraformApply)
		meta.DestroyJobName = uid + "-" + string(TerraformDestroy)
		meta.ConfigurationCMName = fmt.Sprintf(TFInputConfigMapName, uid)
		meta.VariableSecretName = fmt.Sprintf(TFVariableSecret, uid)
		meta.ControllerNamespace = r.ControllerNamespace
		meta.ControllerNSSpecified = true
	}

	// add finalizer
	var isDeleting = !configuration.ObjectMeta.DeletionTimestamp.IsZero()
	if !isDeleting {
		if !controllerutil.ContainsFinalizer(&configuration, configurationFinalizer) {
			controllerutil.AddFinalizer(&configuration, configurationFinalizer)
			if err := r.Update(ctx, &configuration); err != nil {
				return ctrl.Result{RequeueAfter: 3 * time.Second}, errors.Wrap(err, "failed to add finalizer")
			}
		}
	}

	// pre-check Configuration
	if err := r.preCheck(ctx, &configuration, meta); err != nil && !isDeleting {
		return ctrl.Result{}, err
	}

	if isDeleting {
		// terraform destroy
		klog.InfoS("performing Configuration Destroy", "Namespace", req.Namespace, "Name", req.Name, "JobName", meta.DestroyJobName)
		// if allow to delete halfway, we will not check the status of the apply job.

		_, err := terraform.GetTerraformStatus(ctx, meta.ControllerNamespace, meta.DestroyJobName, terraformContainerName, terraformInitContainerName)
		if err != nil {
			klog.ErrorS(err, "Terraform destroy failed")
			if updateErr := meta.updateDestroyStatus(ctx, r.Client, types.ConfigurationDestroyFailed, err.Error()); updateErr != nil {
				return ctrl.Result{}, updateErr
			}
		}

		// If no tfState has been generated, then perform a quick cleanup without dispatching destroying job.
		if meta.isTFStateGenerated(ctx) {
			if err := r.terraformDestroy(ctx, configuration, meta); err != nil {
				if err.Error() == types.MessageDestroyJobNotCompleted {
					return ctrl.Result{RequeueAfter: 3 * time.Second}, nil
				}
				return ctrl.Result{RequeueAfter: 3 * time.Second}, errors.Wrap(err, "continue reconciling to destroy cloud resource")
			}
		} else {
			klog.Infof("No need to execute terraform destroy command, because tfstate file not found: %s/%s", configuration.Namespace, configuration.Name)
			if err := r.cleanUpSubResources(ctx, configuration, meta); err != nil {
				klog.Warningf("Ignoring error when clean up sub-resources, for no resource is actually created: %s", err)
			}
		}

		configuration, err := tfcfg.Get(ctx, r.Client, req.NamespacedName)
		if err != nil {
			return ctrl.Result{}, client.IgnoreNotFound(err)
		}
		if controllerutil.ContainsFinalizer(&configuration, configurationFinalizer) {
			controllerutil.RemoveFinalizer(&configuration, configurationFinalizer)
			if err := r.Update(ctx, &configuration); err != nil {
				return ctrl.Result{RequeueAfter: 3 * time.Second}, errors.Wrap(err, "failed to remove finalizer")
			}
		}
		return ctrl.Result{}, nil
	}

	var tfExecutionJob = &batchv1.Job{}
	if err := meta.getApplyJob(ctx, r.Client, tfExecutionJob); err == nil {
		if !meta.EnvChanged && tfExecutionJob.Status.Succeeded == int32(1) {
			err = meta.updateApplyStatus(ctx, r.Client, types.Available, types.MessageCloudResourceDeployed)
			return ctrl.Result{}, err
		}
	}

	// Terraform apply (create or update)
	klog.InfoS("performing Terraform Apply (cloud resource create/update)", "Namespace", req.Namespace, "Name", req.Name)
	if err := r.terraformApply(ctx, configuration, meta); err != nil {
		if err.Error() == types.MessageApplyJobNotCompleted {
			return ctrl.Result{RequeueAfter: 3 * time.Second}, nil
		}
		return ctrl.Result{RequeueAfter: 3 * time.Second}, errors.Wrap(err, "failed to create/update cloud resource")
	}
	state, err := terraform.GetTerraformStatus(ctx, meta.ControllerNamespace, meta.ApplyJobName, terraformContainerName, terraformInitContainerName)
	if err != nil {
		klog.ErrorS(err, "Terraform apply failed")
		if updateErr := meta.updateApplyStatus(ctx, r.Client, state, err.Error()); updateErr != nil {
			return ctrl.Result{}, updateErr
		}

		return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
	}

	return ctrl.Result{}, nil
}

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

type ResourceQuota struct {
	ResourcesLimitsCPU              string
	ResourcesLimitsCPUQuantity      resource.Quantity
	ResourcesLimitsMemory           string
	ResourcesLimitsMemoryQuantity   resource.Quantity
	ResourcesRequestsCPU            string
	ResourcesRequestsCPUQuantity    resource.Quantity
	ResourcesRequestsMemory         string
	ResourcesRequestsMemoryQuantity resource.Quantity
}

// TFConfigurationMeta is all the metadata of a Configuration
type TFConfigurationMeta struct {
	Name                                         string
	Namespace                                    string
	ControllerNamespace                          string
	ConfigurationType                            types.ConfigurationType
	CompleteConfiguration                        string
	RemoteGit                                    string
	RemoteGitPath                                string
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

	// ResourceQuota series Variables are for Setting Compute Resources required by this container
	ResourceQuota ResourceQuota

	LegacySubResources    LegacySubResources
	ControllerNSSpecified bool

	K8sClient client.Client
}

func initTFConfigurationMeta(req ctrl.Request, configuration v1beta2.Configuration, k8sClient client.Client) *TFConfigurationMeta {
	var meta = &TFConfigurationMeta{
		ControllerNamespace: req.Namespace,
		Namespace:           req.Namespace,
		Name:                req.Name,
		ConfigurationCMName: fmt.Sprintf(TFInputConfigMapName, req.Name),
		VariableSecretName:  fmt.Sprintf(TFVariableSecret, req.Name),
		ApplyJobName:        req.Name + "-" + string(TerraformApply),
		DestroyJobName:      req.Name + "-" + string(TerraformDestroy),
		K8sClient:           k8sClient,
	}

	jobNodeSelectorStr := os.Getenv("JOB_NODE_SELECTOR")
	if jobNodeSelectorStr != "" {
		err := json.Unmarshal([]byte(jobNodeSelectorStr), &meta.JobNodeSelector)
		if err != nil {
			klog.Warningf("the value of JobNodeSelector is not a json string ", err)
		}
	}

	// githubBlocked mark whether GitHub is blocked in the cluster
	githubBlockedStr := os.Getenv("GITHUB_BLOCKED")
	if githubBlockedStr == "" {
		githubBlockedStr = "false"
	}

	meta.RemoteGit = tfcfg.ReplaceTerraformSource(configuration.Spec.Remote, githubBlockedStr)
	if configuration.Spec.DeleteResource != nil {
		meta.DeleteResource = *configuration.Spec.DeleteResource
	} else {
		meta.DeleteResource = true
	}
	if configuration.Spec.Path == "" {
		meta.RemoteGitPath = "."
	} else {
		meta.RemoteGitPath = configuration.Spec.Path
	}

	if !configuration.Spec.InlineCredentials {
		meta.ProviderReference = tfcfg.GetProviderNamespacedName(configuration)
	}

	if configuration.Spec.GitCredentialsSecretReference != nil {
		meta.GitCredentialsSecretReference = configuration.Spec.GitCredentialsSecretReference
	}

	if configuration.Spec.TerraformCredentialsSecretReference != nil {
		meta.TerraformCredentialsSecretReference = configuration.Spec.TerraformCredentialsSecretReference
	}

	if configuration.Spec.TerraformRCConfigMapReference != nil {
		meta.TerraformRCConfigMapReference = configuration.Spec.TerraformRCConfigMapReference
	}

	if configuration.Spec.TerraformCredentialsHelperConfigMapReference != nil {
		meta.TerraformCredentialsHelperConfigMapReference = configuration.Spec.TerraformCredentialsHelperConfigMapReference
	}

	return meta
}

func (r *ConfigurationReconciler) terraformApply(ctx context.Context, configuration v1beta2.Configuration, meta *TFConfigurationMeta) error {

	var (
		k8sClient      = r.Client
		tfExecutionJob batchv1.Job
	)

	if err := meta.getApplyJob(ctx, k8sClient, &tfExecutionJob); err != nil {
		if kerrors.IsNotFound(err) {
			return meta.assembleAndTriggerJob(ctx, k8sClient, TerraformApply)
		}
	}
	klog.InfoS("terraform apply job", "Namespace", tfExecutionJob.Namespace, "Name", tfExecutionJob.Name)

	if err := meta.updateTerraformJobIfNeeded(ctx, k8sClient, tfExecutionJob); err != nil {
		klog.ErrorS(err, types.ErrUpdateTerraformApplyJob, "Name", meta.ApplyJobName)
		return errors.Wrap(err, types.ErrUpdateTerraformApplyJob)
	}

	if !meta.EnvChanged && tfExecutionJob.Status.Succeeded == int32(1) {
		if err := meta.updateApplyStatus(ctx, k8sClient, types.Available, types.MessageCloudResourceDeployed); err != nil {
			return err
		}
	} else {
		// start provisioning and check the status of the provision
		// If the state is types.InvalidRegion, no need to continue checking
		if configuration.Status.Apply.State != types.ConfigurationProvisioningAndChecking &&
			configuration.Status.Apply.State != types.InvalidRegion {
			if err := meta.updateApplyStatus(ctx, r.Client, types.ConfigurationProvisioningAndChecking, types.MessageCloudResourceProvisioningAndChecking); err != nil {
				return err
			}
		}
	}
	return nil
}

func (r *ConfigurationReconciler) terraformDestroy(ctx context.Context, configuration v1beta2.Configuration, meta *TFConfigurationMeta) error {
	var (
		destroyJob batchv1.Job
		k8sClient  = r.Client
	)

	deletable, err := tfcfg.IsDeletable(ctx, k8sClient, &configuration)
	if err != nil {
		return err
	}

	// Sub-resources can be deleted directly without waiting destroy job is done means:
	// - Configuration is deletable (no cloud resources are provisioned or force delete is set) WHEN not allow to delete halfway.
	// - OR user want to keep the resource when delete the configuration CR
	// If allowed to delete halfway, there could be parts of cloud resources are provisioned, so we need to wait destroy job is done.
	notWaitingDestroyJob := deletable && !feature.DefaultFeatureGate.Enabled(features.AllowDeleteProvisioningResource) || !meta.DeleteResource
	// If the configuration is deletable, and it is caused by AllowDeleteProvisioningResource feature, the apply job may be still running, we should clean it first to avoid data race.
	needCleanApplyJob := deletable && feature.DefaultFeatureGate.Enabled(features.AllowDeleteProvisioningResource)

	if !notWaitingDestroyJob {
		if needCleanApplyJob {
			err := deleteApplyJob(ctx, meta, k8sClient)
			if err != nil {
				return err
			}
		}
		if err := k8sClient.Get(ctx, client.ObjectKey{Name: meta.DestroyJobName, Namespace: meta.ControllerNamespace}, &destroyJob); err != nil {
			if kerrors.IsNotFound(err) {
				if err := r.Client.Get(ctx, client.ObjectKey{Name: configuration.Name, Namespace: configuration.Namespace}, &v1beta2.Configuration{}); err == nil {
					if err = meta.assembleAndTriggerJob(ctx, k8sClient, TerraformDestroy); err != nil {
						return err
					}
				}
			}
		}
		if err := meta.updateTerraformJobIfNeeded(ctx, k8sClient, destroyJob); err != nil {
			klog.ErrorS(err, types.ErrUpdateTerraformApplyJob, "Name", meta.DestroyJobName)
			return errors.Wrap(err, types.ErrUpdateTerraformApplyJob)
		}
	}

	// destroying
	if err := meta.updateDestroyStatus(ctx, k8sClient, types.ConfigurationDestroying, types.MessageCloudResourceDestroying); err != nil {
		return err
	}

	if configuration.Spec.ForceDelete != nil && *configuration.Spec.ForceDelete {
		// Try to clean up as more sub-resources as possible. Ignore the issues if it hit any.
		if err := r.cleanUpSubResources(ctx, configuration, meta); err != nil {
			klog.Warningf("Failed to clean up sub-resources, but it's ignored as the resources are being forced to delete: %s", err)
		}
		return nil
	}
	// When the deletion Job process succeeded, clean up work is starting.
	if !notWaitingDestroyJob {
		if err := k8sClient.Get(ctx, client.ObjectKey{Name: meta.DestroyJobName, Namespace: meta.ControllerNamespace}, &destroyJob); err != nil {
			return err
		}
		if destroyJob.Status.Succeeded == int32(1) {
			return r.cleanUpSubResources(ctx, configuration, meta)
		}
	} else {
		return r.cleanUpSubResources(ctx, configuration, meta)
	}

	return errors.New(types.MessageDestroyJobNotCompleted)
}

func (r *ConfigurationReconciler) cleanUpSubResources(ctx context.Context, configuration v1beta2.Configuration, meta *TFConfigurationMeta) error {
	var k8sClient = r.Client

	// 1. delete connectionSecret
	if configuration.Spec.WriteConnectionSecretToReference != nil {
		secretName := configuration.Spec.WriteConnectionSecretToReference.Name
		secretNameSpace := configuration.Spec.WriteConnectionSecretToReference.Namespace
		if err := deleteConnectionSecret(ctx, k8sClient, secretName, secretNameSpace); err != nil {
			return err
		}
	}

	// 2~5. delete the jobs, variable secrets, configuration configmaps
	type cleanupResourceFunc func(ctx context.Context, meta *TFConfigurationMeta, k8sClient client.Client) error
	resourceToCleanup := []cleanupResourceFunc{
		deleteApplyJob,
		deleteDestroyJob,
		deleteVariableSecret,
		deleteConfigMap,
	}
	for _, clean := range resourceToCleanup {
		if err := clean(ctx, meta, k8sClient); err != nil {
			return err
		}
	}

	// 6. delete Kubernetes backend secret
	if meta.Backend != nil {
		if err := meta.Backend.CleanUp(ctx); err != nil {
			return err
		}
	}

	return nil
}

func (r *ConfigurationReconciler) preCheckResourcesSetting(meta *TFConfigurationMeta) error {

	meta.ResourceQuota.ResourcesLimitsCPU = os.Getenv("RESOURCES_LIMITS_CPU")
	if meta.ResourceQuota.ResourcesLimitsCPU != "" {
		limitsCPU, err := resource.ParseQuantity(meta.ResourceQuota.ResourcesLimitsCPU)
		if err != nil {
			errMsg := "failed to parse env variable RESOURCES_LIMITS_CPU into resource.Quantity"
			klog.ErrorS(err, errMsg)
			return errors.Wrap(err, errMsg)
		}
		meta.ResourceQuota.ResourcesLimitsCPUQuantity = limitsCPU
	}
	meta.ResourceQuota.ResourcesLimitsMemory = os.Getenv("RESOURCES_LIMITS_MEMORY")
	if meta.ResourceQuota.ResourcesLimitsMemory != "" {
		limitsMemory, err := resource.ParseQuantity(meta.ResourceQuota.ResourcesLimitsMemory)
		if err != nil {
			errMsg := "failed to parse env variable RESOURCES_LIMITS_MEMORY into resource.Quantity"
			klog.ErrorS(err, errMsg)
			return errors.Wrap(err, errMsg)
		}
		meta.ResourceQuota.ResourcesLimitsMemoryQuantity = limitsMemory
	}
	meta.ResourceQuota.ResourcesRequestsCPU = os.Getenv("RESOURCES_REQUESTS_CPU")
	if meta.ResourceQuota.ResourcesRequestsCPU != "" {
		requestsCPU, err := resource.ParseQuantity(meta.ResourceQuota.ResourcesRequestsCPU)
		if err != nil {
			errMsg := "failed to parse env variable RESOURCES_REQUESTS_CPU into resource.Quantity"
			klog.ErrorS(err, errMsg)
			return errors.Wrap(err, errMsg)
		}
		meta.ResourceQuota.ResourcesRequestsCPUQuantity = requestsCPU
	}
	meta.ResourceQuota.ResourcesRequestsMemory = os.Getenv("RESOURCES_REQUESTS_MEMORY")
	if meta.ResourceQuota.ResourcesRequestsMemory != "" {
		requestsMemory, err := resource.ParseQuantity(meta.ResourceQuota.ResourcesRequestsMemory)
		if err != nil {
			errMsg := "failed to parse env variable RESOURCES_REQUESTS_MEMORY into resource.Quantity"
			klog.ErrorS(err, errMsg)
			return errors.Wrap(err, errMsg)
		}
		meta.ResourceQuota.ResourcesRequestsMemoryQuantity = requestsMemory
	}
	return nil
}

func (r *ConfigurationReconciler) preCheck(ctx context.Context, configuration *v1beta2.Configuration, meta *TFConfigurationMeta) error {
	var k8sClient = r.Client

	meta.TerraformImage = os.Getenv("TERRAFORM_IMAGE")
	if meta.TerraformImage == "" {
		meta.TerraformImage = "oamdev/docker-terraform:1.1.2"
	}

	meta.BusyboxImage = os.Getenv("BUSYBOX_IMAGE")
	if meta.BusyboxImage == "" {
		meta.BusyboxImage = "busybox:latest"
	}
	meta.GitImage = os.Getenv("GIT_IMAGE")
	if meta.GitImage == "" {
		meta.GitImage = "alpine/git:latest"
	}

	if err := r.preCheckResourcesSetting(meta); err != nil {
		return err
	}

	// Validation: 1) validate Configuration itself
	configurationType, err := tfcfg.ValidConfigurationObject(configuration)
	if err != nil {
		if updateErr := meta.updateApplyStatus(ctx, k8sClient, types.ConfigurationStaticCheckFailed, err.Error()); updateErr != nil {
			return updateErr
		}
		return err
	}
	meta.ConfigurationType = configurationType

	// Check provider
	if !configuration.Spec.InlineCredentials {
		p, err := provider.GetProviderFromConfiguration(ctx, k8sClient, meta.ProviderReference.Namespace, meta.ProviderReference.Name)
		if p == nil {
			msg := types.ErrProviderNotFound
			if err != nil {
				msg = err.Error()
			}
			if updateStatusErr := meta.updateApplyStatus(ctx, k8sClient, types.Authorizing, msg); updateStatusErr != nil {
				return errors.Wrap(updateStatusErr, msg)
			}
			return errors.New(msg)
		}
		if configuration.Spec.JobEnv != nil {
			jobEnv, err := tfcfg.RawExtension2Map(configuration.Spec.JobEnv)
			if err != nil {
				return err
			}
			meta.JobEnv = jobEnv
		}
		if err := meta.getCredentials(ctx, k8sClient, p); err != nil {
			return err
		}
	}

	/* validate the following secret references and configmap references.
	   1. GitCredentialsSecretReference
	   2. TerraformCredentialsSecretReference
	   3. TerraformRCConfigMapReference
	   4. TerraformCredentialsHelperConfigMapReference*/
	if err := meta.validateSecretAndConfigMap(ctx, k8sClient); err != nil {
		return err
	}

	// Render configuration with backend
	completeConfiguration, backendConf, err := meta.RenderConfiguration(configuration, configurationType)
	if err != nil {
		return err
	}
	meta.CompleteConfiguration, meta.Backend = completeConfiguration, backendConf

	if configuration.ObjectMeta.DeletionTimestamp.IsZero() {
		if err := meta.storeTFConfiguration(ctx, k8sClient); err != nil {
			return err
		}
	}

	// Check whether configuration(hcl/json) is changed
	if err := meta.CheckWhetherConfigurationChanges(ctx, k8sClient, configurationType); err != nil {
		return err
	}

	if meta.ConfigurationChanged {
		klog.InfoS("Configuration hanged, reloading...")
		if err := meta.updateApplyStatus(ctx, k8sClient, types.ConfigurationReloading, types.ConfigurationReloadingAsHCLChanged); err != nil {
			return err
		}
		// store configuration to ConfigMap
		return meta.storeTFConfiguration(ctx, k8sClient)
	}

	// Check whether env changes
	if err := meta.prepareTFVariables(configuration); err != nil {
		return err
	}

	var variableInSecret v1.Secret
	err = k8sClient.Get(ctx, client.ObjectKey{Name: meta.VariableSecretName, Namespace: meta.ControllerNamespace}, &variableInSecret)
	switch {
	case kerrors.IsNotFound(err):
		var secret = v1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      meta.VariableSecretName,
				Namespace: meta.ControllerNamespace,
			},
			TypeMeta: metav1.TypeMeta{Kind: "Secret"},
			Data:     meta.VariableSecretData,
		}

		if err := k8sClient.Create(ctx, &secret); err != nil {
			return err
		}
	case err == nil:
		for k, v := range meta.VariableSecretData {
			if val, ok := variableInSecret.Data[k]; !ok || !bytes.Equal(v, val) {
				meta.EnvChanged = true
				klog.Info("Job's env changed")
				if err := meta.updateApplyStatus(ctx, k8sClient, types.ConfigurationReloading, types.ConfigurationReloadingAsVariableChanged); err != nil {
					return err
				}
				break
			}
		}
	default:
		return err
	}

	return createTerraformExecutorClusterRole(ctx, k8sClient, fmt.Sprintf("%s-%s", meta.ControllerNamespace, ClusterRoleName))
}

func (meta *TFConfigurationMeta) validateSecretAndConfigMap(ctx context.Context, k8sClient client.Client) error {

	secretConfigMapToCheck := []struct {
		ref           *v1.SecretReference
		notFoundState types.ConfigurationState
		isSecret      bool
		neededKeys    []string
		errKey        string
	}{
		{
			ref:           meta.GitCredentialsSecretReference,
			notFoundState: types.InvalidGitCredentialsSecretReference,
			isSecret:      true,
			neededKeys:    []string{GitCredsKnownHosts, v1.SSHAuthPrivateKey},
			errKey:        "git credentials",
		},
		{
			ref:           meta.TerraformCredentialsSecretReference,
			notFoundState: types.InvalidTerraformCredentialsSecretReference,
			isSecret:      true,
			neededKeys:    []string{TerraformCredentials},
			errKey:        "terraform credentials",
		},
		{
			ref:           meta.TerraformRCConfigMapReference,
			notFoundState: types.InvalidTerraformRCConfigMapReference,
			isSecret:      false,
			neededKeys:    []string{TerraformRegistryConfig},
			errKey:        "terraformrc configuration",
		},
		{
			ref:           meta.TerraformCredentialsHelperConfigMapReference,
			notFoundState: types.InvalidTerraformCredentialsHelperConfigMapReference,
			isSecret:      false,
			neededKeys:    []string{},
			errKey:        "terraform credentials helper",
		},
	}
	for _, check := range secretConfigMapToCheck {
		if check.ref != nil {
			var object metav1.Object
			var err error
			object, err = GetSecretOrConfigMap(ctx, k8sClient, check.isSecret, check.ref, check.neededKeys, check.errKey)
			if object == nil {
				msg := string(check.notFoundState)
				if err != nil {
					msg = err.Error()
				}
				if updateStatusErr := meta.updateApplyStatus(ctx, k8sClient, check.notFoundState, msg); updateStatusErr != nil {
					return errors.Wrap(updateStatusErr, msg)
				}
				return errors.New(msg)
			}
		}
	}
	return nil
}

func (meta *TFConfigurationMeta) updateApplyStatus(ctx context.Context, k8sClient client.Client, state types.ConfigurationState, message string) error {
	var configuration v1beta2.Configuration
	if err := k8sClient.Get(ctx, client.ObjectKey{Name: meta.Name, Namespace: meta.Namespace}, &configuration); err == nil {
		configuration.Status.Apply = v1beta2.ConfigurationApplyStatus{
			State:   state,
			Message: message,
			Region:  meta.Region,
		}
		configuration.Status.ObservedGeneration = configuration.Generation
		if state == types.Available {
			outputs, err := meta.getTFOutputs(ctx, k8sClient, configuration)
			if err != nil {
				klog.InfoS("Failed to get outputs", "error", err)
				configuration.Status.Apply = v1beta2.ConfigurationApplyStatus{
					State:   types.GeneratingOutputs,
					Message: types.ErrGenerateOutputs + ": " + err.Error(),
				}
			} else {
				configuration.Status.Apply.Outputs = outputs
			}
		}

		return k8sClient.Status().Update(ctx, &configuration)
	}
	return nil
}

func (meta *TFConfigurationMeta) updateDestroyStatus(ctx context.Context, k8sClient client.Client, state types.ConfigurationState, message string) error {
	var configuration v1beta2.Configuration
	if err := k8sClient.Get(ctx, client.ObjectKey{Name: meta.Name, Namespace: meta.Namespace}, &configuration); err == nil {
		configuration.Status.Destroy = v1beta2.ConfigurationDestroyStatus{
			State:   state,
			Message: message,
		}
		return k8sClient.Status().Update(ctx, &configuration)
	}
	return nil
}

func (meta *TFConfigurationMeta) assembleAndTriggerJob(ctx context.Context, k8sClient client.Client, executionType TerraformExecutionType) error {
	// apply rbac
	if err := createTerraformExecutorServiceAccount(ctx, k8sClient, meta.ControllerNamespace, ServiceAccountName); err != nil {
		return err
	}
	if err := createTerraformExecutorClusterRoleBinding(ctx, k8sClient, meta.ControllerNamespace, fmt.Sprintf("%s-%s", meta.ControllerNamespace, ClusterRoleName), ServiceAccountName); err != nil {
		return err
	}

	job := meta.assembleTerraformJob(executionType)

	return k8sClient.Create(ctx, job)
}

// updateTerraformJobIfNeeded will set deletion finalizer to the Terraform job if its envs are changed, which will result in
// deleting the job. Finally, a new Terraform job will be generated
func (meta *TFConfigurationMeta) updateTerraformJobIfNeeded(ctx context.Context, k8sClient client.Client, job batchv1.Job) error {
	// if either one changes, delete the job
	if meta.EnvChanged || meta.ConfigurationChanged {
		klog.InfoS("about to delete job", "Name", job.Name, "Namespace", job.Namespace)
		var j batchv1.Job
		if err := k8sClient.Get(ctx, client.ObjectKey{Name: job.Name, Namespace: job.Namespace}, &j); err == nil {
			if deleteErr := k8sClient.Delete(ctx, &job, client.PropagationPolicy(metav1.DeletePropagationBackground)); deleteErr != nil {
				return deleteErr
			}
		}
		var s v1.Secret
		if err := k8sClient.Get(ctx, client.ObjectKey{Name: meta.VariableSecretName, Namespace: meta.ControllerNamespace}, &s); err == nil {
			if deleteErr := k8sClient.Delete(ctx, &s); deleteErr != nil {
				return deleteErr
			}
		}
	}
	return nil
}

func (meta *TFConfigurationMeta) assembleTerraformJob(executionType TerraformExecutionType) *batchv1.Job {
	var (
		initContainer           v1.Container
		tfPreApplyInitContainer v1.Container
		initContainers          []v1.Container
		parallelism             int32 = 1
		completions             int32 = 1
		backoffLimit            int32 = math.MaxInt32
	)

	executorVolumes := meta.assembleExecutorVolumes()
	initContainerVolumeMounts := []v1.VolumeMount{
		{
			Name:      meta.Name,
			MountPath: WorkingVolumeMountPath,
		},
		{
			Name:      InputTFConfigurationVolumeName,
			MountPath: InputTFConfigurationVolumeMountPath,
		},
		{
			Name:      BackendVolumeName,
			MountPath: BackendVolumeMountPath,
		},
	}

	if meta.TerraformCredentialsSecretReference != nil {
		initContainerVolumeMounts = append(initContainerVolumeMounts,
			v1.VolumeMount{
				Name:      TerraformCredentialsConfigVolumeName,
				MountPath: TerraformCredentialsConfigVolumeMountPath,
			})
	}

	if meta.TerraformRCConfigMapReference != nil {
		initContainerVolumeMounts = append(initContainerVolumeMounts,
			v1.VolumeMount{
				Name:      TerraformRCConfigVolumeName,
				MountPath: TerraformRCConfigVolumeMountPath,
			})
	}

	if meta.TerraformCredentialsHelperConfigMapReference != nil {
		initContainerVolumeMounts = append(initContainerVolumeMounts,
			v1.VolumeMount{
				Name:      TerraformCredentialsHelperConfigVolumeName,
				MountPath: TerraformCredentialsHelperConfigVolumeMountPath,
			})
	}

	// prepare local Terraform .tf files
	initContainer = v1.Container{
		Name:            "prepare-input-terraform-configurations",
		Image:           meta.BusyboxImage,
		ImagePullPolicy: v1.PullIfNotPresent,
		Command: []string{
			"sh",
			"-c",
			fmt.Sprintf("cp %s/* %s", InputTFConfigurationVolumeMountPath, WorkingVolumeMountPath),
		},
		VolumeMounts: initContainerVolumeMounts,
	}

	initContainers = append(initContainers, initContainer)

	hclPath := filepath.Join(BackendVolumeMountPath, meta.RemoteGitPath)

	if meta.RemoteGit != "" {
		cloneCommand := fmt.Sprintf("git clone %s %s && cp -r %s/* %s", meta.RemoteGit, BackendVolumeMountPath, hclPath, WorkingVolumeMountPath)

		// Check for git credentials, mount the SSH known hosts and private key, add private key into the SSH authentication agent
		if meta.GitCredentialsSecretReference != nil {
			initContainerVolumeMounts = append(initContainerVolumeMounts,
				v1.VolumeMount{
					Name:      GitAuthConfigVolumeName,
					MountPath: GitAuthConfigVolumeMountPath,
				})

			sshCommand := fmt.Sprintf("eval `ssh-agent` && ssh-add %s/%s", GitAuthConfigVolumeMountPath, v1.SSHAuthPrivateKey)
			cloneCommand = fmt.Sprintf("%s && %s", sshCommand, cloneCommand)
		}

		command := []string{
			"sh",
			"-c",
			cloneCommand,
		}

		initContainers = append(initContainers,
			v1.Container{
				Name:            "git-configuration",
				Image:           meta.GitImage,
				ImagePullPolicy: v1.PullIfNotPresent,
				Command:         command,
				VolumeMounts:    initContainerVolumeMounts,
			})
	}

	// run `terraform init`
	tfPreApplyInitContainer = v1.Container{
		Name:            terraformInitContainerName,
		Image:           meta.TerraformImage,
		ImagePullPolicy: v1.PullIfNotPresent,
		Command: []string{
			"sh",
			"-c",
			"terraform init",
		},
		VolumeMounts: initContainerVolumeMounts,
		Env:          meta.Envs,
	}
	initContainers = append(initContainers, tfPreApplyInitContainer)

	container := v1.Container{
		Name:            terraformContainerName,
		Image:           meta.TerraformImage,
		ImagePullPolicy: v1.PullIfNotPresent,
		Command: []string{
			"bash",
			"-c",
			fmt.Sprintf("terraform %s -lock=false -auto-approve", executionType),
		},
		VolumeMounts: []v1.VolumeMount{
			{
				Name:      meta.Name,
				MountPath: WorkingVolumeMountPath,
			},
			{
				Name:      InputTFConfigurationVolumeName,
				MountPath: InputTFConfigurationVolumeMountPath,
			},
		},
		Env: meta.Envs,
	}

	if meta.ResourceQuota.ResourcesLimitsCPU != "" || meta.ResourceQuota.ResourcesLimitsMemory != "" ||
		meta.ResourceQuota.ResourcesRequestsCPU != "" || meta.ResourceQuota.ResourcesRequestsMemory != "" {
		resourceRequirements := v1.ResourceRequirements{}
		if meta.ResourceQuota.ResourcesLimitsCPU != "" || meta.ResourceQuota.ResourcesLimitsMemory != "" {
			resourceRequirements.Limits = map[v1.ResourceName]resource.Quantity{}
			if meta.ResourceQuota.ResourcesLimitsCPU != "" {
				resourceRequirements.Limits["cpu"] = meta.ResourceQuota.ResourcesLimitsCPUQuantity
			}
			if meta.ResourceQuota.ResourcesLimitsMemory != "" {
				resourceRequirements.Limits["memory"] = meta.ResourceQuota.ResourcesLimitsMemoryQuantity
			}
		}
		if meta.ResourceQuota.ResourcesRequestsCPU != "" || meta.ResourceQuota.ResourcesLimitsMemory != "" {
			resourceRequirements.Requests = map[v1.ResourceName]resource.Quantity{}
			if meta.ResourceQuota.ResourcesRequestsCPU != "" {
				resourceRequirements.Requests["cpu"] = meta.ResourceQuota.ResourcesRequestsCPUQuantity
			}
			if meta.ResourceQuota.ResourcesRequestsMemory != "" {
				resourceRequirements.Requests["memory"] = meta.ResourceQuota.ResourcesRequestsMemoryQuantity
			}
		}
		container.Resources = resourceRequirements
	}

	name := meta.ApplyJobName
	if executionType == TerraformDestroy {
		name = meta.DestroyJobName
	}

	return &batchv1.Job{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Job",
			APIVersion: "batch/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: meta.ControllerNamespace,
		},
		Spec: batchv1.JobSpec{
			Parallelism:  &parallelism,
			Completions:  &completions,
			BackoffLimit: &backoffLimit,
			Template: v1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						// This annotation will prevent istio-proxy sidecar injection in the pods
						// as having the sidecar would have kept the Job in `Running` state and would
						// not transition to `Completed`
						"sidecar.istio.io/inject": "false",
					},
				},
				Spec: v1.PodSpec{
					// InitContainer will copy Terraform configuration files to working directory and create Terraform
					// state file directory in advance
					InitContainers: initContainers,
					// Container terraform-executor will first copy predefined terraform.d to working directory, and
					// then run terraform init/apply.
					Containers:         []v1.Container{container},
					ServiceAccountName: ServiceAccountName,
					Volumes:            executorVolumes,
					RestartPolicy:      v1.RestartPolicyOnFailure,
					NodeSelector:       meta.JobNodeSelector,
				},
			},
		},
	}
}

func (meta *TFConfigurationMeta) assembleExecutorVolumes() []v1.Volume {
	workingVolume := v1.Volume{Name: meta.Name}
	workingVolume.EmptyDir = &v1.EmptyDirVolumeSource{}
	inputTFConfigurationVolume := meta.createConfigurationVolume()
	tfBackendVolume := meta.createTFBackendVolume()
	executorVolumes := []v1.Volume{workingVolume, inputTFConfigurationVolume, tfBackendVolume}
	secretOrConfigMapReferences := []struct {
		ref        *v1.SecretReference
		volumeName string
		isSecret   bool
	}{
		{
			ref:        meta.GitCredentialsSecretReference,
			volumeName: GitAuthConfigVolumeName,
			isSecret:   true,
		},
		{
			ref:        meta.TerraformCredentialsSecretReference,
			volumeName: TerraformCredentialsConfigVolumeName,
			isSecret:   true,
		},
		{
			ref:        meta.TerraformRCConfigMapReference,
			volumeName: TerraformRCConfigVolumeName,
			isSecret:   false,
		},
		{
			ref:        meta.TerraformCredentialsHelperConfigMapReference,
			volumeName: TerraformCredentialsHelperConfigVolumeName,
			isSecret:   false,
		},
	}
	for _, ref := range secretOrConfigMapReferences {
		if ref.ref != nil {
			executorVolumes = append(executorVolumes, meta.createSecretOrConfigMapVolume(ref.isSecret, ref.ref.Name, ref.volumeName))
		}
	}
	return executorVolumes
}

func (meta *TFConfigurationMeta) createConfigurationVolume() v1.Volume {
	inputCMVolumeSource := v1.ConfigMapVolumeSource{}
	inputCMVolumeSource.Name = meta.ConfigurationCMName
	inputTFConfigurationVolume := v1.Volume{Name: InputTFConfigurationVolumeName}
	inputTFConfigurationVolume.ConfigMap = &inputCMVolumeSource
	return inputTFConfigurationVolume

}

func (meta *TFConfigurationMeta) createTFBackendVolume() v1.Volume {
	gitVolume := v1.Volume{Name: BackendVolumeName}
	gitVolume.EmptyDir = &v1.EmptyDirVolumeSource{}
	return gitVolume
}

func (meta *TFConfigurationMeta) createSecretOrConfigMapVolume(isSecret bool, secretOrConfigMapReferenceName string, volumeName string) v1.Volume {
	var defaultMode int32 = 0400
	volume := v1.Volume{Name: volumeName}
	if isSecret {
		volumeSource := v1.SecretVolumeSource{}
		volumeSource.SecretName = secretOrConfigMapReferenceName
		volumeSource.DefaultMode = &defaultMode
		volume.Secret = &volumeSource
	} else {
		volumeSource := v1.ConfigMapVolumeSource{}
		volumeSource.Name = secretOrConfigMapReferenceName
		volumeSource.DefaultMode = &defaultMode
		volume.ConfigMap = &volumeSource
	}
	return volume
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

// TFState is Terraform State
type TFState struct {
	Outputs map[string]TfStateProperty `json:"outputs"`
}

func (meta *TFConfigurationMeta) isTFStateGenerated(ctx context.Context) bool {
	// 1. exist backend
	if meta.Backend == nil {
		return false
	}
	// 2. and exist tfstate file
	tfStateJSON, err := meta.Backend.GetTFStateJSON(ctx)
	if err != nil {
		return false
	}
	// 3. and outputs not empty
	var tfState TFState
	if err = json.Unmarshal(tfStateJSON, &tfState); err != nil {
		return false
	}
	return len(tfState.Outputs) > 0
}

//nolint:funlen
func (meta *TFConfigurationMeta) getTFOutputs(ctx context.Context, k8sClient client.Client, configuration v1beta2.Configuration) (map[string]v1beta2.Property, error) {
	var tfStateJSON []byte
	var err error
	if meta.Backend != nil {
		tfStateJSON, err = meta.Backend.GetTFStateJSON(ctx)
		if err != nil {
			return nil, err
		}
	}

	var tfState TFState
	if err := json.Unmarshal(tfStateJSON, &tfState); err != nil {
		return nil, err
	}
	outputs := make(map[string]v1beta2.Property)
	for k, v := range tfState.Outputs {
		property, err := v.ToProperty()
		if err != nil {
			return outputs, err
		}
		outputs[k] = property
	}
	writeConnectionSecretToReference := configuration.Spec.WriteConnectionSecretToReference
	if writeConnectionSecretToReference == nil || writeConnectionSecretToReference.Name == "" {
		return outputs, nil
	}

	name := writeConnectionSecretToReference.Name
	ns := writeConnectionSecretToReference.Namespace
	if ns == "" {
		ns = defaultNamespace
	}
	data := make(map[string][]byte)
	for k, v := range outputs {
		data[k] = []byte(v.Value)
	}
	var gotSecret v1.Secret
	configurationName := configuration.ObjectMeta.Name
	if err := k8sClient.Get(ctx, client.ObjectKey{Name: name, Namespace: ns}, &gotSecret); err != nil {
		if kerrors.IsNotFound(err) {
			var secret = v1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: ns,
					Labels: map[string]string{
						"terraform.core.oam.dev/created-by":      "terraform-controller",
						"terraform.core.oam.dev/owned-by":        configurationName,
						"terraform.core.oam.dev/owned-namespace": configuration.Namespace,
					},
				},
				TypeMeta: metav1.TypeMeta{Kind: "Secret"},
				Data:     data,
			}
			err = k8sClient.Create(ctx, &secret)
			if kerrors.IsAlreadyExists(err) {
				return nil, fmt.Errorf("secret(%s) already exists", name)
			} else if err != nil {
				return nil, err
			}
		}
	} else {
		// check the owner of this secret
		labels := gotSecret.ObjectMeta.Labels
		ownerName := labels["terraform.core.oam.dev/owned-by"]
		ownerNamespace := labels["terraform.core.oam.dev/owned-namespace"]
		if (ownerName != "" && ownerName != configurationName) ||
			(ownerNamespace != "" && ownerNamespace != configuration.Namespace) {
			errMsg := fmt.Sprintf(
				"configuration(namespace: %s ; name: %s) cannot update secret(namespace: %s ; name: %s) whose owner is configuration(namespace: %s ; name: %s)",
				configuration.Namespace, configurationName,
				gotSecret.Namespace, name,
				ownerNamespace, ownerName,
			)
			klog.ErrorS(err, "fail to update backend secret")
			return nil, errors.New(errMsg)
		}
		gotSecret.Data = data
		if err := k8sClient.Update(ctx, &gotSecret); err != nil {
			return nil, err
		}
	}
	return outputs, nil
}

func (meta *TFConfigurationMeta) prepareTFVariables(configuration *v1beta2.Configuration) error {
	var (
		envs []v1.EnvVar
		data = map[string][]byte{}
	)

	if configuration == nil {
		return errors.New("configuration is nil")
	}
	if !configuration.Spec.InlineCredentials && meta.ProviderReference == nil {
		return errors.New("The referenced provider could not be retrieved")
	}

	tfVariable, err := getTerraformJSONVariable(configuration.Spec.Variable)
	if err != nil {
		return errors.Wrap(err, fmt.Sprintf("failed to get Terraform JSON variables from Configuration Variables %v", configuration.Spec.Variable))
	}
	for k, v := range tfVariable {
		envValue, err := tfcfg.Interface2String(v)
		if err != nil {
			return err
		}
		data[k] = []byte(envValue)
	}

	if !configuration.Spec.InlineCredentials && meta.Credentials == nil {
		return errors.New(provider.ErrCredentialNotRetrieved)
	}
	for k, v := range meta.Credentials {
		data[k] = []byte(v)
	}
	for k, v := range meta.JobEnv {
		envValue, err := tfcfg.Interface2String(v)
		if err != nil {
			return err
		}
		data[k] = []byte(envValue)
	}
	for k := range data {
		valueFrom := &v1.EnvVarSource{SecretKeyRef: &v1.SecretKeySelector{Key: k}}
		valueFrom.SecretKeyRef.Name = meta.VariableSecretName
		envs = append(envs, v1.EnvVar{Name: k, ValueFrom: valueFrom})
	}
	meta.Envs = envs
	meta.VariableSecretData = data

	return nil
}

// SetupWithManager setups with a manager
func (r *ConfigurationReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1beta2.Configuration{}).
		Complete(r)
}

func getTerraformJSONVariable(tfVariables *runtime.RawExtension) (map[string]interface{}, error) {
	variables, err := tfcfg.RawExtension2Map(tfVariables)
	if err != nil {
		return nil, err
	}
	var environments = make(map[string]interface{})

	for k, v := range variables {
		environments[fmt.Sprintf("TF_VAR_%s", k)] = v
	}
	return environments, nil
}

func deleteConfigMap(ctx context.Context, meta *TFConfigurationMeta, k8sClient client.Client) error {
	var cm v1.ConfigMap
	// We have four cases when upgrading. There are three combinations of name and namespace.
	// TODO compatible for case 4
	// 1. no "controller-namespace" -> specify "controller-namespace"
	// 2. no "controller-namespace" -> no "controller-namespace"
	// 3. specify "controller-namespace" -> specify "controller-namespace"
	// 4. specify "controller-namespace" -> no "controller-namespace" (NOT SUPPORTED)
	possibleCombination := [][2]string{
		{meta.LegacySubResources.ConfigurationCMName, meta.LegacySubResources.Namespace},
		{meta.ConfigurationCMName, meta.ControllerNamespace},
		{meta.ConfigurationCMName, meta.Namespace},
	}
	klog.InfoS("Deleting the ConfigMap which stores configuration", "Name", meta.ConfigurationCMName)
	for _, combination := range possibleCombination {
		if err := k8sClient.Get(ctx, client.ObjectKey{Name: combination[0], Namespace: combination[1]}, &cm); err == nil {
			if err := k8sClient.Delete(ctx, &cm); err != nil {
				return client.IgnoreNotFound(err)
			}
		}
	}
	return nil
}

func deleteVariableSecret(ctx context.Context, meta *TFConfigurationMeta, k8sClient client.Client) error {
	var variableSecret v1.Secret
	// see TFConfigurationMeta.deleteConfigMap
	possibleCombination := [][2]string{
		{meta.LegacySubResources.VariableSecretName, meta.LegacySubResources.Namespace},
		{meta.VariableSecretName, meta.ControllerNamespace},
		{meta.VariableSecretName, meta.Namespace},
	}
	klog.InfoS("Deleting the secret which stores variables", "Name", meta.VariableSecretName)
	for _, combination := range possibleCombination {
		if err := k8sClient.Get(ctx, client.ObjectKey{Name: combination[0], Namespace: combination[1]}, &variableSecret); err == nil {
			if err := k8sClient.Delete(ctx, &variableSecret); err != nil {
				return client.IgnoreNotFound(err)
			}
		}
	}
	return nil
}

func deleteApplyJob(ctx context.Context, meta *TFConfigurationMeta, k8sClient client.Client) error {
	var job batchv1.Job
	// see TFConfigurationMeta.deleteConfigMap
	possibleCombination := [][2]string{
		{meta.LegacySubResources.ApplyJobName, meta.LegacySubResources.Namespace},
		{meta.ApplyJobName, meta.ControllerNamespace},
		{meta.ApplyJobName, meta.Namespace},
	}
	klog.InfoS("Deleting the apply job", "Name", meta.ApplyJobName)
	for _, combination := range possibleCombination {
		if err := k8sClient.Get(ctx, client.ObjectKey{Name: combination[0], Namespace: combination[1]}, &job); err == nil {
			if err := k8sClient.Delete(ctx, &job, client.PropagationPolicy(metav1.DeletePropagationBackground)); err != nil {
				return client.IgnoreNotFound(err)
			}
		}
	}
	return nil
}

func deleteDestroyJob(ctx context.Context, meta *TFConfigurationMeta, k8sClient client.Client) error {
	var job batchv1.Job
	// see TFConfigurationMeta.deleteConfigMap
	possibleCombination := [][2]string{
		{meta.LegacySubResources.DestroyJobName, meta.LegacySubResources.Namespace},
		{meta.DestroyJobName, meta.ControllerNamespace},
		{meta.DestroyJobName, meta.Namespace},
	}
	klog.InfoS("Deleting the destroy job", "Name", meta.DestroyJobName)
	for _, combination := range possibleCombination {
		if err := k8sClient.Get(ctx, client.ObjectKey{Name: combination[0], Namespace: combination[1]}, &job); err == nil {
			if err := k8sClient.Delete(ctx, &job, client.PropagationPolicy(metav1.DeletePropagationBackground)); err != nil {
				return client.IgnoreNotFound(err)
			}
		}
	}
	return nil
}

func deleteConnectionSecret(ctx context.Context, k8sClient client.Client, name, ns string) error {
	if len(name) == 0 {
		return nil
	}

	var connectionSecret v1.Secret
	if len(ns) == 0 {
		ns = defaultNamespace
	}
	if err := k8sClient.Get(ctx, client.ObjectKey{Name: name, Namespace: ns}, &connectionSecret); err == nil {
		return k8sClient.Delete(ctx, &connectionSecret)
	}
	return nil
}

func (meta *TFConfigurationMeta) createOrUpdateConfigMap(ctx context.Context, k8sClient client.Client, data map[string]string) error {
	var gotCM v1.ConfigMap
	if err := k8sClient.Get(ctx, client.ObjectKey{Name: meta.ConfigurationCMName, Namespace: meta.ControllerNamespace}, &gotCM); err != nil {
		if !kerrors.IsNotFound(err) {
			return err
		}
		cm := v1.ConfigMap{
			TypeMeta: metav1.TypeMeta{APIVersion: "v1", Kind: "ConfigMap"},
			ObjectMeta: metav1.ObjectMeta{
				Name:      meta.ConfigurationCMName,
				Namespace: meta.ControllerNamespace,
			},
			Data: data,
		}

		if err := k8sClient.Create(ctx, &cm); err != nil {
			return errors.Wrap(err, "failed to create TF configuration ConfigMap")
		}

		return nil
	}

	if !reflect.DeepEqual(gotCM.Data, data) {
		gotCM.Data = data

		return errors.Wrap(k8sClient.Update(ctx, &gotCM), "failed to update TF configuration ConfigMap")
	}

	return nil
}

func (meta *TFConfigurationMeta) prepareTFInputConfigurationData() map[string]string {
	var dataName string
	switch meta.ConfigurationType {
	case types.ConfigurationHCL:
		dataName = types.TerraformHCLConfigurationName
	case types.ConfigurationRemote:
		dataName = "terraform-backend.tf"
	}
	data := map[string]string{dataName: meta.CompleteConfiguration, "kubeconfig": ""}
	return data
}

// storeTFConfiguration will store Terraform configuration to ConfigMap
func (meta *TFConfigurationMeta) storeTFConfiguration(ctx context.Context, k8sClient client.Client) error {
	data := meta.prepareTFInputConfigurationData()
	return meta.createOrUpdateConfigMap(ctx, k8sClient, data)
}

// CheckWhetherConfigurationChanges will check whether configuration is changed
func (meta *TFConfigurationMeta) CheckWhetherConfigurationChanges(ctx context.Context, k8sClient client.Client, configurationType types.ConfigurationType) error {
	var cm v1.ConfigMap
	if err := k8sClient.Get(ctx, client.ObjectKey{Name: meta.ConfigurationCMName, Namespace: meta.ControllerNamespace}, &cm); err != nil {
		return err
	}

	var configurationChanged bool
	switch configurationType {
	case types.ConfigurationHCL:
		configurationChanged = cm.Data[types.TerraformHCLConfigurationName] != meta.CompleteConfiguration
		meta.ConfigurationChanged = configurationChanged
		if configurationChanged {
			klog.InfoS("Configuration HCL changed", "ConfigMap", cm.Data[types.TerraformHCLConfigurationName],
				"RenderedCompletedConfiguration", meta.CompleteConfiguration)
		}

		return nil
	case types.ConfigurationRemote:
		meta.ConfigurationChanged = false
		return nil
	default:
		return errors.New("unsupported configuration type, only HCL or Remote is supported")
	}
}

// getCredentials will get credentials from secret of the Provider
func (meta *TFConfigurationMeta) getCredentials(ctx context.Context, k8sClient client.Client, providerObj *v1beta1.Provider) error {
	region, err := tfcfg.SetRegion(ctx, k8sClient, meta.Namespace, meta.Name, providerObj)
	if err != nil {
		return err
	}
	credentials, err := provider.GetProviderCredentials(ctx, k8sClient, providerObj, region)
	if err != nil {
		return err
	}
	if credentials == nil {
		return errors.New(provider.ErrCredentialNotRetrieved)
	}
	meta.Credentials = credentials
	meta.Region = region
	return nil
}

func (meta *TFConfigurationMeta) KeepLegacySubResourceMetas() {
	meta.LegacySubResources.Namespace = meta.Namespace
	meta.LegacySubResources.ApplyJobName = meta.ApplyJobName
	meta.LegacySubResources.DestroyJobName = meta.DestroyJobName
	meta.LegacySubResources.ConfigurationCMName = meta.ConfigurationCMName
	meta.LegacySubResources.VariableSecretName = meta.VariableSecretName
}

func (meta *TFConfigurationMeta) getApplyJob(ctx context.Context, k8sClient client.Client, job *batchv1.Job) error {
	if err := k8sClient.Get(ctx, client.ObjectKey{Name: meta.LegacySubResources.ApplyJobName, Namespace: meta.LegacySubResources.Namespace}, job); err == nil {
		klog.InfoS("Found legacy apply job", "Configuration", fmt.Sprintf("%s/%s", meta.Name, meta.Namespace),
			"Job", fmt.Sprintf("%s/%s", meta.LegacySubResources.Namespace, meta.LegacySubResources.ApplyJobName))
		return nil
	}
	err := k8sClient.Get(ctx, client.ObjectKey{Name: meta.ApplyJobName, Namespace: meta.ControllerNamespace}, job)
	return err
}

// RenderConfiguration will compose the Terraform configuration with hcl/json and backend
func (meta *TFConfigurationMeta) RenderConfiguration(configuration *v1beta2.Configuration, configurationType types.ConfigurationType) (string, backend.Backend, error) {
	backendInterface, err := backend.ParseConfigurationBackend(configuration, meta.K8sClient, meta.Credentials, meta.ControllerNSSpecified)
	if err != nil {
		return "", nil, errors.Wrap(err, "failed to prepare Terraform backend configuration")
	}

	switch configurationType {
	case types.ConfigurationHCL:
		completedConfiguration := configuration.Spec.HCL
		completedConfiguration += "\n" + backendInterface.HCL()
		return completedConfiguration, backendInterface, nil
	case types.ConfigurationRemote:
		return backendInterface.HCL(), backendInterface, nil
	default:
		return "", nil, errors.New("Unsupported Configuration Type")
	}
}

func GetSecretOrConfigMap(ctx context.Context, k8sClient client.Client, isSecret bool, ref *v1.SecretReference, neededKeys []string, errKey string) (metav1.Object, error) {
	secret := &v1.Secret{}
	configMap := &v1.ConfigMap{}
	var err error
	// key to determine if it is a secret or config map
	var typeKey string
	if isSecret {
		namespacedName := client.ObjectKey{Name: ref.Name, Namespace: ref.Namespace}
		err = k8sClient.Get(ctx, namespacedName, secret)
		typeKey = "secret"
	} else {
		namespacedName := client.ObjectKey{Name: ref.Name, Namespace: ref.Namespace}
		err = k8sClient.Get(ctx, namespacedName, configMap)
		typeKey = "configmap"
	}
	errMsg := fmt.Sprintf("Failed to get %s %s", errKey, typeKey)
	if err != nil {
		klog.ErrorS(err, errMsg, "Name", ref.Name, "Namespace", ref.Namespace)
		return nil, errors.Wrap(err, errMsg)
	}
	for _, key := range neededKeys {
		var keyErr bool
		if isSecret {
			if _, ok := secret.Data[key]; !ok {
				keyErr = true
			}
		} else {
			if _, ok := configMap.Data[key]; !ok {
				keyErr = true
			}
		}
		if keyErr {
			keyErr := errors.Errorf("'%s' not in %s %s", key, errKey, typeKey)
			return nil, keyErr
		}
	}
	if isSecret {
		return secret, nil
	}
	return configMap, nil
}
