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

	state "github.com/oam-dev/terraform-controller/api/types"
	"github.com/oam-dev/terraform-controller/api/v1beta1"
	"github.com/oam-dev/terraform-controller/controllers/util"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const (
	// TerraformImage is the Terraform image which can run `terraform init/plan/apply`
	// hit issue `toomanyrequests` for "oamdev/docker-terraform:0.14.10"
	TerraformImage = "registry.cn-hongkong.aliyuncs.com/zzxwill/docker-terraform:0.14.11"

	TFStateRetrieverImage = "zzxwill/terraform-tfstate-retriever:v0.3"
	TerraformWorkspace    = "default"
)

const (
	WorkingVolumeMountPath              = "/data"
	InputTFConfigurationVolumeName      = "tf-input-configuration"
	InputTFConfigurationVolumeMountPath = "/opt/tfconfiguration"
)

const (
	TerraformJSONConfigurationName = "main.tf.json"
	TerraformHCLConfigurationName  = "main.tf"
	TerraformStateNameInSecret     = "tfstate"
	TFInputConfigMapName           = "%s-tf-input"
)

type TerraformExecutionType string

const (
	TerraformApply   TerraformExecutionType = "apply"
	TerraformDestroy TerraformExecutionType = "destroy"
)

const (
	configurationFinalizer = "configuration.finalizers.terraform-controller"
)

