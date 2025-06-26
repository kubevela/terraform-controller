package process

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/agiledragon/gomonkey/v2"
	"github.com/oam-dev/terraform-controller/api/types"
	crossplane "github.com/oam-dev/terraform-controller/api/types/crossplane-runtime"
	"github.com/oam-dev/terraform-controller/api/v1beta1"
	"github.com/oam-dev/terraform-controller/api/v1beta2"
	"github.com/oam-dev/terraform-controller/controllers/configuration/backend"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

var k8sClient client.Client

func TestInitTFConfigurationMeta(t *testing.T) {
	req := ctrl.Request{}
	const (
		defaultNamespace = "default"
		exampleName      = "abc"
	)
	req.Namespace = defaultNamespace
	req.Name = exampleName

	completeConfiguration := v1beta2.Configuration{
		ObjectMeta: v1.ObjectMeta{
			Name: "abc",
		},
		Spec: v1beta2.ConfigurationSpec{
			Path: "alibaba/rds",
			Backend: &v1beta2.Backend{
				SecretSuffix: "s1",
			},
		},
	}
	completeConfiguration.Spec.ProviderReference = &crossplane.Reference{
		Name:      "xxx",
		Namespace: "default",
	}
	completeConfiguration.Spec.InlineCredentials = false

	testcases := []struct {
		name          string
		configuration v1beta2.Configuration
		want          *TFConfigurationMeta
	}{
		{
			name: "empty configuration",
			configuration: v1beta2.Configuration{
				ObjectMeta: v1.ObjectMeta{
					Name: "abc",
				},
			},
			want: &TFConfigurationMeta{
				Namespace:           "default",
				ControllerNamespace: "default",
				Name:                "abc",
				ConfigurationCMName: "tf-abc",
				VariableSecretName:  "variable-abc",
				ApplyJobName:        "abc-apply",
				DestroyJobName:      "abc-destroy",

				Git: types.Git{
					Path: ".",
				},
				ProviderReference: &crossplane.Reference{
					Name:      "default",
					Namespace: "default",
				},
			},
		},
		{
			name:          "complete configuration",
			configuration: completeConfiguration,
			want: &TFConfigurationMeta{
				Namespace:           "default",
				ControllerNamespace: "default",
				Name:                "abc",
				ConfigurationCMName: "tf-abc",
				VariableSecretName:  "variable-abc",
				ApplyJobName:        "abc-apply",
				DestroyJobName:      "abc-destroy",

				Git: types.Git{
					Path: "alibaba/rds",
				},
				ProviderReference: &crossplane.Reference{
					Name:      "xxx",
					Namespace: "default",
				},
			},
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			meta := New(req, tc.configuration, nil)
			if !reflect.DeepEqual(meta.Name, tc.want.Name) {
				t.Errorf("initTFConfigurationMeta = %v, want %v", meta, tc.want)
			}
		})
	}
}

func TestInitTFConfigurationMetaWithDeleteResource(t *testing.T) {
	req := ctrl.Request{}
	req.Namespace = "default"
	req.Name = "abc"
	testcases := []struct {
		name          string
		configuration v1beta2.Configuration
		meta          *TFConfigurationMeta
	}{
		{
			name: "DeleteResource is true",
			configuration: v1beta2.Configuration{
				ObjectMeta: v1.ObjectMeta{
					Name: "abc",
				},
				Spec: v1beta2.ConfigurationSpec{
					DeleteResource: ptr.To(true),
				},
			},
			meta: &TFConfigurationMeta{
				DeleteResource: true,
			},
		},
		{
			name: "DeleteResource is false",
			configuration: v1beta2.Configuration{
				ObjectMeta: v1.ObjectMeta{
					Name: "abc",
				},
				Spec: v1beta2.ConfigurationSpec{
					DeleteResource: ptr.To(false),
				},
			},
			meta: &TFConfigurationMeta{
				DeleteResource: false,
			},
		},
		{
			name: "DeleteResource is nil",
			configuration: v1beta2.Configuration{
				ObjectMeta: v1.ObjectMeta{
					Name: "abc",
				},
				Spec: v1beta2.ConfigurationSpec{
					DeleteResource: nil,
				},
			},
			meta: &TFConfigurationMeta{
				DeleteResource: true,
			},
		},
	}
	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			meta := New(req, tc.configuration, nil)
			if !reflect.DeepEqual(meta.DeleteResource, tc.meta.DeleteResource) {
				t.Errorf("initTFConfigurationMeta = %v, want %v", meta, tc.meta)
			}

		})
	}
}

func TestInitTFConfigurationMetaWithJobNodeSelector(t *testing.T) {
	req := ctrl.Request{}
	req.Namespace = "default"
	req.Name = "abc"
	err := os.Setenv("JOB_NODE_SELECTOR", "{\"ssd\": \"true\"}")
	assert.Nil(t, err)
	configuration := v1beta2.Configuration{
		ObjectMeta: v1.ObjectMeta{
			Name: "abc",
		},
		Spec: v1beta2.ConfigurationSpec{},
	}
	meta := New(req, configuration, nil)
	assert.Equal(t, meta.JobNodeSelector, map[string]string{"ssd": "true"})
}

func TestCheckValidateSecretAndConfigMap(t *testing.T) {
	ctx := context.Background()
	scheme := runtime.NewScheme()
	assert.Nil(t, corev1.AddToScheme(scheme))
	k8sClient := fake.NewClientBuilder().WithScheme(scheme).Build()

	privateKey := []byte("aaa")
	knownHosts := []byte("zzz")
	secretGitCreds := &corev1.Secret{
		ObjectMeta: v1.ObjectMeta{
			Namespace: "default",
			Name:      "git-ssh",
		},
		Data: map[string][]byte{
			corev1.SSHAuthPrivateKey: privateKey,
			"known_hosts":            knownHosts,
		},
		Type: corev1.SecretTypeSSHAuth,
	}
	assert.Nil(t, k8sClient.Create(ctx, secretGitCreds))
	assert.Nil(t, k8sClient.Get(ctx, client.ObjectKeyFromObject(secretGitCreds), secretGitCreds))

	credentialstfrcjson := []byte("tfcreds")
	terraformrc := []byte("tfrc")
	secretTerraformCredentials := &corev1.Secret{
		ObjectMeta: v1.ObjectMeta{
			Namespace: "default",
			Name:      "terraform-creds",
		},
		Data: map[string][]byte{
			"credentials.tfrc.json": credentialstfrcjson,
			"terraformrc":           terraformrc,
		},
		Type: corev1.SecretTypeSSHAuth,
	}
	assert.Nil(t, k8sClient.Create(ctx, secretTerraformCredentials))
	assert.Nil(t, k8sClient.Get(ctx, client.ObjectKeyFromObject(secretTerraformCredentials), secretTerraformCredentials))

	configMapTerraformRC := &corev1.ConfigMap{
		ObjectMeta: v1.ObjectMeta{
			Namespace: "default",
			Name:      "terraform-registry-config",
		},
		Data: map[string]string{
			".terraformrc": "tfrc",
		},
	}

	assert.Nil(t, k8sClient.Create(ctx, configMapTerraformRC))
	assert.Nil(t, k8sClient.Get(ctx, client.ObjectKeyFromObject(configMapTerraformRC), configMapTerraformRC))

	configMapCredentialsHelper := &corev1.ConfigMap{
		ObjectMeta: v1.ObjectMeta{
			Namespace: "default",
			Name:      "terraform-credentials-helper",
		},
		Data: map[string]string{
			"terraform-credentials-artifactory": "tfrc",
		},
	}

	assert.Nil(t, k8sClient.Create(ctx, configMapCredentialsHelper))
	assert.Nil(t, k8sClient.Get(ctx, client.ObjectKeyFromObject(configMapCredentialsHelper), configMapCredentialsHelper))

	type args struct {
		k8sClient client.Client
		meta      TFConfigurationMeta
	}

	type want struct {
		configMap *corev1.ConfigMap
		errMsg    string
	}

	testcases := []struct {
		name string
		args args
		want want
	}{
		{
			name: "configmap not found",
			args: args{
				k8sClient: k8sClient,
				meta: TFConfigurationMeta{
					Name:                "a",
					ConfigurationCMName: "b",
					BusyboxImage:        "c",
					GitImage:            "d",
					Namespace:           "e",
					TerraformImage:      "f",
					ControllerNamespace: "default",
					GitCredentialsSecretReference: &corev1.SecretReference{
						Namespace: "default",
						Name:      "git-ssh",
					},
					TerraformCredentialsSecretReference: &corev1.SecretReference{
						Namespace: "default",
						Name:      "terraform-creds",
					},
					TerraformRCConfigMapReference: &corev1.SecretReference{
						Namespace: "default",
						Name:      "terraform-registry-config",
					},
					TerraformCredentialsHelperConfigMapReference: &corev1.SecretReference{
						Namespace: "default",
						Name:      "terraform-credentials-helper",
					},
				},
			},
			want: want{
				errMsg: "NoError",
			},
		},
		{
			name: "terraform credentials configmap not found",
			args: args{
				k8sClient: k8sClient,
				meta: TFConfigurationMeta{
					Name:                "a",
					ConfigurationCMName: "b",
					BusyboxImage:        "c",
					GitImage:            "d",
					Namespace:           "e",
					TerraformImage:      "f",
					ControllerNamespace: "default",
					GitCredentialsSecretReference: &corev1.SecretReference{
						Namespace: "default",
						Name:      "git-ssh",
					},
					TerraformCredentialsSecretReference: &corev1.SecretReference{
						Namespace: "default",
						Name:      "terraform-creds",
					},
					TerraformRCConfigMapReference: &corev1.SecretReference{
						Namespace: "default",
						Name:      "terraform-registry-config",
					},
					TerraformCredentialsHelperConfigMapReference: &corev1.SecretReference{
						Namespace: "default",
						Name:      "terraform-registry",
					},
				},
			},
			want: want{
				errMsg: "Failed to get terraform credentials helper configmap: configmaps \"terraform-registry\" not found",
			},
		},
		{
			name: "terraformrc configmap not found",
			args: args{
				k8sClient: k8sClient,
				meta: TFConfigurationMeta{
					Name:                "a",
					ConfigurationCMName: "b",
					BusyboxImage:        "c",
					GitImage:            "d",
					Namespace:           "e",
					TerraformImage:      "f",
					ControllerNamespace: "default",
					GitCredentialsSecretReference: &corev1.SecretReference{
						Namespace: "default",
						Name:      "git-ssh",
					},
					TerraformCredentialsSecretReference: &corev1.SecretReference{
						Namespace: "default",
						Name:      "terraform-creds",
					},
					TerraformRCConfigMapReference: &corev1.SecretReference{
						Namespace: "default",
						Name:      "terraform-registry",
					},
					TerraformCredentialsHelperConfigMapReference: &corev1.SecretReference{
						Namespace: "default",
						Name:      "terraform-credentials-helper",
					},
				},
			},
			want: want{
				errMsg: "Failed to get terraformrc configuration configmap: configmaps \"terraform-registry\" not found",
			},
		},
		{
			name: "git-ssh secret invalid namespace",
			args: args{
				k8sClient: k8sClient,
				meta: TFConfigurationMeta{
					Name:                "a",
					ConfigurationCMName: "b",
					BusyboxImage:        "c",
					GitImage:            "d",
					Namespace:           "e",
					TerraformImage:      "f",
					ControllerNamespace: "vela-system",
					GitCredentialsSecretReference: &corev1.SecretReference{
						Namespace: "default",
						Name:      "git-ssh",
					},
					TerraformCredentialsSecretReference: &corev1.SecretReference{
						Namespace: "default",
						Name:      "terraform-creds",
					},
					TerraformRCConfigMapReference: &corev1.SecretReference{
						Namespace: "default",
						Name:      "terraform-registry-config",
					},
					TerraformCredentialsHelperConfigMapReference: &corev1.SecretReference{
						Namespace: "default",
						Name:      "terraform-credentials-helper",
					},
				},
			},
			want: want{
				errMsg: "Invalid Secret 'default/git-ssh', whose namespace 'vela-system' is different from the Configuration, cannot mount the volume," +
					" you can fix this issue by creating the Secret/ConfigMap in the 'vela-system' namespace.",
			},
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.args.meta.ValidateSecretAndConfigMap(ctx, k8sClient)
			if err != nil {
				assert.EqualError(t, err, tc.want.errMsg)
			}
		})
	}

}

