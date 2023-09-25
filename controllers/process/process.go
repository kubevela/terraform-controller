package process

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"reflect"

	"github.com/oam-dev/terraform-controller/controllers/process/container"

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
	"github.com/oam-dev/terraform-controller/api/v1beta2"
	tfcfg "github.com/oam-dev/terraform-controller/controllers/configuration"
	"github.com/oam-dev/terraform-controller/controllers/configuration/backend"
	"github.com/oam-dev/terraform-controller/controllers/provider"
	"github.com/oam-dev/terraform-controller/controllers/util"
)

type Option func(spec v1beta2.Configuration, meta *TFConfigurationMeta)

// ControllerNamespaceOption will set the controller namespace for TFConfigurationMeta
func ControllerNamespaceOption(controllerNamespace string) Option {
	return func(configuration v1beta2.Configuration, meta *TFConfigurationMeta) {
		if controllerNamespace == "" {
			return
		}
		uid := string(configuration.GetUID())
		// @step: since we are using a single namespace to run these, we must ensure the names
		// are unique across the namespace
		meta.KeepLegacySubResourceMetas()
		meta.ApplyJobName = uid + "-" + string(types.TerraformApply)
		meta.DestroyJobName = uid + "-" + string(types.TerraformDestroy)
		meta.ConfigurationCMName = fmt.Sprintf(types.TFInputConfigMapName, uid)
		meta.VariableSecretName = fmt.Sprintf(types.TFVariableSecret, uid)
		meta.ControllerNamespace = controllerNamespace
		meta.ControllerNSSpecified = true
	}
}

// New will create a new TFConfigurationMeta to process the configuration
func New(req ctrl.Request, configuration v1beta2.Configuration, k8sClient client.Client, option ...Option) *TFConfigurationMeta {
	var meta = &TFConfigurationMeta{
		ControllerNamespace: req.Namespace,
		Namespace:           req.Namespace,
		Name:                req.Name,
		ConfigurationCMName: fmt.Sprintf(types.TFInputConfigMapName, req.Name),
		VariableSecretName:  fmt.Sprintf(types.TFVariableSecret, req.Name),
		ApplyJobName:        req.Name + "-" + string(types.TerraformApply),
		DestroyJobName:      req.Name + "-" + string(types.TerraformDestroy),
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

	meta.Git.URL = tfcfg.ReplaceTerraformSource(configuration.Spec.Remote, githubBlockedStr)
	if configuration.Spec.Path == "" {
		meta.Git.Path = "."
	} else {
		meta.Git.Path = configuration.Spec.Path
	}
	if configuration.Spec.DeleteResource != nil {
		meta.DeleteResource = *configuration.Spec.DeleteResource
	} else {
		meta.DeleteResource = true
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

	for _, opt := range option {
		opt(configuration, meta)
	}
	return meta
}

func (meta *TFConfigurationMeta) ValidateSecretAndConfigMap(ctx context.Context, k8sClient client.Client) error {

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
			neededKeys:    []string{types.GitCredsKnownHosts, v1.SSHAuthPrivateKey},
			errKey:        "git credentials",
		},
		{
			ref:           meta.TerraformCredentialsSecretReference,
			notFoundState: types.InvalidTerraformCredentialsSecretReference,
			isSecret:      true,
			neededKeys:    []string{types.TerraformCredentials},
			errKey:        "terraform credentials",
		},
		{
			ref:           meta.TerraformRCConfigMapReference,
			notFoundState: types.InvalidTerraformRCConfigMapReference,
			isSecret:      false,
			neededKeys:    []string{types.TerraformRegistryConfig},
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
				if updateStatusErr := meta.UpdateApplyStatus(ctx, k8sClient, check.notFoundState, msg); updateStatusErr != nil {
					return errors.Wrap(updateStatusErr, msg)
				}
				return errors.New(msg)
			}
			// fix: The configmap or secret that the pod restricts from mounting must be in the same namespace as the pod,
			//      otherwise the volume mount will fail.
			if object.GetNamespace() != meta.ControllerNamespace {
				objectKind := "ConfigMap"
				if check.isSecret {
					objectKind = "Secret"
				}
				msg := fmt.Sprintf("Invalid %s '%s/%s', whose namespace '%s' is different from the Configuration, cannot mount the volume,"+
					" you can fix this issue by creating the Secret/ConfigMap in the '%s' namespace.",
					objectKind, object.GetNamespace(), object.GetName(), meta.ControllerNamespace, meta.ControllerNamespace)
				if updateStatusErr := meta.UpdateApplyStatus(ctx, k8sClient, check.notFoundState, msg); updateStatusErr != nil {
					return errors.Wrap(updateStatusErr, msg)
				}
				return errors.New(msg)
			}
		}
	}
	return nil
}

