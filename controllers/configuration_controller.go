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
	"path/filepath"

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

	"github.com/oam-dev/terraform-controller/api/v1beta1"
	"github.com/oam-dev/terraform-controller/controllers/util"
)

const (
	// TerraformImage is the Terraform image which can run `terraform init/plan/apply`
	// hit issue `toomanyrequests` for "oamdev/docker-terraform:0.14.10"
	TerraformImage = "registry.cn-hongkong.aliyuncs.com/zzxwill/docker-terraform:0.14.10"

	TFStateRetrieverImage = "zzxwill/terraform-tfstate-retriever:v0.3"
)

const (
	WorkingVolumeMountPath              = "/data"
	InputTFConfigurationVolumeName      = "tf-input-configuration"
	InputTFConfigurationVolumeMountPath = "/opt/tfconfiguration"
	TFStateFileVolumeName               = "tf-state-file"
	TFStateFileVolumeMountPath          = "/opt/tfstate"

	SucceededPod int32 = 1
)

const (
	TerraformJSONConfigurationName = "main.tf.json"
	TerraformHCLConfigurationName  = "main.tf"
	TerraformStateName             = "terraform.tfstate"
)

type ConfigMapName string

const (
	TFInputConfigMapSName ConfigMapName = "%s-tf-input"
	TFStateConfigMapName  ConfigMapName = "%s-tf-state"
)

type TerraformExecutionType string

const (
	TerraformApply   TerraformExecutionType = "apply"
	TerraformDestroy TerraformExecutionType = "destroy"
)

// ConfigurationReconciler reconciles a Configuration object.
type ConfigurationReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=terraform.core.oam.dev,resources=configurations,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=terraform.core.oam.dev,resources=configurations/status,verbs=get;update;patch