func TestAssembleTerraformJob(t *testing.T) {
	meta := &TFConfigurationMeta{
		Name:                "a",
		ConfigurationCMName: "b",
		BusyboxImage:        "c",
		GitImage:            "d",
		Namespace:           "e",
		TerraformImage:      "f",
		Git: types.Git{
			URL: "g",
		},
	}
	job := meta.assembleTerraformJob(types.TerraformApply)
	containers := job.Spec.Template.Spec.InitContainers
	assert.Equal(t, containers[0].Image, "c")
	assert.Equal(t, containers[1].Image, "d")
}

func TestAssembleTerraformJobWithNodeSelectorSetting(t *testing.T) {
	meta := &TFConfigurationMeta{
		Name:                "a",
		ConfigurationCMName: "b",
		BusyboxImage:        "c",
		GitImage:            "d",
		Namespace:           "e",
		TerraformImage:      "f",
		Git: types.Git{
			URL: "g",
		},
		JobNodeSelector: map[string]string{"ssd": "true"},
	}

	job := meta.assembleTerraformJob(types.TerraformApply)
	spec := job.Spec.Template.Spec
	assert.Equal(t, spec.NodeSelector, map[string]string{"ssd": "true"})
}

func TestPrepareTFVariables(t *testing.T) {
	prjID := "PrjID"
	prjIDValue := "test123"
	testKey := "Test"
	testValue := "abc123"
	credentialKey := "key"
	credentialValue := "testkey"
	variable, _ := json.Marshal(map[string]interface{}{
		testKey: testValue,
	})
	configuration := v1beta2.Configuration{
		ObjectMeta: v1.ObjectMeta{
			Namespace: "default",
			Name:      "abc",
		},
		Spec: v1beta2.ConfigurationSpec{
			ProviderReference: &crossplane.Reference{
				Name:      "default",
				Namespace: "default",
			},
			HCL: "test",
			Variable: &runtime.RawExtension{
				Raw: variable,
			},
		},
	}
	meta := &TFConfigurationMeta{
		JobEnv: map[string]interface{}{
			prjID: prjIDValue,
		},
		ProviderReference: &crossplane.Reference{
			Name:      "default",
			Namespace: "default",
		},
		Credentials: map[string]string{
			credentialKey: credentialValue,
		},
	}
	err := meta.PrepareTFVariables(&configuration)
	assert.Nil(t, err)
	rTestKey := fmt.Sprintf("TF_VAR_%s", testKey)
	wantVarSecretData := map[string]string{prjID: prjIDValue, rTestKey: testValue, credentialKey: credentialValue}
	for k, v := range wantVarSecretData {
		actualV, ok := meta.VariableSecretData[k]
		assert.Equal(t, ok, true)
		assert.Equal(t, actualV, []byte(v))
	}
	existMap := map[string]bool{}
	for _, e := range meta.Envs {
		switch e.Name {
		case prjID:
			existMap[prjID] = true
		case rTestKey:
			existMap[rTestKey] = true
		case credentialKey:
			existMap[credentialKey] = true
		default:
			t.Fatalf("unexpected %s", e.Name)
		}
	}
	assert.Equal(t, len(existMap), 3)
	for _, v := range existMap {
		assert.Equal(t, v, true)
	}
}

func TestCheckProvider(t *testing.T) {
	ctx := context.Background()
	scheme := runtime.NewScheme()
	if err := v1beta2.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}

	k8sClient1 := fake.NewClientBuilder().WithScheme(scheme).Build()

	meta := &TFConfigurationMeta{
		ProviderReference: &crossplane.Reference{
			Name:      "default",
			Namespace: "default",
		},
	}

	type args struct {
		k8sClient client.Client
		provider  *v1beta1.Provider
	}

	testcases := []struct {
		name string
		args args
		want string
	}{
		{
			name: "provider exists, and is not ready",
			args: args{
				k8sClient: k8sClient1,
				provider: &v1beta1.Provider{
					ObjectMeta: v1.ObjectMeta{
						Name:      "default",
						Namespace: "default",
					},
					Status: v1beta1.ProviderStatus{
						State: types.ProviderIsNotReady,
					},
				},
			},
		},
		{
			name: "provider doesn't not exist",
			args: args{
				k8sClient: fake.NewClientBuilder().WithScheme(scheme).Build(),
			},
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			if err := meta.GetCredentials(ctx, tc.args.k8sClient, tc.args.provider); tc.want != "" &&
				!strings.Contains(err.Error(), tc.want) {
				t.Errorf("getCredentials = %v, want %v", err.Error(), tc.want)
			}
		})
	}
}

