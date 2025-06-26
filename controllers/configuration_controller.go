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

// Package controllers contains the Terraform configuration and provider controllers.
package controllers

import (
	"bytes"
	"context"
	"fmt"
	"math"
	"os"
	"strconv"
	"time"

	"github.com/oam-dev/terraform-controller/controllers/process"
	"github.com/oam-dev/terraform-controller/controllers/util"

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
	"github.com/oam-dev/terraform-controller/api/v1beta2"
	tfcfg "github.com/oam-dev/terraform-controller/controllers/configuration"
	"github.com/oam-dev/terraform-controller/controllers/features"
	"github.com/oam-dev/terraform-controller/controllers/provider"
	"github.com/oam-dev/terraform-controller/controllers/terraform"
)

const (
	defaultNamespace       = "default"
	configurationFinalizer = "configuration.finalizers.terraform-controller"
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

	meta := process.New(req, configuration, r.Client, process.ControllerNamespaceOption(r.ControllerNamespace))

	// add finalizer
	var isDeleting = !configuration.DeletionTimestamp.IsZero()
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

		_, err := terraform.GetTerraformStatus(ctx, meta.ControllerNamespace, meta.DestroyJobName, types.TerraformContainerName, types.TerraformInitContainerName)
		if err != nil {
			klog.ErrorS(err, "Terraform destroy failed")
			if updateErr := meta.UpdateDestroyStatus(ctx, r.Client, types.ConfigurationDestroyFailed, err.Error()); updateErr != nil {
				return ctrl.Result{}, updateErr
			}
		}

		// If no tfState has been generated, then perform a quick cleanup without dispatching destroying job.
		if meta.IsTFStateGenerated(ctx) {
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
	if err := meta.GetApplyJob(ctx, r.Client, tfExecutionJob); err == nil {
		if !meta.EnvChanged && !meta.ConfigurationChanged && tfExecutionJob.Status.Succeeded == int32(1) {
			err = meta.UpdateApplyStatus(ctx, r.Client, types.Available, types.MessageCloudResourceDeployed)
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
	state, err := terraform.GetTerraformStatus(ctx, meta.ControllerNamespace, meta.ApplyJobName, types.TerraformContainerName, types.TerraformInitContainerName)
	if err != nil {
		klog.ErrorS(err, "Terraform apply failed")
		if updateErr := meta.UpdateApplyStatus(ctx, r.Client, state, err.Error()); updateErr != nil {
			return ctrl.Result{}, updateErr
		}

		return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
	}

	return ctrl.Result{}, nil
}

func (r *ConfigurationReconciler) terraformApply(ctx context.Context, configuration v1beta2.Configuration, meta *process.TFConfigurationMeta) error {

	var (
		k8sClient      = r.Client
		tfExecutionJob batchv1.Job
	)

	if err := meta.GetApplyJob(ctx, k8sClient, &tfExecutionJob); err != nil {
		if kerrors.IsNotFound(err) {
			return meta.AssembleAndTriggerJob(ctx, k8sClient, types.TerraformApply)
		}
	}
	klog.InfoS("terraform apply job", "Namespace", tfExecutionJob.Namespace, "Name", tfExecutionJob.Name)

	if err := meta.UpdateTerraformJobIfNeeded(ctx, k8sClient, tfExecutionJob); err != nil {
		klog.ErrorS(err, types.ErrUpdateTerraformApplyJob, "Name", meta.ApplyJobName)
		return errors.Wrap(err, types.ErrUpdateTerraformApplyJob)
	}

	if !meta.EnvChanged && tfExecutionJob.Status.Succeeded == int32(1) {
		if err := meta.UpdateApplyStatus(ctx, k8sClient, types.Available, types.MessageCloudResourceDeployed); err != nil {
			return err
		}
	} else {
		// start provisioning and check the status of the provision
		// If the state is types.InvalidRegion, no need to continue checking
		if configuration.Status.Apply.State != types.ConfigurationProvisioningAndChecking &&
			configuration.Status.Apply.State != types.InvalidRegion {
			if err := meta.UpdateApplyStatus(ctx, r.Client, types.ConfigurationProvisioningAndChecking, types.MessageCloudResourceProvisioningAndChecking); err != nil {
				return err
			}
		}
	}
	return nil
}

func (r *ConfigurationReconciler) terraformDestroy(ctx context.Context, configuration v1beta2.Configuration, meta *process.TFConfigurationMeta) error {
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
				if err := r.Get(ctx, client.ObjectKey{Name: configuration.Name, Namespace: configuration.Namespace}, &v1beta2.Configuration{}); err == nil {
					if err = meta.AssembleAndTriggerJob(ctx, k8sClient, types.TerraformDestroy); err != nil {
						return err
					}
				}
			}
		}
		if err := meta.UpdateTerraformJobIfNeeded(ctx, k8sClient, destroyJob); err != nil {
			klog.ErrorS(err, types.ErrUpdateTerraformApplyJob, "Name", meta.DestroyJobName)
			return errors.Wrap(err, types.ErrUpdateTerraformApplyJob)
		}
	}

	// destroying
	if err := meta.UpdateDestroyStatus(ctx, k8sClient, types.ConfigurationDestroying, types.MessageCloudResourceDestroying); err != nil {
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

func (r *ConfigurationReconciler) cleanUpSubResources(ctx context.Context, configuration v1beta2.Configuration, meta *process.TFConfigurationMeta) error {
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
	type cleanupResourceFunc func(ctx context.Context, meta *process.TFConfigurationMeta, k8sClient client.Client) error
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
	if meta.Backend != nil && meta.DeleteResource {
		if err := meta.Backend.CleanUp(ctx); err != nil {
			return err
		}
	}

	return nil
}

func (r *ConfigurationReconciler) preCheckResourcesSetting(meta *process.TFConfigurationMeta) error {

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

func (r *ConfigurationReconciler) preCheck(ctx context.Context, configuration *v1beta2.Configuration, meta *process.TFConfigurationMeta) error {
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

	meta.BackoffLimit = math.MaxInt32
	if backoffLimit := os.Getenv("JOB_BACKOFF_LIMIT"); backoffLimit != "" {
		if backoffLimitNumber, parserErr := strconv.ParseInt(backoffLimit, 10, 32); parserErr == nil {
			meta.BackoffLimit = int32(backoffLimitNumber)
		}
	}

	if err := r.preCheckResourcesSetting(meta); err != nil {
		return err
	}

	// Validation: 1) validate Configuration itself
	configurationType, err := tfcfg.ValidConfigurationObject(configuration)
	if err != nil {
		if updateErr := meta.UpdateApplyStatus(ctx, k8sClient, types.ConfigurationStaticCheckFailed, err.Error()); updateErr != nil {
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
			if updateStatusErr := meta.UpdateApplyStatus(ctx, k8sClient, types.Authorizing, msg); updateStatusErr != nil {
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
		if err := meta.GetCredentials(ctx, k8sClient, p); err != nil {
			return err
		}
	}

	/* validate the following secret references and configmap references.
	   1. GitCredentialsSecretReference
	   2. TerraformCredentialsSecretReference
	   3. TerraformRCConfigMapReference
	   4. TerraformCredentialsHelperConfigMapReference*/
	if err := meta.ValidateSecretAndConfigMap(ctx, k8sClient); err != nil {
		return err
	}

	// Render configuration with backend
	completeConfiguration, backendConf, err := meta.RenderConfiguration(configuration, configurationType)
	if err != nil {
		return err
	}
	meta.CompleteConfiguration, meta.Backend = completeConfiguration, backendConf

	// Check whether configuration(hcl/json) is changed
	if err := meta.CheckWhetherConfigurationChanges(ctx, k8sClient, configurationType); err != nil {
		return err
	}

	if configuration.DeletionTimestamp.IsZero() {
		if err := meta.StoreTFConfiguration(ctx, k8sClient); err != nil {
			return err
		}
	}

	if meta.ConfigurationChanged {
		klog.InfoS("Configuration hanged, reloading...")
		return meta.UpdateApplyStatus(ctx, k8sClient, types.ConfigurationReloading, types.ConfigurationReloadingAsHCLChanged)
	}

	// Check whether env changes
	if err := meta.PrepareTFVariables(configuration); err != nil {
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
				if err := meta.UpdateApplyStatus(ctx, k8sClient, types.ConfigurationReloading, types.ConfigurationReloadingAsVariableChanged); err != nil {
					return err
				}
				break
			}
		}
	default:
		return err
	}

	return util.CreateTerraformExecutorClusterRole(ctx, k8sClient, fmt.Sprintf("%s-%s", meta.ControllerNamespace, types.ClusterRoleName))
}

// SetupWithManager setups with a manager
func (r *ConfigurationReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1beta2.Configuration{}).
		Complete(r)
}

func deleteConfigMap(ctx context.Context, meta *process.TFConfigurationMeta, k8sClient client.Client) error {
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

func deleteVariableSecret(ctx context.Context, meta *process.TFConfigurationMeta, k8sClient client.Client) error {
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

func deleteApplyJob(ctx context.Context, meta *process.TFConfigurationMeta, k8sClient client.Client) error {
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

func deleteDestroyJob(ctx context.Context, meta *process.TFConfigurationMeta, k8sClient client.Client) error {
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
