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
	"k8s.io/utils/pointer"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/oam-dev/terraform-controller/api/types"
	"github.com/oam-dev/terraform-controller/api/v1beta1"
	cfgvalidator "github.com/oam-dev/terraform-controller/controllers/configuration"
	"github.com/oam-dev/terraform-controller/controllers/util"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const (
	terraformInitContainerImg = "busybox:latest"
	// TerraformImage is the Terraform image which can run `terraform init/plan/apply`
	terraformImage     = "oamdev/docker-terraform:1.0.4"
	terraformWorkspace = "default"
)

const (
	// WorkingVolumeMountPath is the mount path for working volume
	WorkingVolumeMountPath = "/data"
	// InputTFConfigurationVolumeName is the volume name for input Terraform Configuration
	InputTFConfigurationVolumeName = "tf-input-configuration"
	// InputTFConfigurationVolumeMountPath is the volume mount path for input Terraform Configuration
	InputTFConfigurationVolumeMountPath = "/opt/tfconfiguration"
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
)

// ConfigurationReconciler reconciles a Configuration object.
type ConfigurationReconciler struct {
	client.Client
	Log          logr.Logger
	Scheme       *runtime.Scheme
	ProviderName string
}

var controllerNamespace = os.Getenv("CONTROLLER_NAMESPACE")

// +kubebuilder:rbac:groups=terraform.core.oam.dev,resources=configurations,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=terraform.core.oam.dev,resources=configurations/status,verbs=get;update;patch

// Reconcile will reconcile periodically
func (r *ConfigurationReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	var (
		configuration         v1beta1.Configuration
		ctx                   = context.Background()
		tfInputConfigMapsName = fmt.Sprintf(TFInputConfigMapName, req.Name)
	)
	klog.InfoS("reconciling Terraform Configuration...", "NamespacedName", req.NamespacedName)
	applyJobName := req.Name + "-" + string(TerraformApply)

	if err := r.Get(ctx, req.NamespacedName, &configuration); err != nil {
		if kerrors.IsNotFound(err) {
			klog.ErrorS(err, "unable to fetch Configuration", "NamespacedName", req.NamespacedName)
			err = nil
		}
		return ctrl.Result{}, err
	}

	if configuration.ObjectMeta.DeletionTimestamp.IsZero() {
		if !controllerutil.ContainsFinalizer(&configuration, configurationFinalizer) {
			controllerutil.AddFinalizer(&configuration, configurationFinalizer)
			if err := r.Update(ctx, &configuration); err != nil {
				return ctrl.Result{RequeueAfter: 3 * time.Second}, errors.Wrap(err, "failed to add finalizer")
			}
		}
	} else {
		// terraform destroy
		if controllerutil.ContainsFinalizer(&configuration, configurationFinalizer) {
			klog.InfoS("performing Terraform Destroy", "Namespace", req.Namespace, "Name", req.Name)
			destroyJobName := req.Name + "-" + string(TerraformDestroy)
			klog.InfoS("Terraform destroy job", "Namespace", req.Namespace, "Name", destroyJobName)
			if err := r.terraformDestroy(ctx, req.Namespace, configuration, destroyJobName, tfInputConfigMapsName, applyJobName); err != nil {
				if err.Error() == MessageDestroyJobNotCompleted {
					return ctrl.Result{RequeueAfter: 3 * time.Second}, nil
				}
				return ctrl.Result{RequeueAfter: 3 * time.Second}, errors.Wrap(err, "continue reconciling to destroy cloud resource")
			}
			controllerutil.RemoveFinalizer(&configuration, configurationFinalizer)
			if err := r.Update(ctx, &configuration); err != nil {
				return ctrl.Result{RequeueAfter: 3 * time.Second}, errors.Wrap(err, "failed to remove finalizer")
			}
		}
		return ctrl.Result{}, nil
	}

	// Terraform apply (create or update)
	klog.InfoS("performing Terraform Apply (cloud resource create/update)", "Namespace", req.Namespace, "Name", req.Name)
	if configuration.Spec.ProviderReference != nil {
		r.ProviderName = configuration.Spec.ProviderReference.Name
	}
	if err := r.terraformApply(ctx, req.Namespace, configuration, applyJobName, tfInputConfigMapsName); err != nil {
		if err.Error() == MessageApplyJobNotCompleted {
			return ctrl.Result{RequeueAfter: 3 * time.Second}, nil
		}
		return ctrl.Result{RequeueAfter: 3 * time.Second}, errors.Wrap(err, "failed to create/update cloud resource")
	}
	return ctrl.Result{}, nil
}