func (meta *TFConfigurationMeta) UpdateApplyStatus(ctx context.Context, k8sClient client.Client, state types.ConfigurationState, message string) error {
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

func (meta *TFConfigurationMeta) UpdateDestroyStatus(ctx context.Context, k8sClient client.Client, state types.ConfigurationState, message string) error {
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

func (meta *TFConfigurationMeta) AssembleAndTriggerJob(ctx context.Context, k8sClient client.Client, executionType types.TerraformExecutionType) error {
	// apply rbac
	if err := createTerraformExecutorServiceAccount(ctx, k8sClient, meta.ControllerNamespace, types.ServiceAccountName); err != nil {
		return err
	}
	if err := util.CreateTerraformExecutorClusterRoleBinding(ctx, k8sClient, meta.ControllerNamespace, fmt.Sprintf("%s-%s", meta.ControllerNamespace, types.ClusterRoleName), types.ServiceAccountName); err != nil {
		return err
	}

	job := meta.assembleTerraformJob(executionType)

	return k8sClient.Create(ctx, job)
}

// UpdateTerraformJobIfNeeded will set deletion finalizer to the Terraform job if its envs are changed, which will result in
// deleting the job. Finally, a new Terraform job will be generated
func (meta *TFConfigurationMeta) UpdateTerraformJobIfNeeded(ctx context.Context, k8sClient client.Client, job batchv1.Job) error {
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

func (meta *TFConfigurationMeta) assembleTerraformJob(executionType types.TerraformExecutionType) *batchv1.Job {
	var (
		initContainers []v1.Container
		parallelism    int32 = 1
		completions    int32 = 1
	)

	executorVolumes := meta.assembleExecutorVolumes()

	assembler := container.NewAssembler(meta.Name).
		TerraformCredReference(meta.TerraformCredentialsSecretReference).
		TerraformRCReference(meta.TerraformRCConfigMapReference).
		TerraformCredentialsHelperReference(meta.TerraformCredentialsHelperConfigMapReference).
		GitCredReference(meta.GitCredentialsSecretReference).
		SetGit(meta.Git).
		SetBusyboxImage(meta.BusyboxImage).
		SetTerraformImage(meta.TerraformImage).
		SetGitImage(meta.GitImage).
		SetEnvs(meta.Envs)

	initContainers = append(initContainers, assembler.InputContainer())
	if meta.Git.URL != "" {
		initContainers = append(initContainers, assembler.GitContainer())
	}
	initContainers = append(initContainers, assembler.InitContainer())

	applyContainer := assembler.ApplyContainer(executionType, meta.ResourceQuota)

	name := meta.ApplyJobName
	if executionType == types.TerraformDestroy {
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
			BackoffLimit: &meta.BackoffLimit,
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
					Containers:         []v1.Container{applyContainer},
					ServiceAccountName: types.ServiceAccountName,
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
			volumeName: types.GitAuthConfigVolumeName,
			isSecret:   true,
		},
		{
			ref:        meta.TerraformCredentialsSecretReference,
			volumeName: types.TerraformCredentialsConfigVolumeName,
			isSecret:   true,
		},
		{
			ref:        meta.TerraformRCConfigMapReference,
			volumeName: types.TerraformRCConfigVolumeName,
			isSecret:   false,
		},
		{
			ref:        meta.TerraformCredentialsHelperConfigMapReference,
			volumeName: types.TerraformCredentialsHelperConfigVolumeName,
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
	inputTFConfigurationVolume := v1.Volume{Name: types.InputTFConfigurationVolumeName}
	inputTFConfigurationVolume.ConfigMap = &inputCMVolumeSource
	return inputTFConfigurationVolume

}

func (meta *TFConfigurationMeta) createTFBackendVolume() v1.Volume {
	gitVolume := v1.Volume{Name: types.BackendVolumeName}
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

func (meta *TFConfigurationMeta) KeepLegacySubResourceMetas() {
	meta.LegacySubResources.Namespace = meta.Namespace
	meta.LegacySubResources.ApplyJobName = meta.ApplyJobName
	meta.LegacySubResources.DestroyJobName = meta.DestroyJobName
	meta.LegacySubResources.ConfigurationCMName = meta.ConfigurationCMName
	meta.LegacySubResources.VariableSecretName = meta.VariableSecretName
}

func (meta *TFConfigurationMeta) GetApplyJob(ctx context.Context, k8sClient client.Client, job *batchv1.Job) error {
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

func (meta *TFConfigurationMeta) IsTFStateGenerated(ctx context.Context) bool {
	// 1. exist backend
	if meta.Backend == nil {
		return false
	}
	// 2. and exist tfstate file
	_, err := meta.Backend.GetTFStateJSON(ctx)
	return err == nil
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
		ns = types.DefaultNamespace
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

func (meta *TFConfigurationMeta) PrepareTFVariables(configuration *v1beta2.Configuration) error {
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

// GetCredentials will get credentials from secret of the Provider
func (meta *TFConfigurationMeta) GetCredentials(ctx context.Context, k8sClient client.Client, providerObj *v1beta1.Provider) error {
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

// StoreTFConfiguration will store Terraform configuration to ConfigMap
func (meta *TFConfigurationMeta) StoreTFConfiguration(ctx context.Context, k8sClient client.Client) error {
	data := meta.prepareTFInputConfigurationData()
	return meta.createOrUpdateConfigMap(ctx, k8sClient, data)
}

// CheckWhetherConfigurationChanges will check whether configuration is changed
func (meta *TFConfigurationMeta) CheckWhetherConfigurationChanges(ctx context.Context, k8sClient client.Client, configurationType types.ConfigurationType) error {
	switch configurationType {
	case types.ConfigurationHCL:
		var cm v1.ConfigMap
		if err := k8sClient.Get(ctx, client.ObjectKey{Name: meta.ConfigurationCMName, Namespace: meta.ControllerNamespace}, &cm); err != nil {
			if kerrors.IsNotFound(err) {
				return nil
			}
			return err
		}
		meta.ConfigurationChanged = cm.Data[types.TerraformHCLConfigurationName] != meta.CompleteConfiguration
		if meta.ConfigurationChanged {
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

func createTerraformExecutorServiceAccount(ctx context.Context, k8sClient client.Client, namespace, serviceAccountName string) error {
	var serviceAccount = v1.ServiceAccount{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "ServiceAccount",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      serviceAccountName,
			Namespace: namespace,
		},
	}
	if err := k8sClient.Get(ctx, client.ObjectKey{Name: serviceAccountName, Namespace: namespace}, &v1.ServiceAccount{}); err != nil {
		if kerrors.IsNotFound(err) {
			if err := k8sClient.Create(ctx, &serviceAccount); err != nil {
				return errors.Wrap(err, "failed to create ServiceAccount for Terraform executor")
			}
		}
	}
	return nil
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
