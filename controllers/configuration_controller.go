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
	"context"
	"encoding/json"
	"fmt"
	"os"
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

	"github.com/oam-dev/terraform-controller/api/types"
	"github.com/oam-dev/terraform-controller/api/v1beta1"
	cfgvalidator "github.com/oam-dev/terraform-controller/controllers/configuration"
	"github.com/oam-dev/terraform-controller/controllers/util"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const (
	// TerraformImage is the Terraform image which can run `terraform init/plan/apply`
	terraformImage     = "oamdev/docker-terraform:1.0.6"
	terraformWorkspace = "default"
)

const (
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
)

const (
	// TerraformStateNameInSecret is the key name to store Terraform state
	TerraformStateNameInSecret = "tfstate"
	// TFInputConfigMapName is the CM name for Terraform Input Configuration
	TFInputConfigMapName = "%s-tf-input"
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
)

const (
	// MessageDestroyJobNotCompleted is the message when Configuration deletion isn't completed
	MessageDestroyJobNotCompleted = "Configuration deletion isn't completed"
	// MessageApplyJobNotCompleted is the message when cloud resources are not created completed
	MessageApplyJobNotCompleted = "cloud resources are not created completed"
	// MessageCloudResourceProvisioningAndChecking is the message when cloud resource is being provisioned
	MessageCloudResourceProvisioningAndChecking = "Cloud resources are being provisioned and provisioning status is checking..."
	// ErrUpdateTerraformApplyJob means hitting  an issue to update Terraform apply job
	ErrUpdateTerraformApplyJob = "Hit an issue to update Terraform apply job"
	// MessageCloudResourceDeployed means Cloud resources are deployed and ready to use
	MessageCloudResourceDeployed = "Cloud resources are deployed and ready to use"
	// MessageCloudResourceDestroying is the message when cloud resource is being destroyed
	MessageCloudResourceDestroying = "Cloud resources is being destroyed..."
	// ErrProviderNotReady means provider object is not ready
	ErrProviderNotReady = "Provider is not ready"
)

// ConfigurationReconciler reconciles a Configuration object.
type ConfigurationReconciler struct {
	client.Client
	Log          logr.Logger
	Scheme       *runtime.Scheme
	ProviderName string
}

var controllerNamespace = os.Getenv("CONTROLLER_NAMESPACE")

// TFConfigurationMeta is all the metadata of a Configuration
type TFConfigurationMeta struct {
	Name                  string
	Namespace             string
	ConfigurationType     types.ConfigurationType
	CompleteConfiguration string
	RemoteGit             string
	ConfigurationChanged  bool
	ConfigurationCMName   string
	BackendCMName         string
	ApplyJobName          string
	DestroyJobName        string
	Envs                  []v1.EnvVar
}

// +kubebuilder:rbac:groups=terraform.core.oam.dev,resources=configurations,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=terraform.core.oam.dev,resources=configurations/status,verbs=get;update;patch