func (r *ConfigurationReconciler) terraformApply(ctx context.Context, namespace string, configuration v1beta1.Configuration, applyJobName, tfInputConfigMapName string) error {
	klog.InfoS("terraform apply job", "Namespace", namespace, "Name", applyJobName)

	var (
		k8sClient      = r.Client
		tfExecutionJob batchv1.Job
	)

	var inputConfigurationCM v1.ConfigMap
	r.Client.Get(ctx, client.ObjectKey{Name: tfInputConfigMapName, Namespace: namespace}, &inputConfigurationCM) //nolint:errcheck

	// validation: 1) validate Configuration itself
	configurationType, err := cfgvalidator.ValidConfigurationObject(&configuration)
	if err != nil {
		configuration.Status.State = types.ConfigurationStaticChecking
		configuration.Status.Message = err.Error()
		if err := k8sClient.Status().Update(ctx, &configuration); err != nil {
			return err
		}
		return err
	}

	// validation: 2) validate Configuration syntax
	if err := cfgvalidator.CheckConfigurationSyntax(&configuration, configurationType); err != nil {
		configuration.Status.State = types.ConfigurationSyntaxChecking
		configuration.Status.Message = err.Error()
		if err := k8sClient.Status().Update(ctx, &configuration); err != nil {
			return err
		}
		return err
	}

	// Compose configuration with backend, and check whether configuration(hcl/json) is changed
	inputConfiguration, configurationChanged, err := cfgvalidator.ComposeConfiguration(&configuration, controllerNamespace, configurationType, &inputConfigurationCM)
	if err != nil {
		return err
	}

	if err := k8sClient.Get(ctx, client.ObjectKey{Name: applyJobName, Namespace: controllerNamespace}, &tfExecutionJob); err != nil {
		if kerrors.IsNotFound(err) {
			// store configuration to ConfigMap
			if err := storeTFConfiguration(ctx, k8sClient, configurationType, inputConfiguration, tfInputConfigMapName); err != nil {
				return err
			}
			return assembleAndTriggerJob(ctx, k8sClient, configuration.Name, &configuration, tfInputConfigMapName, namespace, r.ProviderName, TerraformApply)
		}
	}

	// start provisioning
	configuration.Status.State = types.Provisioning
	configuration.Status.Message = "Cloud resources are being provisioned..."
	if err := k8sClient.Status().Update(ctx, &configuration); err != nil {
		return err
	}

	if err := r.updateTerraformJobIfNeeded(ctx, namespace, configuration, tfExecutionJob, configurationChanged); err != nil {
		errMsg := "Hit an issue to update Terraform apply job"
		klog.ErrorS(err, errMsg, "Name", applyJobName)
		return errors.Wrap(err, errMsg)
	}
	// store configuration to ConfigMap
	if err := storeTFConfiguration(ctx, k8sClient, configurationType, inputConfiguration, tfInputConfigMapName); err != nil {
		return err
	}

	if tfExecutionJob.Status.Succeeded == int32(1) && configuration.Status.State != types.Available {
		configuration.Status.State = types.Available
		configuration.Status.Message = "Cloud resources are deployed and ready to use."
		outputs, err := getTFOutputs(ctx, k8sClient, configuration)
		if err != nil {
			return err
		}
		configuration.Status.Outputs = outputs
		if err := k8sClient.Status().Update(ctx, &configuration); err != nil {
			return err
		}
	}
	return nil
}