const (
	MessageDestroyJobNotCompleted = "configuration deletion isn't completed"
	MessageApplyJobNotCompleted   = "cloud resources are not created completed"
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

func (r *ConfigurationReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	var (
		configuration         v1beta1.Configuration
		ctx                   = context.Background()
		ns                    = req.Namespace
		configurationName     = req.Name
		tfInputConfigMapsName = fmt.Sprintf(TFInputConfigMapName, configurationName)
	)
	klog.InfoS("reconciling Terraform Configuration...", "NamespacedName", req.NamespacedName)
	applyJobName := configurationName + "-" + string(TerraformApply)

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
			destroyJobName := configurationName + "-" + string(TerraformDestroy)
			klog.InfoS("Terraform destroy job", "Namespace", req.Namespace, "Name", destroyJobName)
			if err := terraformDestroy(ctx, r.Client, ns, configuration, destroyJobName, tfInputConfigMapsName, applyJobName, r.ProviderName); err != nil {
				if err.Error() == MessageDestroyJobNotCompleted {
					return ctrl.Result{RequeueAfter: 3 * time.Second}, nil
				} else {
					return ctrl.Result{RequeueAfter: 3 * time.Second}, errors.Wrap(err, "continue reconciling to destroy cloud resource")
				}
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
	if err := terraformApply(ctx, r.Client, ns, configuration, applyJobName, tfInputConfigMapsName, r.ProviderName); err != nil {
		if err.Error() == MessageApplyJobNotCompleted {
			return ctrl.Result{RequeueAfter: 3 * time.Second}, nil
		} else {
			return ctrl.Result{RequeueAfter: 3 * time.Second}, errors.Wrap(err, "failed to create/update cloud resource")
		}
	}
	return ctrl.Result{}, nil
}

func terraformApply(ctx context.Context, k8sClient client.Client, namespace string, configuration v1beta1.Configuration, applyJobName, tfInputConfigMapName, providerName string) error {
	klog.InfoS("terraform apply job", "Namespace", namespace, "Name", applyJobName)

	var gotJob batchv1.Job
	err := k8sClient.Get(ctx, client.ObjectKey{Name: applyJobName, Namespace: namespace}, &gotJob)
	if err == nil {
		if gotJob.Status.Succeeded == *pointer.Int32Ptr(1) {
			outputs, err := getTFOutputs(ctx, k8sClient, configuration)
			if err != nil {
				return err
			}
			if configuration.Status.State != state.Available {
				configuration.Status.State = state.Available
				configuration.Status.Message = "Cloud resources are deployed and ready to use."
				configuration.Status.Outputs = outputs
				if err := k8sClient.Status().Update(ctx, &configuration); err != nil {
					return err
				}
			}
			return nil
		}
		return errors.New(MessageApplyJobNotCompleted)
	}

	if kerrors.IsNotFound(err) {
		if configuration.Status.State != state.Provisioning {
			configuration.Status.State = state.Provisioning
			configuration.Status.Message = "Cloud resources are being provisioned."
			if err := k8sClient.Status().Update(ctx, &configuration); err != nil {
				return err
			}
		}

		configurationType, inputConfiguration, err := util.ValidConfiguration(&configuration)
		if err != nil {
			return err
		}
		data := prepareTFInputConfigurationData(configurationType, inputConfiguration)
		if err := createOrUpdateConfigMap(ctx, k8sClient, namespace, tfInputConfigMapName, data); err != nil {
			return err
		}

		if err := assembleAndTriggerJob(ctx, k8sClient, namespace, configuration.Name, &configuration,
			tfInputConfigMapName, providerName, TerraformApply); err != nil {
			return err
		}
		return nil
	}

	if configuration.Status.State != state.Unavailable {
		configuration.Status.State = state.Unavailable
		configuration.Status.Message = fmt.Sprintf("The state of cloud resources is unavailable due to %s", err)
		if err := k8sClient.Status().Update(ctx, &configuration); err != nil {
			return err
		}
	}
	return err
}

func terraformDestroy(ctx context.Context, k8sClient client.Client, namespace string, configuration v1beta1.Configuration, destroyJobName, tfInputConfigMapsName, applyJobName, providerName string) error {
	var destroyJob batchv1.Job

	if err := k8sClient.Get(ctx, client.ObjectKey{Name: destroyJobName, Namespace: namespace}, &destroyJob); err != nil {
		if kerrors.IsNotFound(err) {
			if err = assembleAndTriggerJob(ctx, k8sClient, namespace, configuration.Name, &configuration, tfInputConfigMapsName,
				providerName, TerraformDestroy); err != nil {
				return err
			}
		}
	}
	// When the deletion Job process succeeded, clean up work is starting.
	if destroyJob.Status.Succeeded == *pointer.Int32Ptr(1) {
		// 1. delete Terraform input Configuration ConfigMap
		if err := deleteConfigMap(ctx, k8sClient, namespace, tfInputConfigMapsName); err != nil {
			return err
		}

		// 2. we don't manually delete Terraform state file Secret, as Terraform Kubernetes backend tends to keep the secret

		// 3. delete apply job
		var applyJob batchv1.Job
		if err := k8sClient.Get(ctx, client.ObjectKey{Name: applyJobName, Namespace: namespace}, &applyJob); err == nil {
			if err := k8sClient.Delete(ctx, &applyJob, client.PropagationPolicy(metav1.DeletePropagationBackground)); err != nil {
				return err
			}
		}

		// 4. delete destroy job
		if err := k8sClient.Delete(ctx, &destroyJob, client.PropagationPolicy(metav1.DeletePropagationBackground)); err != nil {
			return err
		}

		return nil
	}
	return errors.New(MessageDestroyJobNotCompleted)
}

func assembleAndTriggerJob(ctx context.Context, k8sClient client.Client, namespace, name string,
	configuration *v1beta1.Configuration, tfInputConfigMapsName, providerName string, executionType TerraformExecutionType) error {
	var job *batchv1.Job
	jobName := name + "-" + string(executionType)
	var parallelism int32 = 1
	var completions int32 = 1
	var ttlSecondsAfterFinished int32 = 0

	workingVolume := v1.Volume{Name: name}
	workingVolume.EmptyDir = &v1.EmptyDirVolumeSource{}

	inputCMVolumeSource := v1.ConfigMapVolumeSource{}
	inputCMVolumeSource.Name = tfInputConfigMapsName
	inputTFConfigurationVolume := v1.Volume{Name: InputTFConfigurationVolumeName}
	inputTFConfigurationVolume.ConfigMap = &inputCMVolumeSource

	envs, err := prepareTFVariables(ctx, k8sClient, namespace, configuration, providerName)
	if err != nil {
		return err
	}

	if executionType == TerraformApply {
		volumes := []v1.Volume{workingVolume, inputTFConfigurationVolume}
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

		job = &batchv1.Job{
			TypeMeta: metav1.TypeMeta{
				Kind:       "Job",
				APIVersion: "batch/v1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      jobName,
				Namespace: namespace,
				OwnerReferences: []metav1.OwnerReference{{
					APIVersion: configuration.APIVersion,
					Kind:       configuration.Kind,
					Name:       configuration.Name,
					UID:        configuration.UID,
					Controller: pointer.BoolPtr(false),
				}},
			},
			Spec: batchv1.JobSpec{
				// TODO(zzxwill) Not enabled in Kubernetes cluster lower than v1.21
				TTLSecondsAfterFinished: &ttlSecondsAfterFinished,
				Parallelism:             &parallelism,
				Completions:             &completions,
				Template: v1.PodTemplateSpec{
					Spec: v1.PodSpec{
						// InitContainer will copy Terraform configuration files to working directory and create Terraform
						// state file directory in advance
						InitContainers: []v1.Container{{
							Name:            "prepare-input-terraform-configurations",
							Image:           "busybox",
							ImagePullPolicy: v1.PullAlways,
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
							Image:           TerraformImage,
							ImagePullPolicy: v1.PullAlways,
							Command: []string{
								"bash",
								"-c",
								fmt.Sprintf("cp -r /root/terraform.d %s && terraform init &&"+
									" terraform apply -auto-approve", WorkingVolumeMountPath),
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
						ServiceAccountName: "tf-controller-service-account",
						Volumes:            volumes,
						RestartPolicy:      v1.RestartPolicyOnFailure,
					},
				},
			},
		}
	} else if executionType == TerraformDestroy {
		job = &batchv1.Job{
			TypeMeta: metav1.TypeMeta{
				Kind:       "Job",
				APIVersion: "batch/v1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      jobName,
				Namespace: namespace,
			},
			Spec: batchv1.JobSpec{
				TTLSecondsAfterFinished: &ttlSecondsAfterFinished,
				Parallelism:             &parallelism,
				Completions:             &completions,
				Template: v1.PodTemplateSpec{
					Spec: v1.PodSpec{
						// InitContainer will copy Terraform configuration files to working directory and create Terraform
						// state file directory in advance
						InitContainers: []v1.Container{{
							Name:            "prepare-input-terraform-configurations",
							Image:           "busybox",
							ImagePullPolicy: v1.PullAlways,
							Command: []string{
								"sh",
								"-c",
								fmt.Sprintf("cp %s/* %s", InputTFConfigurationVolumeMountPath, WorkingVolumeMountPath),
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
						}},
						// Container terraform-executor will first copy predefined terraform.d to working directory, and
						// then run terraform init/apply.
						Containers: []v1.Container{{
							Name:            "terraform-executor",
							Image:           TerraformImage,
							ImagePullPolicy: v1.PullAlways,
							Command: []string{
								"bash",
								"-c",
								fmt.Sprintf("cp -r /root/terraform.d %s && terraform init && terraform destroy -auto-approve",
									WorkingVolumeMountPath),
							},
							VolumeMounts: []v1.VolumeMount{
								{
									Name:      name,
									MountPath: WorkingVolumeMountPath,
								},
							},
							Env: envs,
						}},
						Volumes:       []v1.Volume{workingVolume, inputTFConfigurationVolume},
						RestartPolicy: v1.RestartPolicyOnFailure,
					},
				},
			},
		}
	}
	if err := k8sClient.Create(ctx, job); err != nil {
		return err
	}
	return nil
}

type TFState struct {
	Outputs map[string]v1beta1.Property `json:"outputs"`
}

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
	k8sBackendSecretName := fmt.Sprintf("tfstate-%s-%s", TerraformWorkspace, backendSecretSuffix)
	if err := k8sClient.Get(ctx, client.ObjectKey{Name: k8sBackendSecretName, Namespace: configuration.Namespace}, &s); err != nil {
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
					OwnerReferences: []metav1.OwnerReference{{
						APIVersion: configuration.APIVersion,
						Kind:       configuration.Kind,
						Name:       configuration.Name,
						UID:        configuration.UID,
						Controller: pointer.BoolPtr(false),
					}},
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

func prepareTFVariables(ctx context.Context, k8sClient client.Client, namespace string, configuration *v1beta1.Configuration, providerName string) ([]v1.EnvVar, error) {
	var envs []v1.EnvVar

	var tfVariables *runtime.RawExtension
	if configuration != nil {
		tfVariables = configuration.Spec.Variable
	}
	tfVariable, err := getTerraformJSONVariable(tfVariables)
	if err != nil {
		return nil, errors.Wrap(err, fmt.Sprintf("failed to get Terraform JSON variables from Configuration Variables %v", tfVariables))
	}
	for k, v := range tfVariable {
		envs = append(envs, v1.EnvVar{Name: k, Value: v})
	}

	credential, err := util.GetProviderCredentials(ctx, k8sClient, namespace, providerName)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get credentials from the cloud provider")
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

type Variable map[string]interface{}

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

func deleteConfigMap(ctx context.Context, k8sClient client.Client, namespace, name string) error {
	var cm v1.ConfigMap
	if err := k8sClient.Get(ctx, client.ObjectKey{Name: name, Namespace: namespace}, &cm); err == nil {
		if err := k8sClient.Delete(ctx, &cm); err != nil {
			return err
		}
	}
	return nil
}

func createOrUpdateConfigMap(ctx context.Context, k8sClient client.Client, namespace, name string, data map[string]string) error {
	var gotCM v1.ConfigMap
	if err := k8sClient.Get(ctx, client.ObjectKey{Name: name, Namespace: namespace}, &gotCM); err != nil {
		if kerrors.IsNotFound(err) {
			cm := v1.ConfigMap{
				TypeMeta: metav1.TypeMeta{APIVersion: "v1", Kind: "ConfigMap"},
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: namespace,
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

func prepareTFInputConfigurationData(configurationType util.ConfigurationType, inputConfiguration string) map[string]string {
	var dataName string
	switch configurationType {
	case util.ConfigurationJSON:
		dataName = TerraformJSONConfigurationName
	case util.ConfigurationHCL:
		dataName = TerraformHCLConfigurationName
	}
	data := map[string]string{dataName: inputConfiguration}
	return data
}