func TestAssembleTerraformJobWithResourcesSetting(t *testing.T) {
	quantityLimitsCPU, _ := resource.ParseQuantity("10m")
	quantityLimitsMemory, _ := resource.ParseQuantity("10Mi")
	quantityRequestsCPU, _ := resource.ParseQuantity("100m")
	quantityRequestsMemory, _ := resource.ParseQuantity("5Gi")
	meta := &TFConfigurationMeta{
		Name:                "a",
		ConfigurationCMName: "b",
		BusyboxImage:        "c",
		GitImage:            "d",
		Namespace:           "e",
		TerraformImage:      "f",
		Git: types.Git{
			URL: "g",
		},

		ResourceQuota: types.ResourceQuota{
			ResourcesLimitsCPU:              "10m",
			ResourcesLimitsCPUQuantity:      quantityLimitsCPU,
			ResourcesLimitsMemory:           "10Mi",
			ResourcesLimitsMemoryQuantity:   quantityLimitsMemory,
			ResourcesRequestsCPU:            "100m",
			ResourcesRequestsCPUQuantity:    quantityRequestsCPU,
			ResourcesRequestsMemory:         "5Gi",
			ResourcesRequestsMemoryQuantity: quantityRequestsMemory,
		},
	}

	job := meta.assembleTerraformJob(types.TerraformApply)
	initContainers := job.Spec.Template.Spec.InitContainers
	assert.Equal(t, initContainers[0].Image, "c")
	assert.Equal(t, initContainers[1].Image, "d")

	container := job.Spec.Template.Spec.Containers[0]
	limitsCPU := container.Resources.Limits["cpu"]
	limitsMemory := container.Resources.Limits["memory"]
	requestsCPU := container.Resources.Requests["cpu"]
	requestsMemory := container.Resources.Requests["memory"]
	assert.Equal(t, "10m", limitsCPU.String())
	assert.Equal(t, "10Mi", limitsMemory.String())
	assert.Equal(t, "100m", requestsCPU.String())
	assert.Equal(t, "5Gi", requestsMemory.String())
}

func TestAssembleTerraformJobWithGitCredentialsSecretRef(t *testing.T) {
	meta := &TFConfigurationMeta{
		Name:                "a",
		ConfigurationCMName: "b",
		BusyboxImage:        "c",
		GitImage:            "d",
		Namespace:           "e",
		TerraformImage:      "f",
		Git: types.Git{
			URL: "g",
		},
		GitCredentialsSecretReference: &corev1.SecretReference{
			Namespace: "default",
			Name:      "git-ssh",
		},
	}

	job := meta.assembleTerraformJob(types.TerraformApply)
	spec := job.Spec.Template.Spec

	var gitSecretDefaultMode int32 = 0400
	gitAuthSecretVolume := corev1.Volume{Name: types.GitAuthConfigVolumeName}
	gitAuthSecretVolume.Secret = &corev1.SecretVolumeSource{
		SecretName:  "git-ssh",
		DefaultMode: &gitSecretDefaultMode,
	}

	gitSecretVolumeMount := corev1.VolumeMount{
		Name:      types.GitAuthConfigVolumeName,
		MountPath: types.GitAuthConfigVolumeMountPath,
	}
	assert.Contains(t, spec.InitContainers[1].VolumeMounts, gitSecretVolumeMount)
	assert.Contains(t, spec.Volumes, gitAuthSecretVolume)
}

func TestAssembleTerraformJobWithTerraformRCAndCredentials(t *testing.T) {
	meta := &TFConfigurationMeta{
		Name:                "a",
		ConfigurationCMName: "b",
		BusyboxImage:        "c",
		GitImage:            "d",
		Namespace:           "e",
		TerraformImage:      "f",
		Git: types.Git{
			URL: "g",
		},
		TerraformRCConfigMapReference: &corev1.SecretReference{
			Namespace: "default",
			Name:      "terraform-registry-config",
		},
		TerraformCredentialsSecretReference: &corev1.SecretReference{
			Namespace: "default",
			Name:      "terraform-credentials",
		},
		TerraformCredentialsHelperConfigMapReference: &corev1.SecretReference{
			Namespace: "default",
			Name:      "terraform-credentials-helper",
		},
	}

	job := meta.assembleTerraformJob(types.TerraformApply)
	spec := job.Spec.Template.Spec

	var terraformSecretDefaultMode int32 = 0400

	terraformRegistryConfigMapVolume := corev1.Volume{Name: types.TerraformRCConfigVolumeName}
	terraformRegistryConfigMapVolume.ConfigMap = &corev1.ConfigMapVolumeSource{
		LocalObjectReference: corev1.LocalObjectReference{
			Name: "terraform-registry-config",
		},
		DefaultMode: &terraformSecretDefaultMode,
	}

	terraformRegistryConfigVolumeMount := corev1.VolumeMount{
		Name:      types.TerraformRCConfigVolumeName,
		MountPath: types.TerraformRCConfigVolumeMountPath,
	}
	terraformCredentialsSecretVolume := corev1.Volume{Name: types.TerraformCredentialsConfigVolumeName}
	terraformCredentialsSecretVolume.Secret = &corev1.SecretVolumeSource{
		SecretName:  "terraform-credentials",
		DefaultMode: &terraformSecretDefaultMode,
	}

	terraformCredentialsSecretVolumeMount := corev1.VolumeMount{
		Name:      types.TerraformCredentialsConfigVolumeName,
		MountPath: types.TerraformCredentialsConfigVolumeMountPath,
	}

	terraformCredentialsHelperConfigVolume := corev1.Volume{Name: types.TerraformCredentialsHelperConfigVolumeName}
	terraformCredentialsHelperConfigVolume.ConfigMap = &corev1.ConfigMapVolumeSource{
		LocalObjectReference: corev1.LocalObjectReference{
			Name: "terraform-credentials-helper",
		},
		DefaultMode: &terraformSecretDefaultMode,
	}

	terraformCredentialsHelperConfigVolumeMount := corev1.VolumeMount{
		Name:      types.TerraformCredentialsHelperConfigVolumeName,
		MountPath: types.TerraformCredentialsHelperConfigVolumeMountPath,
	}

	terraformInitContainerMounts := spec.InitContainers[2].VolumeMounts

	assert.Contains(t, terraformInitContainerMounts, terraformCredentialsHelperConfigVolumeMount)
	assert.Contains(t, spec.Volumes, terraformCredentialsHelperConfigVolume)

	assert.Contains(t, terraformInitContainerMounts, terraformRegistryConfigVolumeMount)
	assert.Contains(t, spec.Volumes, terraformRegistryConfigMapVolume)

	assert.Contains(t, terraformInitContainerMounts, terraformCredentialsSecretVolumeMount)
	assert.Contains(t, spec.Volumes, terraformCredentialsSecretVolume)

	assert.Contains(t, terraformInitContainerMounts, terraformRegistryConfigVolumeMount)
	assert.Contains(t, spec.Volumes, terraformRegistryConfigMapVolume)
}

func TestAssembleTerraformJobWithTerraformRCAndCredentialsHelper(t *testing.T) {
	meta := &TFConfigurationMeta{
		Name:                "a",
		ConfigurationCMName: "b",
		BusyboxImage:        "c",
		GitImage:            "d",
		Namespace:           "e",
		TerraformImage:      "f",
		Git: types.Git{
			URL: "g",
		},
		TerraformRCConfigMapReference: &corev1.SecretReference{
			Namespace: "default",
			Name:      "terraform-registry-config",
		},
		TerraformCredentialsHelperConfigMapReference: &corev1.SecretReference{
			Namespace: "default",
			Name:      "terraform-credentials-helper",
		},
	}

	job := meta.assembleTerraformJob(types.TerraformApply)
	spec := job.Spec.Template.Spec

	var terraformSecretDefaultMode int32 = 0400

	terraformRegistryConfigMapVolume := corev1.Volume{Name: types.TerraformRCConfigVolumeName}
	terraformRegistryConfigMapVolume.ConfigMap = &corev1.ConfigMapVolumeSource{
		LocalObjectReference: corev1.LocalObjectReference{
			Name: "terraform-registry-config",
		},
		DefaultMode: &terraformSecretDefaultMode,
	}

	terraformRegistryConfigVolumeMount := corev1.VolumeMount{
		Name:      types.TerraformRCConfigVolumeName,
		MountPath: types.TerraformRCConfigVolumeMountPath,
	}
	terraformCredentialsHelperConfigVolume := corev1.Volume{Name: types.TerraformCredentialsHelperConfigVolumeName}
	terraformCredentialsHelperConfigVolume.ConfigMap = &corev1.ConfigMapVolumeSource{
		LocalObjectReference: corev1.LocalObjectReference{
			Name: "terraform-credentials-helper",
		},
		DefaultMode: &terraformSecretDefaultMode,
	}

	terraformCredentialsHelperConfigVolumeMount := corev1.VolumeMount{
		Name:      types.TerraformCredentialsHelperConfigVolumeName,
		MountPath: types.TerraformCredentialsHelperConfigVolumeMountPath,
	}

	terraformInitContainerMounts := spec.InitContainers[2].VolumeMounts
	assert.Contains(t, terraformInitContainerMounts, terraformRegistryConfigVolumeMount)
	assert.Contains(t, spec.Volumes, terraformRegistryConfigMapVolume)

	assert.Contains(t, terraformInitContainerMounts, terraformCredentialsHelperConfigVolumeMount)
	assert.Contains(t, spec.Volumes, terraformCredentialsHelperConfigVolume)

}