func (r *ConfigurationReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	var (
		configuration         v1beta1.Configuration
		ctx                   = context.Background()
		ns                    = req.Namespace
		configurationName     = req.Name
		tfInputConfigMapsName = fmt.Sprintf(string(TFInputConfigMapSName), configurationName)
		tfStateConfigMapName  = fmt.Sprintf(string(TFStateConfigMapName), configurationName)
	)
	klog.InfoS("reconciling Terraform Template...", "NamespacedName", req.NamespacedName)

	// Terraform destroy
	if err := r.Get(ctx, req.NamespacedName, &configuration); err != nil {
		klog.InfoS("performing Terraform Destroy", "Namespace", req.Namespace, "Name", req.Name)
		if kerrors.IsNotFound(err) {
			destroyJobName := configurationName + "-" + string(TerraformDestroy)
			klog.InfoS("creating job", "Namespace", req.Namespace, "Name", destroyJobName)
			err := terraformDestroy(ctx, r.Client, ns, req.Name, destroyJobName, tfInputConfigMapsName, tfStateConfigMapName)
			return ctrl.Result{}, errors.Wrap(err, "continue reconciling to destroy cloud resource")
		}
	}

	// Terraform apply (create or update)
	klog.InfoS("performing Terraform Apply (cloud resource create/update)", "Namespace", req.Namespace, "Name", req.Name)
	applyJobName := configurationName + "-" + string(TerraformApply)
	klog.InfoS("creating job", "Namespace", req.Namespace, "Name", applyJobName)
	if err := terraformApply(ctx, r.Client, ns, configuration, applyJobName, tfInputConfigMapsName, tfStateConfigMapName); err != nil {
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}

func assembleAndTriggerJob(ctx context.Context, k8sClient client.Client, namespace, name string, configuration *v1beta1.Configuration, tfInputConfigMapsName string, executionType TerraformExecutionType) error {
	var job *batchv1.Job
	jobName := name + "-" + string(executionType)
	var parallelism int32 = 1
	var completions int32 = 1
	var ttlSecondsAfterFinished int32 = 0

	workingVolume := v1.Volume{Name: name}
	workingVolume.EmptyDir = &v1.EmptyDirVolumeSource{}

	tfStateConfigMapName := fmt.Sprintf(string(TFStateConfigMapName), name)
	tfStateDir := filepath.Join(WorkingVolumeMountPath, "tfstate")

	inputCMVolumeSource := v1.ConfigMapVolumeSource{}
	inputCMVolumeSource.Name = tfInputConfigMapsName
	inputTFConfigurationVolume := v1.Volume{Name: InputTFConfigurationVolumeName}
	inputTFConfigurationVolume.ConfigMap = &inputCMVolumeSource

	stateCMVolumeSource := v1.ConfigMapVolumeSource{}
	stateCMVolumeSource.Name = tfStateConfigMapName
	tfStateFileVolume := v1.Volume{Name: TFStateFileVolumeName}
	tfStateFileVolume.ConfigMap = &stateCMVolumeSource

	var tfStateFileCM v1.ConfigMap
	tfStateError := k8sClient.Get(ctx, client.ObjectKey{Name: tfStateConfigMapName, Namespace: namespace}, &tfStateFileCM)

	envs, err := prepareTFVariables(ctx, k8sClient, configuration)
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
		initContainerCMD := fmt.Sprintf("cp %s/* %s && mkdir -p %s",
			InputTFConfigurationVolumeMountPath, WorkingVolumeMountPath, tfStateDir)

		if tfStateError == nil {
			volumes = append(volumes, tfStateFileVolume)
			initContainerVolumeMounts = append(initContainerVolumeMounts, v1.VolumeMount{
				Name:      TFStateFileVolumeName,
				MountPath: TFStateFileVolumeMountPath,
			})
			initContainerCMD = fmt.Sprintf("cp %s/* %s && cp %s/* %s && mkdir -p %s", InputTFConfigurationVolumeMountPath,
				WorkingVolumeMountPath, TFStateFileVolumeMountPath, WorkingVolumeMountPath, tfStateDir)
		}
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
						// Containers has two container
						// 1) Container terraform-executor will first copy predefined terraform.d to working directory, and then
						// run terraform init/apply.
						// 2) Container terraform-tfstate-retriever will wait forever for state file until it got the file
						// and will write it to configmap for future use.
						Containers: []v1.Container{{
							Name:            "terraform-executor",
							Image:           TerraformImage,
							ImagePullPolicy: v1.PullAlways,
							Command: []string{
								"bash",
								"-c",
								fmt.Sprintf("cp -r /root/terraform.d %s && rm -f %s/ && terraform init &&"+
									" terraform apply -auto-approve && cp %s %s/",
									WorkingVolumeMountPath, filepath.Join(tfStateDir, TerraformStateName), TerraformStateName, tfStateDir),
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
							{
								Name:            "terraform-tfstate-retriever",
								Image:           TFStateRetrieverImage,
								ImagePullPolicy: v1.PullAlways,
								Env: []v1.EnvVar{
									{Name: "CONFIGMAPS_NAMESPACE", Value: namespace},
									{Name: "CONFIGMAPS_NAME", Value: tfStateConfigMapName},
									{Name: "TF_STATE_DIR", Value: tfStateDir},
									{Name: "TF_STATE_NAME", Value: TerraformStateName},
								},
								VolumeMounts: []v1.VolumeMount{
									{
										Name:      name,
										MountPath: WorkingVolumeMountPath,
									},
								},
							},
						},
						Volumes:       volumes,
						RestartPolicy: v1.RestartPolicyOnFailure,
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
								fmt.Sprintf("cp %s/* %s && cp %s/* %s", InputTFConfigurationVolumeMountPath, WorkingVolumeMountPath, TFStateFileVolumeMountPath, WorkingVolumeMountPath),
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
								{
									Name:      TFStateFileVolumeName,
									MountPath: TFStateFileVolumeMountPath,
								},
							},
						}},
						// Containers has two container
						// 1) Container terraform-executor will first copy predefined terraform.d to working directory, and then
						// run terraform init/apply.
						// 2) Container terraform-tfstate-retriever will wait forever for state file until it got the file
						// and will write it to configmap for future use.
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
						Volumes:       []v1.Volume{workingVolume, inputTFConfigurationVolume, tfStateFileVolume},
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

func getTFOutputs(ctx context.Context, k8sClient client.Client, configuration v1beta1.Configuration, tfStateConfigMapName string) (map[string]v1beta1.Property, error) {
	var configMap = v1.ConfigMap{}
	// Check the existence of ConfigMap which is used to store TF state file
	if err := k8sClient.Get(ctx, client.ObjectKey{Name: tfStateConfigMapName, Namespace: configuration.Namespace}, &configMap); err != nil {
		return nil, err
	}
	tfStateJSON, ok := configMap.Data[TerraformStateName]
	if !ok {
		return nil, fmt.Errorf("failed to get %s from ConfigMap %s", TerraformStateName, configMap.Name)
	}

	var tfState TFState
	if tfStateJSON != "" {
		if err := json.Unmarshal([]byte(tfStateJSON), &tfState); err != nil {
			return nil, err
		}
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

func prepareTFVariables(ctx context.Context, k8sClient client.Client, configuration *v1beta1.Configuration) ([]v1.EnvVar, error) {
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

	credential, err := util.GetProviderCredentials(ctx, k8sClient)
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

func createConfigMap(ctx context.Context, k8sClient client.Client, namespace, name string, data map[string]string) error {
	cm := v1.ConfigMap{
		TypeMeta: metav1.TypeMeta{APIVersion: "v1", Kind: "ConfigMap"},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Data: data,
	}
	if err := k8sClient.Create(ctx, &cm); err != nil {
		return err
	}
	return nil
}

func terraformDestroy(ctx context.Context, k8sClient client.Client, namespace, configurationName, jobName, tfInputConfigMapsName, tfStateConfigMapName string) error {
	var destroyJob batchv1.Job

	if err := k8sClient.Get(ctx, client.ObjectKey{Name: jobName, Namespace: namespace}, &destroyJob); err != nil {
		if kerrors.IsNotFound(err) {
			if err = assembleAndTriggerJob(ctx, k8sClient, namespace, configurationName, nil, tfInputConfigMapsName,
				TerraformDestroy); err != nil {
				return err
			}
		}
	}
	if destroyJob.Status.Succeeded == SucceededPod {
		// delete Terraform input Configuration ConfigMap
		if err := deleteConfigMap(ctx, k8sClient, namespace, tfInputConfigMapsName); err != nil {
			return err
		}

		// delete Terraform state file ConfigMap
		if err := deleteConfigMap(ctx, k8sClient, namespace, tfStateConfigMapName); err != nil {
			return err
		}

		// delete Job itself
		if err := k8sClient.Delete(ctx, &destroyJob); err != nil {
			return err
		}
		return nil
	}
	return errors.New("configuration deletion isn't completed.")
}

func terraformApply(ctx context.Context, k8sClient client.Client, namespace string, configuration v1beta1.Configuration, applyJobName, tfInputConfigMapsName, tfStateConfigMapName string) error {
	var gotJob batchv1.Job

	err := k8sClient.Get(ctx, client.ObjectKey{Name: applyJobName, Namespace: namespace}, &gotJob)
	if err == nil {
		if gotJob.Status.Succeeded == SucceededPod {
			outputs, err := getTFOutputs(ctx, k8sClient, configuration, tfStateConfigMapName)
			if err != nil {
				return err
			}
			configuration.Status.State = "provisioned"
			configuration.Status.Outputs = outputs
			if err := k8sClient.Update(ctx, &configuration); err != nil {
				return err
			}
			return nil
		}
	}

	if kerrors.IsNotFound(err) {
		configurationType, inputConfiguration, err := util.ValidConfiguration(configuration)
		if err != nil {
			return err
		}

		// Check the existence of ConfigMap which is used to input TF configuration file
		// TODO(zzxwill) replace the configmap every time?
		var configMap v1.ConfigMap
		if err := k8sClient.Get(ctx, client.ObjectKey{Name: tfInputConfigMapsName, Namespace: namespace}, &configMap); err != nil {
			if kerrors.IsNotFound(err) {
				var dataName string
				switch configurationType {
				case util.ConfigurationJSON:
					dataName = TerraformJSONConfigurationName
				case util.ConfigurationHCL:
					dataName = TerraformHCLConfigurationName
				}
				data := map[string]string{dataName: inputConfiguration}
				if err = createConfigMap(ctx, k8sClient, namespace, tfInputConfigMapsName, data); err != nil {
					return err
				}
			} else {
				return err
			}
		}

		if err := assembleAndTriggerJob(ctx, k8sClient, namespace, configuration.Name, &configuration,
			tfInputConfigMapsName, TerraformApply); err != nil {
			return err
		}
	}
	return nil
}