func (r *ConfigurationReconciler) terraformDestroy(ctx context.Context, namespace string, configuration v1beta1.Configuration, destroyJobName, tfInputConfigMapsName, applyJobName string) error {
	var (
		destroyJob           batchv1.Job
		inputConfigurationCM v1.ConfigMap
		k8sClient            = r.Client
	)

	if err := k8sClient.Get(ctx, client.ObjectKey{Name: destroyJobName, Namespace: controllerNamespace}, &destroyJob); err != nil {
		if kerrors.IsNotFound(err) {
			if err = assembleAndTriggerJob(ctx, k8sClient, configuration.Name, &configuration, tfInputConfigMapsName, namespace, r.ProviderName, TerraformDestroy); err != nil {
				return err
			}
		}
	}

	configurationType, _ := cfgvalidator.ValidConfigurationObject(&configuration)
	r.Client.Get(ctx, client.ObjectKey{Name: tfInputConfigMapsName, Namespace: namespace}, &inputConfigurationCM) //nolint:errcheck
	_, configurationChanged, err := cfgvalidator.ComposeConfiguration(&configuration, controllerNamespace, configurationType, &inputConfigurationCM)
	if err != nil {
		return err
	}
	if err := r.updateTerraformJobIfNeeded(ctx, namespace, configuration, destroyJob, configurationChanged); err != nil {
		klog.ErrorS(err, "Hit an issue to update Terraform delete job", "Name", destroyJobName)
		return err
	}

	// When the deletion Job process succeeded, clean up work is starting.
	if destroyJob.Status.Succeeded == *pointer.Int32Ptr(1) {
		// 1. delete Terraform input Configuration ConfigMap and ConnectionSecret
		if err := deleteConfigMap(ctx, k8sClient, tfInputConfigMapsName); err != nil {
			return err
		}

		if configuration.Spec.WriteConnectionSecretToReference != nil {
			secretName := configuration.Spec.WriteConnectionSecretToReference.Name
			secretNameSpace := configuration.Spec.WriteConnectionSecretToReference.Namespace
			if err = deleteConnectionSecret(ctx, k8sClient, secretName, secretNameSpace); err != nil {
				return err
			}
		}

		// 2. we don't manually delete Terraform state file Secret, as Terraform Kubernetes backend tends to keep the secret

		// 3. delete apply job
		var applyJob batchv1.Job
		if err := k8sClient.Get(ctx, client.ObjectKey{Name: applyJobName, Namespace: controllerNamespace}, &applyJob); err == nil {
			if err := k8sClient.Delete(ctx, &applyJob, client.PropagationPolicy(metav1.DeletePropagationBackground)); err != nil {
				return err
			}
		}

		// 4. delete destroy job
		return k8sClient.Delete(ctx, &destroyJob, client.PropagationPolicy(metav1.DeletePropagationBackground))
	}
	return errors.New(MessageDestroyJobNotCompleted)
}

func assembleAndTriggerJob(ctx context.Context, k8sClient client.Client, name string, configuration *v1beta1.Configuration,
	tfInputConfigMapsName, providerNamespace, providerName string, executionType TerraformExecutionType) error {
	jobName := name + "-" + string(executionType)

	envs, err := prepareTFVariables(ctx, k8sClient, configuration, providerName, providerNamespace)
	if err != nil {
		return err
	}

	job := assembleTerraformJob(name, jobName, tfInputConfigMapsName, envs, executionType)
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
		return r.Client.Delete(ctx, &job, client.PropagationPolicy(metav1.DeletePropagationBackground))
	}
	return nil
}