func TestTfStatePropertyToToProperty(t *testing.T) {
	testcases := []TfStateProperty{
		{
			Value: 123,
			Type:  "integer",
		},
		{
			Value: 123.1,
			Type:  "float64",
		},
		{
			Value: "123",
			Type:  "string",
		},
		{
			Value: true,
			Type:  "bool",
		},
		{
			Value: []interface{}{"21", "aaa", 12, true},
			Type:  "slice",
		},
		{
			Value: map[string]interface{}{
				"test1": "abc",
				"test2": 123,
			},
			Type: "map",
		},
	}
	for _, testcase := range testcases {
		property, err := testcase.ToProperty()
		assert.Equal(t, err, nil)
		if testcase.Type == "integer" {
			assert.Equal(t, property.Value, "123")
		}
		if testcase.Type == "float64" {
			assert.Equal(t, property.Value, "123.1")
		}
		if testcase.Type == "bool" {
			assert.Equal(t, property.Value, "true")
		}
		if testcase.Type == "slice" {
			assert.Equal(t, property.Value, `["21","aaa",12,true]`)
		}
		if testcase.Type == "map" {
			assert.Equal(t, property.Value, `{"test1":"abc","test2":123}`)
		}
	}
}

func TestIsTFStateGenerated(t *testing.T) {
	type args struct {
		ctx       context.Context
		k8sClient client.Client
		meta      *TFConfigurationMeta
	}
	type want struct {
		generated bool
	}

	ctx := context.Background()

	k8sClient1 := fake.NewClientBuilder().Build()
	meta1 := &TFConfigurationMeta{
		Backend: &backend.K8SBackend{
			Client:       k8sClient1,
			SecretSuffix: "a",
			SecretNS:     "default",
		},
	}

	secret2 := &corev1.Secret{
		ObjectMeta: v1.ObjectMeta{
			Name:      "tfstate-default-a",
			Namespace: "default",
		},
		Type: corev1.SecretTypeOpaque,
	}
	k8sClient2 := fake.NewClientBuilder().WithObjects(secret2).Build()
	meta2 := &TFConfigurationMeta{
		Backend: &backend.K8SBackend{
			Client:       k8sClient2,
			SecretSuffix: "a",
			SecretNS:     "default",
		},
	}

	tfStateData3, _ := base64.StdEncoding.DecodeString("H4sIAAAAAAAA/0SMwa7CIBBF9/0KMutH80ArDb9ijKHDYEhqMQO4afrvBly4POfc3H0QAt7EOaYNrDj/NS7E7ELi5/1XQI3/o4beM3F0K1ihO65xI/egNsLThLPRWi6agkR/CVIppaSZJrfgbBx6//1ItbxqyWDFfnTBlFNlpKaut+EYPgEAAP//xUXpvZsAAAA=")
	secret3 := &corev1.Secret{
		ObjectMeta: v1.ObjectMeta{
			Name:      "tfstate-default-a",
			Namespace: "default",
		},
		Data: map[string][]byte{
			"tfstate": tfStateData3,
		},
		Type: corev1.SecretTypeOpaque,
	}
	k8sClient3 := fake.NewClientBuilder().WithObjects(secret3).Build()
	meta3 := &TFConfigurationMeta{
		Backend: &backend.K8SBackend{
			Client:       k8sClient3,
			SecretSuffix: "a",
			SecretNS:     "default",
		},
	}

	tfStateData4, _ := base64.StdEncoding.DecodeString("H4sIAAAAAAAA/4SQzarbMBCF934KoXUdPKNf+1VKCWNp5AocO8hyaSl592KlcBd3cZfnHPHpY/52QshfXI68b3IS+tuVK5dCaS+P+8ci4TbcULb94JJplZPAFte8MS18PQrKBO8Q+xk59SHa1AMA9M4YmoN3FGJ8M/azPs96yElcCkLIsG+V8sblnqOc3uXlRuvZ0GxSSuiCRUYbw2gGHRFGPxitEgJYQDQ0a68I2ChNo1cAZJ2bR20UtW8bsv55NuJRS94W2erXe5X5QQs3A/FZ4fhJaOwUgZTVMRjto1HGpSGSQuuD955hdDDPcR6NY1ZpQJ/YwagTRAvBpsi8LXn7Pa1U+ahfWHX/zWThYz9L4Otg3390r+5fAAAA//8hmcuNuQEAAA==")
	secret4 := &corev1.Secret{
		ObjectMeta: v1.ObjectMeta{
			Name:      "tfstate-default-a",
			Namespace: "default",
		},
		Data: map[string][]byte{
			"tfstate": tfStateData4,
		},
		Type: corev1.SecretTypeOpaque,
	}
	k8sClient4 := fake.NewClientBuilder().WithObjects(secret4).Build()
	meta4 := &TFConfigurationMeta{
		Backend: &backend.K8SBackend{
			Client:       k8sClient4,
			SecretSuffix: "a",
			SecretNS:     "default",
		},
	}

	testcases := map[string]struct {
		args args
		want want
	}{
		"could not find backend secret": {
			args: args{
				ctx:       ctx,
				k8sClient: k8sClient1,
				meta:      meta1,
			},
			want: want{
				generated: false,
			},
		},
		"no data in a backend secret": {
			args: args{
				ctx:       ctx,
				k8sClient: k8sClient2,
				meta:      meta2,
			},
			want: want{
				generated: false,
			},
		},
		"outputs in the backend secret are empty.": {
			args: args{
				ctx:       ctx,
				k8sClient: k8sClient3,
				meta:      meta3,
			},
			want: want{
				generated: true,
			},
		},
		"outputs in the backend secret are not empty": {
			args: args{
				ctx:       ctx,
				k8sClient: k8sClient4,
				meta:      meta4,
			},
			want: want{
				generated: true,
			},
		},
	}

	for name, tc := range testcases {
		t.Run(name, func(t *testing.T) {
			generated := tc.args.meta.IsTFStateGenerated(tc.args.ctx)
			assert.Equal(t, tc.want.generated, generated)
		})
	}
}