// Reconcile will reconcile periodically
func (r *ConfigurationReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	var (
		configuration v1beta1.Configuration
		ctx           = context.Background()
		meta          = &TFConfigurationMeta{
			Namespace:           controllerNamespace,
			Name:                req.Name,
			ConfigurationCMName: fmt.Sprintf(TFInputConfigMapName, req.Name),
			ApplyJobName:        req.Name + "-" + string(TerraformApply),
			DestroyJobName:      req.Name + "-" + string(TerraformDestroy),
		}
	)
	klog.InfoS("reconciling Terraform Configuration...", "NamespacedName", req.NamespacedName)

	if err := r.Get(ctx, req.NamespacedName, &configuration); err != nil {
		if kerrors.IsNotFound(err) {
			klog.ErrorS(err, "unable to fetch Configuration", "NamespacedName", req.NamespacedName)
			err = nil
		}
		return ctrl.Result{}, err
	}
	meta.RemoteGit = configuration.Spec.Remote

	if configuration.ObjectMeta.DeletionTimestamp.IsZero() {
		if !controllerutil.ContainsFinalizer(&configuration, configurationFinalizer) {
			controllerutil.AddFinalizer(&configuration, configurationFinalizer)
			if err := r.Update(ctx, &configuration); err != nil {
				return ctrl.Result{RequeueAfter: 3 * time.Second}, errors.Wrap(err, "failed to add finalizer")
			}
		}
	}

	if err := r.preCheck(ctx, &configuration, meta); err != nil {
		return ctrl.Result{}, err
	}

	if !configuration.ObjectMeta.DeletionTimestamp.IsZero() {
		// terraform destroy
		klog.InfoS("Performing Configuration Destroy", "Namespace", req.Namespace, "Name", req.Name, "JobName", meta.DestroyJobName)
		if err := r.terraformDestroy(ctx, req.Namespace, configuration, meta); err != nil {
			if err.Error() == MessageDestroyJobNotCompleted {
				return ctrl.Result{RequeueAfter: 3 * time.Second}, nil
			}
			return ctrl.Result{RequeueAfter: 3 * time.Second}, errors.Wrap(err, "continue reconciling to destroy cloud resource")
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
	if configuration.Status.Apply.State == types.ConfigurationSyntaxError || configuration.Status.Apply.State == types.ProviderNotReady {
		return ctrl.Result{RequeueAfter: 3 * time.Second}, nil
	}
	klog.InfoS("performing Terraform Apply (cloud resource create/update)", "Namespace", req.Namespace, "Name", req.Name)
	if configuration.Spec.ProviderReference != nil {
		r.ProviderName = configuration.Spec.ProviderReference.Name
	}
	if err := r.terraformApply(ctx, req.Namespace, configuration, meta); err != nil {
		if err.Error() == MessageApplyJobNotCompleted {
			return ctrl.Result{RequeueAfter: 3 * time.Second}, nil
		}
		return ctrl.Result{RequeueAfter: 3 * time.Second}, errors.Wrap(err, "failed to create/update cloud resource")
	}
	return ctrl.Result{}, nil
}

func (r *ConfigurationReconciler) terraformApply(ctx context.Context, namespace string, configuration v1beta1.Configuration, meta *TFConfigurationMeta) error {
	klog.InfoS("terraform apply job", "Namespace", namespace, "Name", meta.ApplyJobName)

	var (
		k8sClient      = r.Client
		tfExecutionJob batchv1.Job
	)

	// start provisioning and check the status of the provision
	if configuration.Status.Apply.State != types.Available {
		// For not serious, we don't want to throw an error
		updateStatus(ctx, k8sClient, configuration, types.ConfigurationProvisioningAndChecking, MessageCloudResourceProvisioningAndChecking) //nolint:errcheck
	}

	if err := k8sClient.Get(ctx, client.ObjectKey{Name: meta.ApplyJobName, Namespace: controllerNamespace}, &tfExecutionJob); err != nil {
		if kerrors.IsNotFound(err) {
			return meta.assembleAndTriggerJob(ctx, k8sClient, &configuration, namespace, r.ProviderName, TerraformApply)
		}
	}

	if err := r.updateTerraformJobIfNeeded(ctx, namespace, configuration, tfExecutionJob, meta.ConfigurationChanged); err != nil {
		klog.ErrorS(err, ErrUpdateTerraformApplyJob, "Name", meta.ApplyJobName)
		return errors.Wrap(err, ErrUpdateTerraformApplyJob)
	}

	if tfExecutionJob.Status.Succeeded == int32(1) && configuration.Status.Apply.State != types.Available {
		return updateStatus(ctx, k8sClient, configuration, types.Available, MessageCloudResourceDeployed)
	}
	return nil
}

func (r *ConfigurationReconciler) terraformDestroy(ctx context.Context, namespace string, configuration v1beta1.Configuration, meta *TFConfigurationMeta) error {
	var (
		destroyJob batchv1.Job
		k8sClient  = r.Client
	)
	if configuration.Status.Apply.State == types.ConfigurationProvisioningAndChecking {
		warning := fmt.Sprintf("Destroy could not complete and needs to wait for Provision to complet first: %s", MessageCloudResourceProvisioningAndChecking)
		klog.Warning(warning)
		return errors.New(warning)
	}

	if err := k8sClient.Get(ctx, client.ObjectKey{Name: meta.DestroyJobName, Namespace: meta.Namespace}, &destroyJob); err != nil {
		if kerrors.IsNotFound(err) {
			if err := r.Client.Get(ctx, client.ObjectKey{Name: configuration.Name, Namespace: configuration.Namespace}, &v1beta1.Configuration{}); err == nil {
				if err = meta.assembleAndTriggerJob(ctx, k8sClient, &configuration, namespace, r.ProviderName, TerraformDestroy); err != nil {
					return err
				}
			}
		}
	}

	// destroying
	if err := updateStatus(ctx, k8sClient, configuration, types.ConfigurationDestroying, MessageCloudResourceDestroying); err != nil {
		return err
	}

	if err := r.updateTerraformJobIfNeeded(ctx, namespace, configuration, destroyJob, meta.ConfigurationChanged); err != nil {
		klog.ErrorS(err, ErrUpdateTerraformApplyJob, "Name", meta.ApplyJobName)
		return errors.Wrap(err, ErrUpdateTerraformApplyJob)
	}

	// When the deletion Job process succeeded, clean up work is starting.
	if destroyJob.Status.Succeeded == int32(1) {
		// 1. delete Terraform input Configuration ConfigMap
		if err := deleteConfigMap(ctx, k8sClient, meta.ConfigurationCMName); err != nil {
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
		if err := k8sClient.Get(ctx, client.ObjectKey{Name: meta.ApplyJobName, Namespace: controllerNamespace}, &applyJob); err == nil {
			if err := k8sClient.Delete(ctx, &applyJob, client.PropagationPolicy(metav1.DeletePropagationBackground)); err != nil {
				return err
			}
		}

		// 4. delete destroy job
		var j batchv1.Job
		if err := r.Client.Get(ctx, client.ObjectKey{Name: destroyJob.Name, Namespace: destroyJob.Namespace}, &j); err == nil {
			return r.Client.Delete(ctx, &j, client.PropagationPolicy(metav1.DeletePropagationBackground))
		}
	}
	return errors.New(MessageDestroyJobNotCompleted)
}

func (r *ConfigurationReconciler) preCheck(ctx context.Context, configuration *v1beta1.Configuration, meta *TFConfigurationMeta) error {
	var (
		k8sClient = r.Client
	)

	// Validation: 1) validate Configuration itself
	configurationType, err := cfgvalidator.ValidConfigurationObject(configuration)
	if err != nil {
		return updateStatus(ctx, k8sClient, *configuration, types.ConfigurationStaticChecking, err.Error())
	}
	meta.ConfigurationType = configurationType

	// Validation: 2) validate Configuration syntax
	if err := cfgvalidator.CheckConfigurationSyntax(configuration, configurationType); err != nil {
		return updateStatus(ctx, k8sClient, *configuration, types.ConfigurationSyntaxError, err.Error())
	}
	if configuration.Status.Apply.State == types.ConfigurationSyntaxError {
		updateStatus(ctx, k8sClient, *configuration, types.ConfigurationSyntaxGood, "") //nolint:errcheck
	}

	// Render configuration with backend
	completeConfiguration, err := cfgvalidator.RenderConfiguration(configuration, controllerNamespace, configurationType)
	if err != nil {
		return err
	}
	meta.CompleteConfiguration = completeConfiguration

	var inputConfigurationCM v1.ConfigMap
	if err := r.Client.Get(ctx, client.ObjectKey{Name: meta.ConfigurationCMName, Namespace: controllerNamespace}, &inputConfigurationCM); err != nil {
		if kerrors.IsNotFound(err) {
			klog.InfoS("The input Configuration ConfigMaps doesn't exist", "Namespace", controllerNamespace, "Name", meta.ConfigurationCMName)
		} else {
			return err
		}
	}

	// Check whether configuration(hcl/json) is changed
	configurationChanged, err := cfgvalidator.CheckWhetherConfigurationChanges(configurationType, &inputConfigurationCM, completeConfiguration)
	if err != nil {
		return err
	}
	meta.ConfigurationChanged = configurationChanged
	if configurationChanged {
		// store configuration to ConfigMap
		return meta.storeTFConfiguration(ctx, k8sClient)
	}
	return nil
}

func updateStatus(ctx context.Context, k8sClient client.Client, configuration v1beta1.Configuration, state types.ConfigurationState, message string) error {
	if !configuration.ObjectMeta.DeletionTimestamp.IsZero() {
		configuration.Status.Destroy = v1beta1.ConfigurationDestroyStatus{
			State:   state,
			Message: message,
		}
	} else {
		configuration.Status.Apply = v1beta1.ConfigurationApplyStatus{
			State:   state,
			Message: message,
		}
		if state == types.Available {
			outputs, err := getTFOutputs(ctx, k8sClient, configuration)
			if err != nil {
				return err
			}
			configuration.Status.Apply.Outputs = outputs
		}
	}
	return k8sClient.Status().Update(ctx, &configuration)
}

func (meta *TFConfigurationMeta) assembleAndTriggerJob(ctx context.Context, k8sClient client.Client, configuration *v1beta1.Configuration,
	providerNamespace, providerName string, executionType TerraformExecutionType) error {
	envs, err := prepareTFVariables(ctx, k8sClient, configuration, providerName, providerNamespace)
	if err != nil {
		return err
	}
	meta.Envs = envs

	job := meta.assembleTerraformJob(executionType)
	return k8sClient.Create(ctx, job)
}

// updateTerraformJob will set deletion finalizer to the Terraform job if its envs are changed, which will result in
// deleting the job. Finally a new Terraform job will be generated
func (r *ConfigurationReconciler) updateTerraformJobIfNeeded(ctx context.Context, namespace string,
	configuration v1beta1.Configuration, job batchv1.Job, configurationChanged bool) error {

	envs, err := prepareTFVariables(ctx, r.Client, &configuration, r.ProviderName, namespace)
	if err != nil {
		return err
	}

	// check whether env changes
	var envChanged bool
	if len(job.Spec.Template.Spec.Containers) == 1 && !cfgvalidator.CompareTwoContainerEnvs(job.Spec.Template.Spec.Containers[0].Env, envs) {
		envChanged = true
		klog.InfoS("Job's env changed", "Previous", envs, "Current", job.Spec.Template.Spec.Containers[0].Env)
	}

	if configurationChanged {
		klog.InfoS("configuration(hcl/json) changed")
	}

	// if either one changes, delete the job
	if envChanged || configurationChanged {
		var j batchv1.Job
		if err := r.Client.Get(ctx, client.ObjectKey{Name: job.Name, Namespace: job.Namespace}, &j); err == nil {
			return r.Client.Delete(ctx, &job, client.PropagationPolicy(metav1.DeletePropagationBackground))
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
		Image:           "busybox:latest",
		ImagePullPolicy: v1.PullIfNotPresent,
		Command: []string{
			"sh",
			"-c",
			fmt.Sprintf("cp %s/* %s", InputTFConfigurationVolumeMountPath, WorkingVolumeMountPath),
		},
		VolumeMounts: initContainerVolumeMounts,
	}
	initContainers = append(initContainers, initContainer)

	if meta.RemoteGit != "" {
		initContainers = append(initContainers,
			v1.Container{
				Name:            "git-configuration",
				Image:           "alpine/git:latest",
				ImagePullPolicy: v1.PullIfNotPresent,
				Command: []string{
					"sh",
					"-c",
					fmt.Sprintf("git clone %s %s && cp -r %s/* %s", meta.RemoteGit, BackendVolumeMountPath,
						BackendVolumeMountPath, WorkingVolumeMountPath),
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
			Namespace: controllerNamespace,
		},
		Spec: batchv1.JobSpec{
			Parallelism: &parallelism,
			Completions: &completions,
			Template: v1.PodTemplateSpec{
				Spec: v1.PodSpec{
					// InitContainer will copy Terraform configuration files to working directory and create Terraform
					// state file directory in advance
					InitContainers: initContainers,
					// Container terraform-executor will first copy predefined terraform.d to working directory, and
					// then run terraform init/apply.
					Containers: []v1.Container{{
						Name:            "terraform-executor",
						Image:           terraformImage,
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
					ServiceAccountName: "tf-executor-service-account",
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

// TFState is Terraform State
type TFState struct {
	Outputs map[string]v1beta1.Property `json:"outputs"`
}

//nolint:funlen
func getTFOutputs(ctx context.Context, k8sClient client.Client, configuration v1beta1.Configuration) (map[string]v1beta1.Property, error) {
	var s = v1.Secret{}
	// Check the existence of Terraform state secret which is used to store TF state file. For detailed information,
	// please refer to https://www.terraform.io/docs/language/settings/backends/kubernetes.html#configuration-variables
	var backendSecretSuffix string
	if configuration.Spec.Backend != nil && configuration.Spec.Backend.SecretSuffix != "" {
		backendSecretSuffix = configuration.Spec.Backend.SecretSuffix
	} else {
		backendSecretSuffix = configuration.Name
	}
	// Secrets will be named in the format: tfstate-{workspace}-{secret_suffix}
	k8sBackendSecretName := fmt.Sprintf("tfstate-%s-%s", terraformWorkspace, backendSecretSuffix)
	if err := k8sClient.Get(ctx, client.ObjectKey{Name: k8sBackendSecretName, Namespace: controllerNamespace}, &s); err != nil {
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

	outputs := tfState.Outputs
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
	if err := k8sClient.Get(ctx, client.ObjectKey{Name: name, Namespace: ns}, &gotSecret); err != nil {
		if kerrors.IsNotFound(err) {
			var secret = v1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: ns,
				},
				TypeMeta: metav1.TypeMeta{Kind: "Secret"},
				Data:     data,
			}
			if err := k8sClient.Create(ctx, &secret); err != nil {
				return nil, err
			}
		}
	} else {
		gotSecret.Data = data
		if err := k8sClient.Update(ctx, &gotSecret); err != nil {
			return nil, err
		}
	}
	return outputs, nil
}

func prepareTFVariables(ctx context.Context, k8sClient client.Client, configuration *v1beta1.Configuration, providerName, providerNamespace string) ([]v1.EnvVar, error) {
	var envs []v1.EnvVar

	if configuration == nil {
		return nil, errors.New("configuration is nil")
	}

	tfVariable, err := getTerraformJSONVariable(configuration.Spec.Variable)
	if err != nil {
		return nil, errors.Wrap(err, fmt.Sprintf("failed to get Terraform JSON variables from Configuration Variables %v", configuration.Spec.Variable))
	}
	for k, v := range tfVariable {
		envs = append(envs, v1.EnvVar{Name: k, Value: v})
	}

	credential, err := util.GetProviderCredentials(ctx, k8sClient, providerNamespace, providerName)
	if err != nil {
		if updateStatusErr := updateStatus(ctx, k8sClient, *configuration, types.ProviderNotReady, ErrProviderNotReady); updateStatusErr != nil {
			return nil, errors.Wrap(updateStatusErr, errSettingStatus)
		}
		return nil, errors.Wrap(err, ErrProviderNotReady)
	}
	for k, v := range credential {
		envs = append(envs,
			v1.EnvVar{
				Name:  k,
				Value: v,
			})
	}
	return envs, nil
}

// SetupWithManager setups with a manager
func (r *ConfigurationReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1beta1.Configuration{}).
		Complete(r)
}

func getTerraformJSONVariable(tfVariables *runtime.RawExtension) (map[string]string, error) {
	variables, err := util.RawExtension2Map(tfVariables)
	if err != nil {
		return nil, err
	}
	var environments = make(map[string]string)

	for k, v := range variables {
		environments[fmt.Sprintf("TF_VAR_%s", k)] = fmt.Sprint(v)
	}
	return environments, nil
}

func deleteConfigMap(ctx context.Context, k8sClient client.Client, name string) error {
	var cm v1.ConfigMap
	if err := k8sClient.Get(ctx, client.ObjectKey{Name: name, Namespace: controllerNamespace}, &cm); err == nil {
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
	if err := k8sClient.Get(ctx, client.ObjectKey{Name: meta.ConfigurationCMName, Namespace: controllerNamespace}, &gotCM); err != nil {
		if kerrors.IsNotFound(err) {
			cm := v1.ConfigMap{
				TypeMeta: metav1.TypeMeta{APIVersion: "v1", Kind: "ConfigMap"},
				ObjectMeta: metav1.ObjectMeta{
					Name:      meta.ConfigurationCMName,
					Namespace: controllerNamespace,
				},
				Data: data,
			}
			err := k8sClient.Create(ctx, &cm)
			return errors.Wrap(err, "failed to create TF configuration ConfigMap")
		}
		return err
	}
	gotCM.Data = data
	err := k8sClient.Update(ctx, &gotCM)
	return errors.Wrap(err, "failed to update TF configuration ConfigMap")
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