func assembleTerraformJob(name, jobName string, tfInputConfigMapsName string, envs []v1.EnvVar, executionType TerraformExecutionType) *batchv1.Job {
	var parallelism int32 = 1
	var completions int32 = 1

	initContainerVolumeMounts := []v1.VolumeMount{
		{
			Name:      name,
			MountPath: WorkingVolumeMountPath,
		},
		{
			Name:      InputTFConfigurationVolumeName,
			MountPath: InputTFConfigurationVolumeMountPath,
		},
	}
	initContainerCMD := fmt.Sprintf("cp %s/* %s", InputTFConfigurationVolumeMountPath, WorkingVolumeMountPath)

	executorVolumes := assembleExecutorVolumes(name, tfInputConfigMapsName)

	return &batchv1.Job{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Job",
			APIVersion: "batch/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      jobName,
			Namespace: controllerNamespace,
		},
		Spec: batchv1.JobSpec{
			Parallelism: &parallelism,
			Completions: &completions,
			Template: v1.PodTemplateSpec{
				Spec: v1.PodSpec{
					// InitContainer will copy Terraform configuration files to working directory and create Terraform
					// state file directory in advance
					InitContainers: []v1.Container{{
						Name:            "prepare-input-terraform-configurations",
						Image:           terraformInitContainerImg,
						ImagePullPolicy: v1.PullIfNotPresent,
						Command: []string{
							"sh",
							"-c",
							initContainerCMD,
						},
						VolumeMounts: initContainerVolumeMounts,
					}},
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
								Name:      name,
								MountPath: WorkingVolumeMountPath,
							},
							{
								Name:      InputTFConfigurationVolumeName,
								MountPath: InputTFConfigurationVolumeMountPath,
							},
						},
						Env: envs,
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

func assembleExecutorVolumes(name, tfInputConfigMapsName string) []v1.Volume {
	workingVolume := v1.Volume{Name: name}
	workingVolume.EmptyDir = &v1.EmptyDirVolumeSource{}
	inputTFConfigurationVolume := createConfigurationVolume(tfInputConfigMapsName)
	return []v1.Volume{workingVolume, inputTFConfigurationVolume}
}

func createConfigurationVolume(tfInputConfigMapsName string) v1.Volume {
	inputCMVolumeSource := v1.ConfigMapVolumeSource{}
	inputCMVolumeSource.Name = tfInputConfigMapsName
	inputTFConfigurationVolume := v1.Volume{Name: InputTFConfigurationVolumeName}
	inputTFConfigurationVolume.ConfigMap = &inputCMVolumeSource
	return inputTFConfigurationVolume
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
		errMsg := "failed to get OpenAPI credentials from the cloud provider"
		configuration.Status = v1beta1.ConfigurationStatus{
			State:   types.ConfigurationIsPreChecking,
			Message: fmt.Sprintf("%s: %s", errMsg, err.Error()),
		}
		if updateErr := k8sClient.Status().Update(ctx, configuration); updateErr != nil {
			klog.ErrorS(updateErr, errSettingStatus, "ConfigurationNamespace", configuration.Namespace,
				"ConfigurationName", configuration.Namespace)
			return nil, errors.Wrap(updateErr, errSettingStatus)
		}
		return nil, errors.Wrap(err, errMsg)
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

func createOrUpdateConfigMap(ctx context.Context, k8sClient client.Client, name string, data map[string]string) error {
	var gotCM v1.ConfigMap
	if err := k8sClient.Get(ctx, client.ObjectKey{Name: name, Namespace: controllerNamespace}, &gotCM); err != nil {
		if kerrors.IsNotFound(err) {
			cm := v1.ConfigMap{
				TypeMeta: metav1.TypeMeta{APIVersion: "v1", Kind: "ConfigMap"},
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
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

func prepareTFInputConfigurationData(configurationType types.ConfigurationType, inputConfiguration string) map[string]string {
	var dataName string
	switch configurationType {
	case types.ConfigurationJSON:
		dataName = types.TerraformJSONConfigurationName
	case types.ConfigurationHCL:
		dataName = types.TerraformHCLConfigurationName
	}
	data := map[string]string{dataName: inputConfiguration, "kubeconfig": ""}
	return data
}

// storeTFConfiguration will store Terraform configuration to ConfigMap
func storeTFConfiguration(ctx context.Context, k8sClient client.Client, configurationType types.ConfigurationType, inputConfiguration string, tfInputConfigMapName string) error {
	data := prepareTFInputConfigurationData(configurationType, inputConfiguration)
	return createOrUpdateConfigMap(ctx, k8sClient, tfInputConfigMapName, data)
}