func TestGetTFOutputs(t *testing.T) {
	type args struct {
		ctx           context.Context
		k8sClient     client.Client
		configuration v1beta2.Configuration
		meta          *TFConfigurationMeta
	}
	type want struct {
		property map[string]v1beta2.Property
		errMsg   string
	}

	ctx := context.Background()
	k8sClient1 := fake.NewClientBuilder().Build()
	meta1 := &TFConfigurationMeta{
		Backend: &backend.K8SBackend{
			Client:       k8sClient1,
			SecretSuffix: "a",
			SecretNS:     "default",
		},
	}

	secret2 := &corev1.Secret{
		ObjectMeta: v1.ObjectMeta{
			Name:      "tfstate-default-a",
			Namespace: "default",
		},
		Type: corev1.SecretTypeOpaque,
	}
	k8sClient2 := fake.NewClientBuilder().WithObjects(secret2).Build()
	meta2 := &TFConfigurationMeta{
		Backend: &backend.K8SBackend{
			Client:       k8sClient2,
			SecretSuffix: "a",
			SecretNS:     "default",
		},
	}

	tfStateData, _ := base64.StdEncoding.DecodeString("H4sIAAAAAAAA/4SQzarbMBCF934KoXUdPKNf+1VKCWNp5AocO8hyaSl592KlcBd3cZfnHPHpY/52QshfXI68b3IS+tuVK5dCaS+P+8ci4TbcULb94JJplZPAFte8MS18PQrKBO8Q+xk59SHa1AMA9M4YmoN3FGJ8M/azPs96yElcCkLIsG+V8sblnqOc3uXlRuvZ0GxSSuiCRUYbw2gGHRFGPxitEgJYQDQ0a68I2ChNo1cAZJ2bR20UtW8bsv55NuJRS94W2erXe5X5QQs3A/FZ4fhJaOwUgZTVMRjto1HGpSGSQuuD955hdDDPcR6NY1ZpQJ/YwagTRAvBpsi8LXn7Pa1U+ahfWHX/zWThYz9L4Otg3390r+5fAAAA//8hmcuNuQEAAA==")
	tfStateOutputs := map[string]v1beta2.Property{
		"container_id": {
			Value: "e5fff27c62e26dc9504d21980543f21161225ab483a1e534a98311a677b9453a",
		},
		"image_id": {
			Value: "sha256:d1a364dc548d5357f0da3268c888e1971bbdb957ee3f028fe7194f1d61c6fdeenginx:latest",
		},
	}

	secret3 := &corev1.Secret{
		ObjectMeta: v1.ObjectMeta{
			Name:      "tfstate-default-b",
			Namespace: "default",
		},
		Type: corev1.SecretTypeOpaque,
		Data: map[string][]byte{
			"tfstate": tfStateData,
		},
	}
	k8sClient3 := fake.NewClientBuilder().WithObjects(secret3).Build()
	meta3 := &TFConfigurationMeta{
		Backend: &backend.K8SBackend{
			Client:       k8sClient3,
			SecretSuffix: "b",
			SecretNS:     "default",
		},
	}

	secret4 := &corev1.Secret{
		ObjectMeta: v1.ObjectMeta{
			Name:      "tfstate-default-c",
			Namespace: "default",
		},
		Type: corev1.SecretTypeOpaque,
		Data: map[string][]byte{
			"tfstate": tfStateData,
		},
	}
	k8sClient4 := fake.NewClientBuilder().WithObjects(secret4).Build()
	configuration4 := v1beta2.Configuration{
		Spec: v1beta2.ConfigurationSpec{
			WriteConnectionSecretToReference: &crossplane.SecretReference{
				Name:      "connection-secret-c",
				Namespace: "default",
			},
		},
	}
	meta4 := &TFConfigurationMeta{
		Backend: &backend.K8SBackend{
			Client:       k8sClient4,
			SecretSuffix: "c",
			SecretNS:     "default",
		},
	}

	secret5 := &corev1.Secret{
		ObjectMeta: v1.ObjectMeta{
			Name:      "tfstate-default-d",
			Namespace: "default",
		},
		Type: corev1.SecretTypeOpaque,
		Data: map[string][]byte{
			"tfstate": tfStateData,
		},
	}
	oldConnectionSecret5 := &corev1.Secret{
		ObjectMeta: v1.ObjectMeta{
			Name:      "connection-secret-d",
			Namespace: "default",
			Labels: map[string]string{
				"terraform.core.oam.dev/created-by": "terraform-controller",
				"terraform.core.oam.dev/owned-by":   "configuration5",
			},
		},
		TypeMeta: v1.TypeMeta{Kind: "Secret"},
		Data: map[string][]byte{
			"container_id": []byte("something"),
		},
	}
	k8sClient5 := fake.NewClientBuilder().WithObjects(secret5, oldConnectionSecret5).Build()
	configuration5 := v1beta2.Configuration{
		ObjectMeta: v1.ObjectMeta{
			Name: "configuration5",
		},
		Spec: v1beta2.ConfigurationSpec{
			WriteConnectionSecretToReference: &crossplane.SecretReference{
				Name:      "connection-secret-d",
				Namespace: "default",
			},
		},
	}
	meta5 := &TFConfigurationMeta{
		Backend: &backend.K8SBackend{
			Client:       k8sClient5,
			SecretSuffix: "d",
			SecretNS:     "default",
		},
	}

	secret6 := &corev1.Secret{
		ObjectMeta: v1.ObjectMeta{
			Name:      "tfstate-default-e",
			Namespace: "default",
		},
		Type: corev1.SecretTypeOpaque,
		Data: map[string][]byte{
			"tfstate": tfStateData,
		},
	}
	oldConnectionSecret6 := &corev1.Secret{
		ObjectMeta: v1.ObjectMeta{
			Name:      "connection-secret-e",
			Namespace: "default",
			Labels: map[string]string{
				"terraform.core.oam.dev/created-by":      "terraform-controller",
				"terraform.core.oam.dev/owned-by":        "configuration5",
				"terraform.core.oam.dev/owned-namespace": "default",
			},
		},
		TypeMeta: v1.TypeMeta{Kind: "Secret"},
		Data: map[string][]byte{
			"container_id": []byte("something"),
		},
	}
	k8sClient6 := fake.NewClientBuilder().WithObjects(secret6, oldConnectionSecret6).Build()
	configuration6 := v1beta2.Configuration{
		ObjectMeta: v1.ObjectMeta{
			Name:      "configuration6",
			Namespace: "default",
		},
		Spec: v1beta2.ConfigurationSpec{
			WriteConnectionSecretToReference: &crossplane.SecretReference{
				Name:      "connection-secret-e",
				Namespace: "default",
			},
		},
	}
	meta6 := &TFConfigurationMeta{
		Backend: &backend.K8SBackend{
			Client:       k8sClient6,
			SecretSuffix: "e",
			SecretNS:     "default",
		},
	}

	namespaceA := &corev1.Namespace{ObjectMeta: v1.ObjectMeta{Name: "a"}}
	namespaceB := &corev1.Namespace{ObjectMeta: v1.ObjectMeta{Name: "b"}}
	secret7 := &corev1.Secret{
		ObjectMeta: v1.ObjectMeta{
			Name:      "tfstate-default-f",
			Namespace: "a",
		},
		Type: corev1.SecretTypeOpaque,
		Data: map[string][]byte{
			"tfstate": tfStateData,
		},
	}
	oldConnectionSecret7 := &corev1.Secret{
		ObjectMeta: v1.ObjectMeta{
			Name:      "connection-secret-e",
			Namespace: "default",
			Labels: map[string]string{
				"terraform.core.oam.dev/created-by":      "terraform-controller",
				"terraform.core.oam.dev/owned-by":        "configuration6",
				"terraform.core.oam.dev/owned-namespace": "a",
			},
		},
		TypeMeta: v1.TypeMeta{Kind: "Secret"},
		Data: map[string][]byte{
			"container_id": []byte("something"),
		},
	}
	k8sClient7 := fake.NewClientBuilder().WithObjects(namespaceA, namespaceB, secret7, oldConnectionSecret7).Build()
	configuration7 := v1beta2.Configuration{
		ObjectMeta: v1.ObjectMeta{
			Name:      "configuration6",
			Namespace: "b",
		},
		Spec: v1beta2.ConfigurationSpec{
			WriteConnectionSecretToReference: &crossplane.SecretReference{
				Name:      "connection-secret-e",
				Namespace: "default",
			},
		},
	}
	meta7 := &TFConfigurationMeta{
		Backend: &backend.K8SBackend{
			Client:       k8sClient7,
			SecretSuffix: "f",
			SecretNS:     "a",
		},
	}

	secret8 := &corev1.Secret{
		ObjectMeta: v1.ObjectMeta{
			Name:      "tfstate-default-d",
			Namespace: "default",
		},
		Type: corev1.SecretTypeOpaque,
		Data: map[string][]byte{
			"tfstate": tfStateData,
		},
	}
	oldConnectionSecret8 := &corev1.Secret{
		ObjectMeta: v1.ObjectMeta{
			Name:      "connection-secret-d",
			Namespace: "default",
			Labels: map[string]string{
				"terraform.core.oam.dev/created-by": "terraform-controller",
			},
		},
		TypeMeta: v1.TypeMeta{Kind: "Secret"},
		Data: map[string][]byte{
			"container_id": []byte("something"),
		},
	}
	k8sClient8 := fake.NewClientBuilder().WithObjects(secret8, oldConnectionSecret8).Build()
	configuration8 := v1beta2.Configuration{
		ObjectMeta: v1.ObjectMeta{
			Name: "configuration5",
		},
		Spec: v1beta2.ConfigurationSpec{
			WriteConnectionSecretToReference: &crossplane.SecretReference{
				Name:      "connection-secret-d",
				Namespace: "default",
			},
		},
	}
	meta8 := &TFConfigurationMeta{
		Backend: &backend.K8SBackend{
			Client:       k8sClient8,
			SecretSuffix: "d",
			SecretNS:     "default",
		},
	}

	testcases := map[string]struct {
		args args
		want want
	}{
		"could not find backend secret": {
			args: args{
				ctx:       ctx,
				k8sClient: k8sClient1,
				meta:      meta1,
			},
			want: want{
				property: nil,
				errMsg:   "terraform state file backend secret is not generated",
			},
		},
		"no data in a backend secret": {
			args: args{
				ctx:       ctx,
				k8sClient: k8sClient2,
				meta:      meta2,
			},
			want: want{
				property: nil,
				errMsg:   "failed to get tfstate from Terraform State secret",
			},
		},
		"some data in a backend secret": {
			args: args{
				ctx:       ctx,
				k8sClient: k8sClient3,
				meta:      meta3,
			},
			want: want{
				property: tfStateOutputs,
				errMsg:   "",
			},
		},
		"some data in a backend secret and creates a connectionSecret": {
			args: args{
				ctx:           ctx,
				k8sClient:     k8sClient4,
				configuration: configuration4,
				meta:          meta4,
			},
			want: want{
				property: tfStateOutputs,
				errMsg:   "",
			},
		},
		"some data in a backend secret and update a connectionSecret belong to the same configuration": {
			args: args{
				ctx:           ctx,
				k8sClient:     k8sClient5,
				configuration: configuration5,
				meta:          meta5,
			},
			want: want{
				property: tfStateOutputs,
				errMsg:   "",
			},
		},
		"some data in a backend secret and update a connectionSecret belong to another configuration": {
			args: args{
				ctx:           ctx,
				k8sClient:     k8sClient6,
				configuration: configuration6,
				meta:          meta6,
			},
			want: want{
				property: nil,
				errMsg:   "configuration(namespace: default ; name: configuration6) cannot update secret(namespace: default ; name: connection-secret-e) whose owner is configuration(namespace: default ; name: configuration5)",
			},
		},
		"update a connectionSecret belong to another configuration(same name but different namespace": {
			args: args{
				ctx:           ctx,
				k8sClient:     k8sClient7,
				configuration: configuration7,
				meta:          meta7,
			},
			want: want{
				property: nil,
				errMsg:   "configuration(namespace: b ; name: configuration6) cannot update secret(namespace: default ; name: connection-secret-e) whose owner is configuration(namespace: a ; name: configuration6)",
			},
		},
		"update a connectionSecret without owner labels": {
			args: args{
				ctx:           ctx,
				k8sClient:     k8sClient8,
				configuration: configuration8,
				meta:          meta8,
			},
			want: want{
				property: tfStateOutputs,
				errMsg:   "",
			},
		},
	}

	for name, tc := range testcases {
		t.Run(name, func(t *testing.T) {
			property, err := tc.args.meta.getTFOutputs(tc.args.ctx, tc.args.k8sClient, tc.args.configuration)
			if tc.want.errMsg != "" {
				if !strings.Contains(err.Error(), tc.want.errMsg) {
					t.Errorf("getTFOutputs() error = %v, wantErr %v", err, tc.want.errMsg)
				}
			}
			assert.Equal(t, tc.want.property, property)
		})
	}

}

