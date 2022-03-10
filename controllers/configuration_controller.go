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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/oam-dev/terraform-controller/api/types"
	crossplane "github.com/oam-dev/terraform-controller/api/types/crossplane-runtime"
	"github.com/oam-dev/terraform-controller/api/v1beta1"
	"github.com/oam-dev/terraform-controller/api/v1beta2"
	tfcfg "github.com/oam-dev/terraform-controller/controllers/configuration"
	"github.com/oam-dev/terraform-controller/controllers/provider"
	"github.com/oam-dev/terraform-controller/controllers/terraform"
	"github.com/oam-dev/terraform-controller/controllers/util"
)

const (
	terraformWorkspace = "default"
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
	terraformContainerName = "terraform-executor"
)

const (
	// TerraformStateNameInSecret is the key name to store Terraform state
	TerraformStateNameInSecret = "tfstate"
	// TFInputConfigMapName is the CM name for Terraform Input Configuration
	TFInputConfigMapName = "tf-%s"
	// TFVariableSecret is the Secret name for variables, including credentials from Provider
	TFVariableSecret = "variable-%s"
	// TFBackendSecret is the Secret name for Kubernetes backend
	TFBackendSecret = "tfstate-%s-%s"
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

// ConfigurationReconciler reconciles a Configuration object.
type ConfigurationReconciler struct {
	client.Client
	Log          logr.Logger
	Scheme       *runtime.Scheme
	ProviderName string
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

	meta := initTFConfigurationMeta(req, configuration)

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

	var tfExecutionJob = &batchv1.Job{}
	if err := r.Client.Get(ctx, client.ObjectKey{Name: meta.ApplyJobName, Namespace: meta.Namespace}, tfExecutionJob); err == nil {
		if !meta.EnvChanged && tfExecutionJob.Status.Succeeded == int32(1) {
			if err := meta.updateApplyStatus(ctx, r.Client, types.Available, types.MessageCloudResourceDeployed); err != nil {
				return ctrl.Result{}, err
			}
		}
	}

	if isDeleting {
		// terraform destroy
		klog.InfoS("performing Configuration Destroy", "Namespace", req.Namespace, "Name", req.Name, "JobName", meta.DestroyJobName)

		_, err := terraform.GetTerraformStatus(ctx, meta.Namespace, meta.DestroyJobName, terraformContainerName)
		if err != nil {
			klog.ErrorS(err, "Terraform destroy failed")
			if updateErr := meta.updateDestroyStatus(ctx, r.Client, types.ConfigurationDestroyFailed, err.Error()); updateErr != nil {
				return ctrl.Result{}, updateErr
			}
		}

		if err := r.terraformDestroy(ctx, req.Namespace, configuration, meta); err != nil {
			if err.Error() == types.MessageDestroyJobNotCompleted {
				return ctrl.Result{RequeueAfter: 3 * time.Second}, nil
			}
			return ctrl.Result{RequeueAfter: 3 * time.Second}, errors.Wrap(err, "continue reconciling to destroy cloud resource")
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

	// Terraform apply (create or update)
	klog.InfoS("performing Terraform Apply (cloud resource create/update)", "Namespace", req.Namespace, "Name", req.Name)
	if err := r.terraformApply(ctx, req.Namespace, configuration, meta); err != nil {
		if err.Error() == types.MessageApplyJobNotCompleted {
			return ctrl.Result{RequeueAfter: 3 * time.Second}, nil
		}
		return ctrl.Result{RequeueAfter: 3 * time.Second}, errors.Wrap(err, "failed to create/update cloud resource")
	}
	state, err := terraform.GetTerraformStatus(ctx, meta.Namespace, meta.ApplyJobName, terraformContainerName)
	if err != nil {
		klog.ErrorS(err, "Terraform apply failed")
		if updateErr := meta.updateApplyStatus(ctx, r.Client, state, err.Error()); updateErr != nil {
			return ctrl.Result{}, updateErr
		}
	}

	return ctrl.Result{}, nil
}

// TFConfigurationMeta is all the metadata of a Configuration
type TFConfigurationMeta struct {
	Name                  string
	Namespace             string
	ConfigurationType     types.ConfigurationType
	CompleteConfiguration string
	RemoteGit             string
	RemoteGitPath         string
	ConfigurationChanged  bool
	EnvChanged            bool
	ConfigurationCMName   string
	BackendSecretName     string
	ApplyJobName          string
	DestroyJobName        string
	Envs                  []v1.EnvVar
	ProviderReference     *crossplane.Reference
	VariableSecretName    string
	VariableSecretData    map[string][]byte
	DeleteResource        bool
	Credentials           map[string]string

	// TerraformImage is the Terraform image which can run `terraform init/plan/apply`
	TerraformImage            string
	TerraformBackendNamespace string
	BusyboxImage              string
	GitImage                  string
}

func initTFConfigurationMeta(req ctrl.Request, configuration v1beta2.Configuration) *TFConfigurationMeta {
	var meta = &TFConfigurationMeta{
		Namespace:           req.Namespace,
		Name:                req.Name,
		ConfigurationCMName: fmt.Sprintf(TFInputConfigMapName, req.Name),
		VariableSecretName:  fmt.Sprintf(TFVariableSecret, req.Name),
		ApplyJobName:        req.Name + "-" + string(TerraformApply),
		DestroyJobName:      req.Name + "-" + string(TerraformDestroy),
	}

	// githubBlocked mark whether GitHub is blocked in the cluster
	githubBlockedStr := os.Getenv("GITHUB_BLOCKED")
	if githubBlockedStr == "" {
		githubBlockedStr = "false"
	}

	meta.RemoteGit = tfcfg.ReplaceTerraformSource(configuration.Spec.Remote, githubBlockedStr)
	meta.DeleteResource = configuration.Spec.DeleteResource
	if configuration.Spec.Path == "" {
		meta.RemoteGitPath = "."
	} else {
		meta.RemoteGitPath = configuration.Spec.Path
	}

	meta.ProviderReference = tfcfg.GetProviderNamespacedName(configuration)

	// Check the existence of Terraform state secret which is used to store TF state file. For detailed information,
	// please refer to https://www.terraform.io/docs/language/settings/backends/kubernetes.html#configuration-variables
	var backendSecretSuffix string
	if configuration.Spec.Backend != nil && configuration.Spec.Backend.SecretSuffix != "" {
		backendSecretSuffix = configuration.Spec.Backend.SecretSuffix
	} else {
		backendSecretSuffix = configuration.Name
	}
	// Secrets will be named in the format: tfstate-{workspace}-{secret_suffix}
	meta.BackendSecretName = fmt.Sprintf(TFBackendSecret, terraformWorkspace, backendSecretSuffix)

	return meta
}

func (r *ConfigurationReconciler) terraformApply(ctx context.Context, namespace string, configuration v1beta2.Configuration, meta *TFConfigurationMeta) error {
	klog.InfoS("terraform apply job", "Namespace", namespace, "Name", meta.ApplyJobName)

	var (
		k8sClient      = r.Client
		tfExecutionJob batchv1.Job
	)

	if err := k8sClient.Get(ctx, client.ObjectKey{Name: meta.ApplyJobName, Namespace: namespace}, &tfExecutionJob); err != nil {
		if kerrors.IsNotFound(err) {
			return meta.assembleAndTriggerJob(ctx, k8sClient, TerraformApply)
		}
	}

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

func (r *ConfigurationReconciler) terraformDestroy(ctx context.Context, namespace string, configuration v1beta2.Configuration, meta *TFConfigurationMeta) error {
	var (
		destroyJob batchv1.Job
		k8sClient  = r.Client
	)

	deletable, err := tfcfg.IsDeletable(ctx, k8sClient, &configuration)
	if err != nil {
		return err
	}

	deleteConfigurationDirectly := deletable || !meta.DeleteResource

	if !deleteConfigurationDirectly {
		if err := k8sClient.Get(ctx, client.ObjectKey{Name: meta.DestroyJobName, Namespace: meta.Namespace}, &destroyJob); err != nil {
			if kerrors.IsNotFound(err) {
				if err := r.Client.Get(ctx, client.ObjectKey{Name: configuration.Name, Namespace: configuration.Namespace}, &v1beta2.Configuration{}); err == nil {
					if err = meta.assembleAndTriggerJob(ctx, k8sClient, TerraformDestroy); err != nil {
						return err
					}
				}
			}
		}
		if err := meta.updateTerraformJobIfNeeded(ctx, k8sClient, destroyJob); err != nil {
			klog.ErrorS(err, types.ErrUpdateTerraformApplyJob, "Name", meta.ApplyJobName)
			return errors.Wrap(err, types.ErrUpdateTerraformApplyJob)
		}
	}

	// destroying
	if err := meta.updateDestroyStatus(ctx, k8sClient, types.ConfigurationDestroying, types.MessageCloudResourceDestroying); err != nil {
		return err
	}

	// When the deletion Job process succeeded, clean up work is starting.
	if err := k8sClient.Get(ctx, client.ObjectKey{Name: meta.DestroyJobName, Namespace: meta.Namespace}, &destroyJob); err != nil {
		return err
	}
	if destroyJob.Status.Succeeded == int32(1) || deleteConfigurationDirectly {
		// 1. delete Terraform input Configuration ConfigMap
		if err := meta.deleteConfigMap(ctx, k8sClient); err != nil {
			return err
		}

		// 2. delete connectionSecret
		if configuration.Spec.WriteConnectionSecretToReference != nil {
			secretName := configuration.Spec.WriteConnectionSecretToReference.Name
			secretNameSpace := configuration.Spec.WriteConnectionSecretToReference.Namespace
			if err := deleteConnectionSecret(ctx, k8sClient, secretName, secretNameSpace); err != nil {
				return err
			}
		}

		// 3. delete apply job
		var applyJob batchv1.Job
		if err := k8sClient.Get(ctx, client.ObjectKey{Name: meta.ApplyJobName, Namespace: namespace}, &applyJob); err == nil {
			if err := k8sClient.Delete(ctx, &applyJob, client.PropagationPolicy(metav1.DeletePropagationBackground)); err != nil {
				return err
			}
		}

		// 4. delete destroy job
		var j batchv1.Job
		if err := r.Client.Get(ctx, client.ObjectKey{Name: destroyJob.Name, Namespace: destroyJob.Namespace}, &j); err == nil {
			if err := r.Client.Delete(ctx, &j, client.PropagationPolicy(metav1.DeletePropagationBackground)); err != nil {
				return err
			}
		}

		// 5. delete secret which stores variables
		klog.InfoS("Deleting the secret which stores variables", "Name", meta.VariableSecretName)
		var variableSecret v1.Secret
		if err := r.Client.Get(ctx, client.ObjectKey{Name: meta.VariableSecretName, Namespace: meta.Namespace}, &variableSecret); err == nil {
			if err := r.Client.Delete(ctx, &variableSecret); err != nil {
				return err
			}
		}

		// 6. delete Kubernetes backend secret
		klog.InfoS("Deleting the secret which stores Kubernetes backend", "Name", meta.BackendSecretName)
		var kubernetesBackendSecret v1.Secret
		if err := r.Client.Get(ctx, client.ObjectKey{Name: meta.BackendSecretName, Namespace: meta.TerraformBackendNamespace}, &kubernetesBackendSecret); err == nil {
			if err := r.Client.Delete(ctx, &kubernetesBackendSecret); err != nil {
				return err
			}
		}
		return nil
	}
	return errors.New(types.MessageDestroyJobNotCompleted)
}

func (r *ConfigurationReconciler) preCheck(ctx context.Context, configuration *v1beta2.Configuration, meta *TFConfigurationMeta) error {
	var k8sClient = r.Client

	meta.TerraformImage = os.Getenv("TERRAFORM_IMAGE")
	if meta.TerraformImage == "" {
		meta.TerraformImage = "oamdev/docker-terraform:1.1.2"
	}

	meta.TerraformBackendNamespace = os.Getenv("TERRAFORM_BACKEND_NAMESPACE")
	if meta.TerraformBackendNamespace == "" {
		meta.TerraformBackendNamespace = "vela-system"
	}

	meta.BusyboxImage = os.Getenv("BUSYBOX_IMAGE")
	if meta.BusyboxImage == "" {
		meta.BusyboxImage = "busybox:latest"
	}
	meta.GitImage = os.Getenv("GIT_IMAGE")
	if meta.GitImage == "" {
		meta.GitImage = "alpine/git:latest"
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

	// TODO(zzxwill) Need to find an alternative to check whether there is an state backend in the Configuration

	// Render configuration with backend
	completeConfiguration, err := tfcfg.RenderConfiguration(configuration, meta.TerraformBackendNamespace, configurationType)
	if err != nil {
		return err
	}
	meta.CompleteConfiguration = completeConfiguration

	if err := meta.storeTFConfiguration(ctx, k8sClient); err != nil {
		return err
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

	// Check provider
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

	if err := meta.getCredentials(ctx, k8sClient, p); err != nil {
		return err
	}

	// Check whether env changes
	if err := meta.prepareTFVariables(configuration); err != nil {
		return err
	}

	var variableInSecret v1.Secret
	err = k8sClient.Get(ctx, client.ObjectKey{Name: meta.VariableSecretName, Namespace: meta.Namespace}, &variableInSecret)
	switch {
	case kerrors.IsNotFound(err):
		var secret = v1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      meta.VariableSecretName,
				Namespace: meta.Namespace,
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

	// Apply ClusterRole
	return createTerraformExecutorClusterRole(ctx, k8sClient, fmt.Sprintf("%s-%s", meta.Namespace, ClusterRoleName))
}

func (meta *TFConfigurationMeta) updateApplyStatus(ctx context.Context, k8sClient client.Client, state types.ConfigurationState, message string) error {
	var configuration v1beta2.Configuration
	if err := k8sClient.Get(ctx, client.ObjectKey{Name: meta.Name, Namespace: meta.Namespace}, &configuration); err == nil {
		configuration.Status.Apply = v1beta2.ConfigurationApplyStatus{
			State:   state,
			Message: message,
		}
		configuration.Status.ObservedGeneration = configuration.Generation
		if state == types.Available {
			outputs, err := meta.getTFOutputs(ctx, k8sClient, configuration)
			if err != nil {
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
	if err := createTerraformExecutorServiceAccount(ctx, k8sClient, meta.Namespace, ServiceAccountName); err != nil {
		return err
	}
	if err := createTerraformExecutorClusterRoleBinding(ctx, k8sClient, meta.Namespace, fmt.Sprintf("%s-%s", meta.Namespace, ClusterRoleName), ServiceAccountName); err != nil {
		return err
	}

	job := meta.assembleTerraformJob(executionType)
	return k8sClient.Create(ctx, job)
}

// updateTerraformJob will set deletion finalizer to the Terraform job if its envs are changed, which will result in
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
		if err := k8sClient.Get(ctx, client.ObjectKey{Name: meta.VariableSecretName, Namespace: meta.Namespace}, &s); err == nil {
			if deleteErr := k8sClient.Delete(ctx, &s); deleteErr != nil {
				return deleteErr
			}
		}
	}
	return nil
}

func (meta *TFConfigurationMeta) assembleTerraformJob(executionType TerraformExecutionType) *batchv1.Job {
	var (
		initContainer  v1.Container
		initContainers []v1.Container
		parallelism    int32 = 1
		completions    int32 = 1
		backoffLimit   int32 = math.MaxInt32
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
		initContainers = append(initContainers,
			v1.Container{
				Name:            "git-configuration",
				Image:           meta.GitImage,
				ImagePullPolicy: v1.PullIfNotPresent,
				Command: []string{
					"sh",
					"-c",
					fmt.Sprintf("git clone %s %s && cp -r %s/* %s", meta.RemoteGit, BackendVolumeMountPath,
						hclPath, WorkingVolumeMountPath),
				},
				VolumeMounts: initContainerVolumeMounts,
			})
	}

	return &batchv1.Job{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Job",
			APIVersion: "batch/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      meta.Name + "-" + string(executionType),
			Namespace: meta.Namespace,
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
					Containers: []v1.Container{{
						Name:            terraformContainerName,
						Image:           meta.TerraformImage,
						ImagePullPolicy: v1.PullIfNotPresent,
						Command: []string{
							"bash",
							"-c",
							fmt.Sprintf("terraform init && terraform %s -lock=false -auto-approve", executionType),
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
					},
					},
					ServiceAccountName: ServiceAccountName,
					Volumes:            executorVolumes,
					RestartPolicy:      v1.RestartPolicyOnFailure,
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
	return []v1.Volume{workingVolume, inputTFConfigurationVolume, tfBackendVolume}
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

// TfStateProperty is the tf state property for an output
type TfStateProperty struct {
	Value interface{} `json:"value,omitempty"`
	Type  string      `json:"type,omitempty"`
}

// ToProperty converts TfStateProperty type to Property
func (tp *TfStateProperty) ToProperty() (v1beta2.Property, error) {
	var (
		property v1beta2.Property
		err      error
	)
	sv, err := tfcfg.Interface2String(tp.Value)
	if err != nil {
		return property, errors.Wrap(err, "failed to get terraform state outputs")
	}
	property = v1beta2.Property{
		Type:  tp.Type,
		Value: sv,
	}
	return property, err
}

// TFState is Terraform State
type TFState struct {
	Outputs map[string]TfStateProperty `json:"outputs"`
}

//nolint:funlen
func (meta *TFConfigurationMeta) getTFOutputs(ctx context.Context, k8sClient client.Client, configuration v1beta2.Configuration) (map[string]v1beta2.Property, error) {
	var s = v1.Secret{}
	if err := k8sClient.Get(ctx, client.ObjectKey{Name: meta.BackendSecretName, Namespace: meta.TerraformBackendNamespace}, &s); err != nil {
		return nil, errors.Wrap(err, "terraform state file backend secret is not generated")
	}
	tfStateData, ok := s.Data[TerraformStateNameInSecret]
	if !ok {
		return nil, fmt.Errorf("failed to get %s from Terraform State secret %s", TerraformStateNameInSecret, s.Name)
	}

	tfStateJSON, err := util.DecompressTerraformStateSecret(string(tfStateData))
	if err != nil {
		return nil, errors.Wrap(err, "failed to decompress state secret data")
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
		ns = "default"
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
						"created-by":                 "terraform-controller",
						"terraform-controller-owner": configurationName,
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
		if owner, ok := gotSecret.ObjectMeta.Labels["terraform-controller-owner"]; ok && owner != configurationName {
			return nil, fmt.Errorf(
				"configuration(%s) cannot update secret(%s) which owner is configuration(%s)",
				configurationName, name, owner)
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
	if meta.ProviderReference == nil {
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
		valueFrom := &v1.EnvVarSource{SecretKeyRef: &v1.SecretKeySelector{Key: k}}
		valueFrom.SecretKeyRef.Name = meta.VariableSecretName
		envs = append(envs, v1.EnvVar{Name: k, ValueFrom: valueFrom})
	}

	if meta.Credentials == nil {
		return errors.New(provider.ErrCredentialNotRetrieved)
	}
	for k, v := range meta.Credentials {
		data[k] = []byte(v)
		valueFrom := &v1.EnvVarSource{SecretKeyRef: &v1.SecretKeySelector{Key: k}}
		valueFrom.SecretKeyRef.Name = meta.VariableSecretName
		envs = append(envs, v1.EnvVar{Name: k, ValueFrom: valueFrom})
	}
	// make sure the env of the Job is set
	if envs == nil {
		return errors.New(provider.ErrCredentialNotRetrieved)
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

func (meta *TFConfigurationMeta) deleteConfigMap(ctx context.Context, k8sClient client.Client) error {
	var cm v1.ConfigMap
	if err := k8sClient.Get(ctx, client.ObjectKey{Name: meta.ConfigurationCMName, Namespace: meta.Namespace}, &cm); err == nil {
		if err := k8sClient.Delete(ctx, &cm); err != nil {
			return err
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
		ns = "default"
	}
	if err := k8sClient.Get(ctx, client.ObjectKey{Name: name, Namespace: ns}, &connectionSecret); err == nil {
		return k8sClient.Delete(ctx, &connectionSecret)
	}
	return nil
}

func (meta *TFConfigurationMeta) createOrUpdateConfigMap(ctx context.Context, k8sClient client.Client, data map[string]string) error {
	var gotCM v1.ConfigMap
	if err := k8sClient.Get(ctx, client.ObjectKey{Name: meta.ConfigurationCMName, Namespace: meta.Namespace}, &gotCM); err != nil {
		if kerrors.IsNotFound(err) {
			cm := v1.ConfigMap{
				TypeMeta: metav1.TypeMeta{APIVersion: "v1", Kind: "ConfigMap"},
				ObjectMeta: metav1.ObjectMeta{
					Name:      meta.ConfigurationCMName,
					Namespace: meta.Namespace,
				},
				Data: data,
			}
			err := k8sClient.Create(ctx, &cm)
			return errors.Wrap(err, "failed to create TF configuration ConfigMap")
		}
		return err
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
	case types.ConfigurationJSON:
		dataName = types.TerraformJSONConfigurationName
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
	if err := k8sClient.Get(ctx, client.ObjectKey{Name: meta.ConfigurationCMName, Namespace: meta.Namespace}, &cm); err != nil {
		return err
	}

	var configurationChanged bool
	switch configurationType {
	case types.ConfigurationJSON:
		meta.ConfigurationChanged = true
		return nil
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
	}

	return errors.New("unknown issue")
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
	return nil
}