func TestUpdateApplyStatus(t *testing.T) {
	type args struct {
		state   types.ConfigurationState
		message string
		meta    *TFConfigurationMeta
	}
	type want struct {
		errMsg string
	}
	ctx := context.Background()
	s := runtime.NewScheme()
	if err := v1beta2.AddToScheme(s); err != nil {
		t.Fatal(err)
	}

	configuration := &v1beta2.Configuration{
		ObjectMeta: v1.ObjectMeta{
			Name:       "a",
			Namespace:  "b",
			Generation: int64(1),
		},
		Spec: v1beta2.ConfigurationSpec{
			HCL: "c",
		},
		Status: v1beta2.ConfigurationStatus{
			Apply: v1beta2.ConfigurationApplyStatus{
				State: types.Available,
			},
		},
	}
	k8sClient = fake.NewClientBuilder().WithScheme(s).WithObjects(configuration).Build()

	testcases := map[string]struct {
		args args
		want want
	}{
		"configuration is available": {
			args: args{
				meta: &TFConfigurationMeta{
					Name:      "a",
					Namespace: "b",
				},
				state:   types.Available,
				message: "xxx",
			},
		},
		"configuration cloud not be found": {
			args: args{
				meta: &TFConfigurationMeta{
					Name:      "z",
					Namespace: "b",
				},
				state:   types.Available,
				message: "xxx",
			},
		},
	}
	for name, tc := range testcases {
		t.Run(name, func(t *testing.T) {
			err := tc.args.meta.UpdateApplyStatus(ctx, k8sClient, tc.args.state, tc.args.message)
			if tc.want.errMsg != "" || err != nil {
				if !strings.Contains(err.Error(), tc.want.errMsg) {
					t.Errorf("updateApplyStatus() error = %v, wantErr %v", err, tc.want.errMsg)
				}
			}
		})
	}
}

func TestAssembleAndTriggerJob(t *testing.T) {
	type args struct {
		executionType types.TerraformExecutionType
	}
	type want struct {
		errMsg string
	}
	ctx := context.Background()
	k8sClient = fake.NewClientBuilder().Build()
	meta := &TFConfigurationMeta{
		Namespace: "b",
	}

	patches := gomonkey.ApplyFunc(apiutil.GVKForObject, func(_ runtime.Object, _ *runtime.Scheme) (schema.GroupVersionKind, error) {
		return schema.GroupVersionKind{}, apierrors.NewNotFound(schema.GroupResource{}, "")
	})
	defer patches.Reset()

	testcases := map[string]struct {
		args args
		want want
	}{
		"failed to create ServiceAccount": {
			args: args{
				executionType: types.TerraformApply,
			},
			want: want{
				errMsg: "failed to create ServiceAccount for Terraform executor",
			},
		},
	}
	for name, tc := range testcases {
		t.Run(name, func(t *testing.T) {
			err := meta.AssembleAndTriggerJob(ctx, k8sClient, tc.args.executionType)
			if tc.want.errMsg != "" || err != nil {
				if !strings.Contains(err.Error(), tc.want.errMsg) {
					t.Errorf("assembleAndTriggerJob() error = %v, wantErr %v", err, tc.want.errMsg)
				}
			}
		})
	}
}

func TestCheckWhetherConfigurationChanges(t *testing.T) {
	type args struct {
		configurationType types.ConfigurationType
		meta              *TFConfigurationMeta
	}
	type want struct {
		errMsg string
	}
	ctx := context.Background()
	cm := &corev1.ConfigMap{
		ObjectMeta: v1.ObjectMeta{
			Name:      "a",
			Namespace: "b",
		},
		Data: map[string]string{
			"c": "d",
		},
	}
	k8sClient = fake.NewClientBuilder().WithObjects(cm).Build()

	testcases := map[string]struct {
		args args
		want want
	}{
		"unknown configuration type": {
			args: args{
				meta: &TFConfigurationMeta{
					ConfigurationCMName: "a",
					Namespace:           "b",
					ControllerNamespace: "b",
				},
				configurationType: "xxx",
			},
			want: want{
				errMsg: "unsupported configuration type, only HCL or Remote is supported",
			},
		},
		"configuration map is not found": {
			args: args{
				meta: &TFConfigurationMeta{
					ConfigurationCMName: "aaa",
					Namespace:           "b",
					ControllerNamespace: "b",
				},
				configurationType: "HCL",
			},
			want: want{
				errMsg: "",
			},
		},
		"configuration type is remote": {
			args: args{
				meta: &TFConfigurationMeta{
					ConfigurationCMName: "aaa",
					Namespace:           "b",
					ControllerNamespace: "b",
				},
				configurationType: "Remote",
			},
			want: want{
				errMsg: "",
			},
		},
		"create configuration for the first time": {
			args: args{
				meta: &TFConfigurationMeta{
					ConfigurationCMName: "aa",
					Namespace:           "b",
					ControllerNamespace: "b",
				},
				configurationType: "HCL",
			},
			want: want{
				errMsg: "",
			},
		},
	}
	for name, tc := range testcases {
		t.Run(name, func(t *testing.T) {
			err := tc.args.meta.CheckWhetherConfigurationChanges(ctx, k8sClient, tc.args.configurationType)
			if tc.want.errMsg != "" || err != nil {
				if !strings.Contains(err.Error(), tc.want.errMsg) {
					t.Errorf("CheckWhetherConfigurationChanges() error = %v, wantErr %v", err, tc.want.errMsg)
				}
			}

			if tc.want.errMsg == "" {
				assert.Nil(t, err)
				assert.False(t, tc.args.meta.ConfigurationChanged)
			}
		})
	}
}

func TestGetApplyJob(t *testing.T) {
	const (
		applyJobNameWithUID = "xxxx-xxxx-xxxx"
		controllerNamespace = "ctrl-ns"
		applyJobName        = "configuraion-apply"
		jobNamespace        = "configuration-ns"
		// legacyApplyJobName  = "legacy-job-name"
		// legacyJobNamespace  = "legacy-job-ns"
	)

	ctx := context.Background()
	s := runtime.NewScheme()
	if err := v1beta1.AddToScheme(s); err != nil {
		t.Fatal(err)
	}
	if err := v1beta2.AddToScheme(s); err != nil {
		t.Fatal(err)
	}
	if err := batchv1.AddToScheme(s); err != nil {
		t.Fatal(err)
	}
	baseMeta := TFConfigurationMeta{
		ApplyJobName:        applyJobName,
		ControllerNamespace: jobNamespace,
	}
	baseJob := &batchv1.Job{
		ObjectMeta: v1.ObjectMeta{
			Name:      applyJobName,
			Namespace: jobNamespace,
		},
	}
	jobInCtrlNS := &batchv1.Job{
		ObjectMeta: v1.ObjectMeta{
			Name:      applyJobNameWithUID,
			Namespace: controllerNamespace,
		},
	}
	legacyJob := baseJob.DeepCopy()

	testCases := []struct {
		name string

		objects []client.Object
		wantErr error
		meta    TFConfigurationMeta
	}{
		{
			name:    "get job successfully",
			objects: []client.Object{baseJob},
			meta:    baseMeta,
		},
		{name: "get legacy job successfully",
			objects: []client.Object{legacyJob},
			meta: TFConfigurationMeta{LegacySubResources: LegacySubResources{
				Namespace:    jobNamespace,
				ApplyJobName: applyJobName,
			}},
		},
		{
			name:    "get job in controller namespace",
			objects: []client.Object{jobInCtrlNS},
			meta: TFConfigurationMeta{
				ControllerNamespace: controllerNamespace,
				ApplyJobName:        applyJobNameWithUID,
			},
		},
		{
			name:    "not get any job",
			objects: []client.Object{},
			meta: TFConfigurationMeta{
				ControllerNamespace: controllerNamespace,
				ApplyJobName:        applyJobNameWithUID,
				LegacySubResources: LegacySubResources{
					Namespace:    jobNamespace,
					ApplyJobName: applyJobName,
				},
			},
			wantErr: errors.New("not found"),
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var job batchv1.Job
			k8sClient := fake.NewClientBuilder().WithScheme(s).WithObjects(tc.objects...).Build()

			err := tc.meta.GetApplyJob(ctx, k8sClient, &job)
			if tc.wantErr != nil {
				if err == nil {
					t.Errorf("expected error: %s, got nil", tc.wantErr)
				} else if !strings.Contains(err.Error(), tc.wantErr.Error()) {
					t.Errorf("expected error containing %q, got %q", tc.wantErr, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			}
		})
	}
}

func TestRenderConfiguration(t *testing.T) {
	type args struct {
		configuration         *v1beta2.Configuration
		configurationType     types.ConfigurationType
		controllerNSSpecified bool
	}
	type want struct {
		cfg              string
		backendInterface backend.Backend
		errMsg           string
	}

	k8sClient := fake.NewClientBuilder().Build()
	baseMeta := TFConfigurationMeta{
		K8sClient: k8sClient,
	}

	testcases := []struct {
		name string
		args args
		want want
	}{
		{
			name: "backend is not nil, configuration is hcl",
			args: args{
				configuration: &v1beta2.Configuration{
					ObjectMeta: v1.ObjectMeta{
						Namespace: "n1",
					},
					Spec: v1beta2.ConfigurationSpec{
						Backend: &v1beta2.Backend{},
						HCL:     "image_id=123",
					},
				},
				configurationType: types.ConfigurationHCL,
			},
			want: want{
				cfg: `image_id=123

terraform {
  backend "kubernetes" {
    secret_suffix     = ""
    in_cluster_config = true
    namespace         = "n1"
  }
}
`,
				backendInterface: &backend.K8SBackend{
					Client: k8sClient,
					HCLCode: `
terraform {
  backend "kubernetes" {
    secret_suffix     = ""
    in_cluster_config = true
    namespace         = "n1"
  }
}
`,
					SecretSuffix: "",
					SecretNS:     "n1",
				},
			},
		},
		{
			name: "backend is nil, configuration is remote",
			args: args{
				configuration: &v1beta2.Configuration{
					ObjectMeta: v1.ObjectMeta{
						Namespace: "n2",
					},
					Spec: v1beta2.ConfigurationSpec{
						Remote: "https://github.com/a/b.git",
					},
				},
				configurationType: types.ConfigurationRemote,
			},
			want: want{
				cfg: `
terraform {
  backend "kubernetes" {
    secret_suffix     = ""
    in_cluster_config = true
    namespace         = "n2"
  }
}
`,
				backendInterface: &backend.K8SBackend{
					Client: k8sClient,
					HCLCode: `
terraform {
  backend "kubernetes" {
    secret_suffix     = ""
    in_cluster_config = true
    namespace         = "n2"
  }
}
`,
					SecretSuffix: "",
					SecretNS:     "n2",
				},
			},
		},
		{
			name: "backend is nil, configuration is not supported",
			args: args{
				configuration: &v1beta2.Configuration{
					Spec: v1beta2.ConfigurationSpec{},
				},
			},
			want: want{
				errMsg: "Unsupported Configuration Type",
			},
		},
		{
			name: "controller-namespace specified, backend should have legacy secret suffix",
			args: args{
				configuration: &v1beta2.Configuration{
					Spec: v1beta2.ConfigurationSpec{
						Backend: nil,
					},
					ObjectMeta: v1.ObjectMeta{
						UID:       "xxxx-xxxx",
						Namespace: "n2",
						Name:      "name",
					},
				},
				controllerNSSpecified: true,
				configurationType:     types.ConfigurationRemote,
			},
			want: want{
				cfg: `
terraform {
  backend "kubernetes" {
    secret_suffix     = "xxxx-xxxx"
    in_cluster_config = true
    namespace         = "n2"
  }
}
`,
				backendInterface: &backend.K8SBackend{
					Client: k8sClient,
					HCLCode: `
terraform {
  backend "kubernetes" {
    secret_suffix     = "xxxx-xxxx"
    in_cluster_config = true
    namespace         = "n2"
  }
}
`,
					SecretSuffix:       "xxxx-xxxx",
					SecretNS:           "n2",
					LegacySecretSuffix: "name",
				}},
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			meta := baseMeta
			meta.ControllerNSSpecified = tc.args.controllerNSSpecified
			got, backendConf, err := meta.RenderConfiguration(tc.args.configuration, tc.args.configurationType)
			if tc.want.errMsg != "" && !strings.Contains(err.Error(), tc.want.errMsg) {
				t.Errorf("ValidConfigurationObject() error = %v, wantErr %v", err, tc.want.errMsg)
				return
			}
			if tc.want.errMsg == "" && err != nil {
				t.Errorf("ValidConfigurationObject() error = %v, wantErr nil", err)
				return
			}
			assert.Equal(t, tc.want.cfg, got)

			if !reflect.DeepEqual(tc.want.backendInterface, backendConf) {
				t.Errorf("backendInterface is not equal.\n got %#v\n, want %#v", backendConf, tc.want.backendInterface)
			}
		})
	}
}

func TestCheckGitCredentialsSecretReference(t *testing.T) {
	ctx := context.Background()
	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	k8sClient := fake.NewClientBuilder().WithScheme(scheme).Build()

	privateKey := []byte("aaa")
	knownHosts := []byte("zzz")
	secret := &corev1.Secret{
		ObjectMeta: v1.ObjectMeta{
			Namespace: "default",
			Name:      "git-ssh",
		},
		Data: map[string][]byte{
			corev1.SSHAuthPrivateKey: privateKey,
			"known_hosts":            knownHosts,
		},
		Type: corev1.SecretTypeSSHAuth,
	}
	assert.Nil(t, k8sClient.Create(ctx, secret))
	assert.Nil(t, k8sClient.Get(ctx, client.ObjectKeyFromObject(secret), secret))

	secretNoKnownhost := &corev1.Secret{
		ObjectMeta: v1.ObjectMeta{
			Namespace: "default",
			Name:      "git-ssh-no-known-hosts",
		},
		Data: map[string][]byte{
			corev1.SSHAuthPrivateKey: privateKey,
		},
		Type: corev1.SecretTypeSSHAuth,
	}
	assert.Nil(t, k8sClient.Create(ctx, secretNoKnownhost))

	secretNoPrivateKey := &corev1.Secret{
		ObjectMeta: v1.ObjectMeta{
			Namespace: "default",
			Name:      "git-ssh-no-private-key",
		},
		Data: map[string][]byte{
			"known_hosts": knownHosts,
		},
	}
	assert.Nil(t, k8sClient.Create(ctx, secretNoPrivateKey))

	type args struct {
		k8sClient                     client.Client
		GitCredentialsSecretReference *corev1.SecretReference
	}

	type want struct {
		secret *corev1.Secret
		errMsg string
	}

	testcases := []struct {
		name string
		args args
		want want
	}{
		{
			name: "secret not found",
			args: args{
				k8sClient: k8sClient,
				GitCredentialsSecretReference: &corev1.SecretReference{
					Namespace: "default",
					Name:      "git-shh",
				},
			},
			want: want{
				errMsg: "Failed to get git credentials secret: secrets \"git-shh\" not found",
			},
		},
		{
			name: "key 'known_hosts' not in git credentials secret",
			args: args{
				k8sClient: k8sClient,
				GitCredentialsSecretReference: &corev1.SecretReference{
					Namespace: "default",
					Name:      "git-ssh-no-known-hosts",
				},
			},
			want: want{
				errMsg: fmt.Sprintf("'%s' not in git credentials secret", types.GitCredsKnownHosts),
			},
		},
		{
			name: "key 'ssh-privatekey' not in git credentials secret",
			args: args{
				k8sClient: k8sClient,
				GitCredentialsSecretReference: &corev1.SecretReference{
					Namespace: "default",
					Name:      "git-ssh-no-private-key",
				},
			},
			want: want{
				errMsg: fmt.Sprintf("'%s' not in git credentials secret", corev1.SSHAuthPrivateKey),
			},
		},
		{
			name: "secret exists",
			args: args{
				k8sClient: k8sClient,
				GitCredentialsSecretReference: &corev1.SecretReference{
					Namespace: "default",
					Name:      "git-ssh",
				},
			},
			want: want{
				secret: secret,
			},
		},
	}
	neededKeys := []string{types.GitCredsKnownHosts, corev1.SSHAuthPrivateKey}
	errKey := "git credentials"

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			sec, err := GetSecretOrConfigMap(ctx, tc.args.k8sClient, true, tc.args.GitCredentialsSecretReference, neededKeys, errKey)
			if err != nil {
				assert.EqualError(t, err, tc.want.errMsg)
			}
			if tc.want.secret != nil {
				assert.EqualValues(t, sec, tc.want.secret)
			}
		})
	}
}

func TestCheckTerraformCredentialsSecretReference(t *testing.T) {
	ctx := context.Background()
	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	k8sClient := fake.NewClientBuilder().WithScheme(scheme).Build()

	credentialstfrcjson := []byte("tfcreds")
	terraformrc := []byte("tfrc")

	secret := &corev1.Secret{
		ObjectMeta: v1.ObjectMeta{
			Namespace: "default",
			Name:      "terraform-creds",
		},
		Data: map[string][]byte{
			"credentials.tfrc.json": credentialstfrcjson,
			"terraformrc":           terraformrc,
		},
		Type: corev1.SecretTypeSSHAuth,
	}
	assert.Nil(t, k8sClient.Create(ctx, secret))
	assert.Nil(t, k8sClient.Get(ctx, client.ObjectKeyFromObject(secret), secret))

	secretNotTerraformCreds := &corev1.Secret{
		ObjectMeta: v1.ObjectMeta{
			Namespace: "default",
			Name:      "terraform-creds-no-creds",
		},
		Data: map[string][]byte{
			"terraformrc": terraformrc,
		},
		Type: corev1.SecretTypeSSHAuth,
	}

	assert.Nil(t, k8sClient.Create(ctx, secretNotTerraformCreds))

	type args struct {
		k8sClient                           client.Client
		TerraformCredentialsSecretReference *corev1.SecretReference
	}

	type want struct {
		secret *corev1.Secret
		errMsg string
	}

	testcases := []struct {
		name string
		args args
		want want
	}{
		{
			name: "secret not found",
			args: args{
				k8sClient: k8sClient,
				TerraformCredentialsSecretReference: &corev1.SecretReference{
					Namespace: "default",
					Name:      "secret-not-exists",
				},
			},
			want: want{
				errMsg: "Failed to get terraform credentials secret: secrets \"secret-not-exists\" not found",
			},
		},
		{
			name: "key 'credentials.tfrc.json' not in terraform credentials secret",
			args: args{
				k8sClient: k8sClient,
				TerraformCredentialsSecretReference: &corev1.SecretReference{
					Namespace: "default",
					Name:      "terraform-creds-no-creds",
				},
			},
			want: want{
				errMsg: fmt.Sprintf("'%s' not in terraform credentials secret", types.TerraformCredentials),
			},
		},
		{
			name: "secret exists",
			args: args{
				k8sClient: k8sClient,
				TerraformCredentialsSecretReference: &corev1.SecretReference{
					Namespace: "default",
					Name:      "terraform-creds",
				},
			},
			want: want{
				secret: secret,
			},
		},
	}

	neededKeys := []string{types.TerraformCredentials}
	errKey := "terraform credentials"

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			sec, err := GetSecretOrConfigMap(ctx, tc.args.k8sClient, true, tc.args.TerraformCredentialsSecretReference, neededKeys, errKey)

			if err != nil {
				assert.EqualError(t, err, tc.want.errMsg)
			}
			if tc.want.secret != nil {
				assert.EqualValues(t, sec, tc.want.secret)
			}
		})
	}

}

func TestCheckTerraformRCConfigMapReference(t *testing.T) {
	ctx := context.Background()
	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	k8sClient := fake.NewClientBuilder().WithScheme(scheme).Build()

	configMap := &corev1.ConfigMap{
		ObjectMeta: v1.ObjectMeta{
			Namespace: "default",
			Name:      "terraform-registry-config",
		},
		Data: map[string]string{
			".terraformrc": "tfrc",
		},
	}

	assert.Nil(t, k8sClient.Create(ctx, configMap))
	assert.Nil(t, k8sClient.Get(ctx, client.ObjectKeyFromObject(configMap), configMap))

	configMapNotTerraformRc := &corev1.ConfigMap{
		ObjectMeta: v1.ObjectMeta{
			Namespace: "default",
			Name:      "terraform-registry-config-no-terraformrc",
		},
		Data: map[string]string{
			"terraform": "tfrc",
		},
	}

	assert.Nil(t, k8sClient.Create(ctx, configMapNotTerraformRc))
	assert.Nil(t, k8sClient.Get(ctx, client.ObjectKeyFromObject(configMapNotTerraformRc), configMapNotTerraformRc))

	type args struct {
		k8sClient                     client.Client
		TerraformRCConfigMapReference *corev1.SecretReference
	}

	type want struct {
		configMap *corev1.ConfigMap
		errMsg    string
	}

	testcases := []struct {
		name string
		args args
		want want
	}{
		{
			name: "configmap not found",
			args: args{
				k8sClient: k8sClient,
				TerraformRCConfigMapReference: &corev1.SecretReference{
					Namespace: "default",
					Name:      "configmap-not-exists",
				},
			},
			want: want{
				errMsg: "Failed to get terraformrc configuration configmap: configmaps \"configmap-not-exists\" not found",
			},
		},
		{
			name: "key '.terraformrc' not in terraform registry config",
			args: args{
				k8sClient: k8sClient,
				TerraformRCConfigMapReference: &corev1.SecretReference{
					Namespace: "default",
					Name:      "terraform-registry-config-no-terraformrc",
				},
			},
			want: want{
				errMsg: fmt.Sprintf("'%s' not in terraformrc configuration configmap", types.TerraformRegistryConfig),
			},
		},
		{
			name: "configmap exists",
			args: args{
				k8sClient: k8sClient,
				TerraformRCConfigMapReference: &corev1.SecretReference{
					Namespace: "default",
					Name:      "terraform-registry-config",
				},
			},
			want: want{
				configMap: configMap,
			},
		},
	}

	neededKeys := []string{types.TerraformRegistryConfig}
	errKey := "terraformrc configuration"

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			configMap, err := GetSecretOrConfigMap(ctx, tc.args.k8sClient, false, tc.args.TerraformRCConfigMapReference, neededKeys, errKey)

			if err != nil {
				assert.EqualError(t, err, tc.want.errMsg)
			}
			if tc.want.configMap != nil {
				assert.EqualValues(t, configMap, tc.want.configMap)
			}
		})
	}
}

func TestTerraformCredentialsHelperConfigMap(t *testing.T) {
	ctx := context.Background()
	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	k8sClient := fake.NewClientBuilder().WithScheme(scheme).Build()

	configMap := &corev1.ConfigMap{
		ObjectMeta: v1.ObjectMeta{
			Namespace: "default",
			Name:      "terraform-credentials-helper",
		},
		Data: map[string]string{
			"terraform-credentials-artifactory": "tfrc",
		},
	}

	assert.Nil(t, k8sClient.Create(ctx, configMap))
	assert.Nil(t, k8sClient.Get(ctx, client.ObjectKeyFromObject(configMap), configMap))

	type args struct {
		k8sClient                                    client.Client
		TerraformCredentialsHelperConfigMapReference *corev1.SecretReference
	}

	type want struct {
		configMap *corev1.ConfigMap
		errMsg    string
	}

	testcases := []struct {
		name string
		args args
		want want
	}{
		{
			name: "configmap not found",
			args: args{
				k8sClient: k8sClient,
				TerraformCredentialsHelperConfigMapReference: &corev1.SecretReference{
					Namespace: "default",
					Name:      "terraform-registry",
				},
			},
			want: want{
				errMsg: "Failed to get terraform credentials helper configmap: configmaps \"terraform-registry\" not found",
			},
		},
		{
			name: "configmap exists",
			args: args{
				k8sClient: k8sClient,
				TerraformCredentialsHelperConfigMapReference: &corev1.SecretReference{
					Namespace: "default",
					Name:      "terraform-credentials-helper",
				},
			},
			want: want{
				configMap: configMap,
			},
		},
	}

	neededKeys := []string{}
	errKey := "terraform credentials helper"

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			configMap, err := GetSecretOrConfigMap(ctx, tc.args.k8sClient, false, tc.args.TerraformCredentialsHelperConfigMapReference, neededKeys, errKey)

			if err != nil {
				assert.EqualError(t, err, tc.want.errMsg)
			}
			if tc.want.configMap != nil {
				assert.EqualValues(t, configMap, tc.want.configMap)
			}
		})
	}
}
