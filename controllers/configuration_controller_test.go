package controllers

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"k8s.io/utils/pointer"

	"github.com/agiledragon/gomonkey/v2"
	"github.com/aliyun/alibaba-cloud-sdk-go/services/sts"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	k8stypes "k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/oam-dev/terraform-controller/api/types"
	crossplane "github.com/oam-dev/terraform-controller/api/types/crossplane-runtime"
	"github.com/oam-dev/terraform-controller/api/v1beta1"
	"github.com/oam-dev/terraform-controller/api/v1beta2"
	"github.com/oam-dev/terraform-controller/controllers/configuration/backend"
	"github.com/oam-dev/terraform-controller/controllers/provider"
)

func TestInitTFConfigurationMeta(t *testing.T) {
	req := ctrl.Request{}
	req.Namespace = "default"
	req.Name = "abc"

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

				RemoteGitPath: ".",
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

				RemoteGitPath: "alibaba/rds",
				ProviderReference: &crossplane.Reference{
					Name:      "xxx",
					Namespace: "default",
				},
			},
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			meta := initTFConfigurationMeta(req, tc.configuration, nil)
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
					DeleteResource: pointer.Bool(true),
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
					DeleteResource: pointer.Bool(false),
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
			meta := initTFConfigurationMeta(req, tc.configuration, nil)
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
	meta := initTFConfigurationMeta(req, configuration, nil)
	assert.Equal(t, meta.JobNodeSelector, map[string]string{"ssd": "true"})
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
	err := meta.prepareTFVariables(&configuration)
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

func TestInitTFConfigurationMetaWithJobEnv(t *testing.T) {
	req := ctrl.Request{}
	r := &ConfigurationReconciler{}
	s := runtime.NewScheme()
	v1beta1.AddToScheme(s)
	v1beta2.AddToScheme(s)
	corev1.AddToScheme(s)
	req.Namespace = "default"
	req.Name = "abc"
	prjID := "PrjID"
	prjIDValue := "abc"
	data, _ := json.Marshal(map[string]interface{}{
		prjID: prjIDValue,
	})
	ak := provider.AlibabaCloudCredentials{
		AccessKeyID:     "aaaa",
		AccessKeySecret: "bbbbb",
	}
	credentials, err := json.Marshal(&ak)
	secret := corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "default",
			Namespace: "default",
		},
		Data: map[string][]byte{
			"credentials": credentials,
		},
		Type: corev1.SecretTypeOpaque,
	}
	provider := v1beta1.Provider{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "terraform.core.oam.dev/v1beta2",
			Kind:       "Provider",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "default",
			Namespace: "default",
		},
		Spec: v1beta1.ProviderSpec{
			Provider: "ucloud",
			Credentials: v1beta1.ProviderCredentials{
				Source: crossplane.CredentialsSourceSecret,
				SecretRef: &crossplane.SecretKeySelector{
					SecretReference: crossplane.SecretReference{
						Name:      "default",
						Namespace: "default",
					},
					Key: "credentials",
				},
			},
		},
	}
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
			JobEnv: &runtime.RawExtension{
				Raw: data,
			},
			HCL: "test",
		},
	}
	r.Client = fake.NewClientBuilder().WithScheme(s).WithObjects(&configuration, &provider, &secret).Build()
	meta := initTFConfigurationMeta(req, configuration, r.Client)
	err = r.preCheck(context.Background(), &configuration, meta)
	assert.Nil(t, err)
	assert.Equal(t, meta.JobEnv, map[string]interface{}{
		prjID: prjIDValue,
	})
	assert.Equal(t, meta.VariableSecretData[prjID], []byte(prjIDValue))
	envExist := false
	for _, e := range meta.Envs {
		if e.Name == prjID {
			envExist = true
		}
	}
	assert.Equal(t, envExist, true)
}

func TestCheckProvider(t *testing.T) {
	ctx := context.Background()
	scheme := runtime.NewScheme()
	v1beta2.AddToScheme(scheme)

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
			if err := meta.getCredentials(ctx, tc.args.k8sClient, tc.args.provider); tc.want != "" &&
				!strings.Contains(err.Error(), tc.want) {
				t.Errorf("getCredentials = %v, want %v", err.Error(), tc.want)
			}
		})
	}
}

func TestConfigurationReconcile(t *testing.T) {
	req := ctrl.Request{}
	req.NamespacedName = k8stypes.NamespacedName{
		Name:      "a",
		Namespace: "b",
	}

	r1 := &ConfigurationReconciler{}
	ctx := context.Background()
	s := runtime.NewScheme()
	v1beta1.AddToScheme(s)
	v1beta2.AddToScheme(s)
	corev1.AddToScheme(s)
	batchv1.AddToScheme(s)
	r1.Client = fake.NewClientBuilder().WithScheme(s).Build()

	ak := provider.AlibabaCloudCredentials{
		AccessKeyID:     "aaaa",
		AccessKeySecret: "bbbbb",
	}
	credentials, err := json.Marshal(&ak)
	assert.Nil(t, err)
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "default",
			Namespace: "default",
		},
		Data: map[string][]byte{
			"credentials": credentials,
		},
		Type: corev1.SecretTypeOpaque,
	}

	provider := &v1beta1.Provider{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "terraform.core.oam.dev/v1beta2",
			Kind:       "Provider",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "default",
			Namespace: "default",
		},
		Spec: v1beta1.ProviderSpec{
			Provider: "alibaba",
			Credentials: v1beta1.ProviderCredentials{
				Source: "Secret",
				SecretRef: &crossplane.SecretKeySelector{
					SecretReference: crossplane.SecretReference{
						Name:      "default",
						Namespace: "default",
					},
					Key: "credentials",
				},
			},
			Region: "xxx",
		},
	}

	data, _ := json.Marshal(map[string]interface{}{
		"name": "abc",
	})
	variables := &runtime.RawExtension{Raw: data}
	configuration2 := &v1beta2.Configuration{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "a",
			Namespace: "b",
		},
		Spec: v1beta2.ConfigurationSpec{
			HCL:      "c",
			Variable: variables,
		},
		Status: v1beta2.ConfigurationStatus{
			Apply: v1beta2.ConfigurationApplyStatus{
				State: types.Available,
			},
		},
	}
	configuration2.Spec.ProviderReference = &crossplane.Reference{
		Name:      "default",
		Namespace: "default",
	}
	configuration2.Spec.WriteConnectionSecretToReference = &crossplane.SecretReference{
		Name:      "db-conn",
		Namespace: "default",
	}

	patches := gomonkey.ApplyMethod(reflect.TypeOf(&sts.Client{}), "GetCallerIdentity", func(_ *sts.Client, request *sts.GetCallerIdentityRequest) (response *sts.GetCallerIdentityResponse, err error) {
		response = nil
		err = nil
		return
	})
	defer patches.Reset()

	applyingJob2 := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      req.Name + "-" + string(TerraformApply),
			Namespace: req.Namespace,
		},
		Status: batchv1.JobStatus{
			Succeeded: int32(1),
		},
	}

	stateData, _ := base64.StdEncoding.DecodeString("H4sIAAAAAAAA/4SQzarbMBCF934KoXUdPKNf+1VKCWNp5AocO8hyaSl592KlcBd3cZfnHPHpY/52QshfXI68b3IS+tuVK5dCaS+P+8ci4TbcULb94JJplZPAFte8MS18PQrKBO8Q+xk59SHa1AMA9M4YmoN3FGJ8M/azPs96yElcCkLIsG+V8sblnqOc3uXlRuvZ0GxSSuiCRUYbw2gGHRFGPxitEgJYQDQ0a68I2ChNo1cAZJ2bR20UtW8bsv55NuJRS94W2erXe5X5QQs3A/FZ4fhJaOwUgZTVMRjto1HGpSGSQuuD955hdDDPcR6NY1ZpQJ/YwagTRAvBpsi8LXn7Pa1U+ahfWHX/zWThYz9L4Otg3390r+5fAAAA//8hmcuNuQEAAA==")

	backendSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "tfstate-default-a",
			Namespace: "vela-system",
		},
		Data: map[string][]byte{
			"tfstate": stateData,
		},
		Type: corev1.SecretTypeOpaque,
	}

	variableSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf(TFVariableSecret, req.Name),
			Namespace: req.Namespace,
		},
		Data: map[string][]byte{
			"name": []byte("def"),
		},
		Type: corev1.SecretTypeOpaque,
	}

	r2 := &ConfigurationReconciler{}
	r2.Client = fake.NewClientBuilder().WithScheme(s).WithObjects(secret, provider, applyingJob2, backendSecret,
		variableSecret, configuration2).Build()

	time := v1.NewTime(time.Now())
	configuration3 := &v1beta2.Configuration{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "a",
			Namespace:         "b",
			DeletionTimestamp: &time,
		},
		Spec: v1beta2.ConfigurationSpec{
			HCL: "c",
		},
	}
	configuration3.Spec.ProviderReference = &crossplane.Reference{
		Name:      "default",
		Namespace: "default",
	}

	destroyJob3 := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "a-destroy",
			Namespace: req.Namespace,
		},
		Status: batchv1.JobStatus{
			Succeeded: int32(1),
		},
	}

	r3 := &ConfigurationReconciler{}
	r3.Client = fake.NewClientBuilder().WithScheme(s).WithObjects(secret, provider, configuration3, destroyJob3).Build()

	configuration4 := &v1beta2.Configuration{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "a",
			Namespace:         "b",
			DeletionTimestamp: &time,
			Finalizers:        []string{configurationFinalizer},
		},
		Spec: v1beta2.ConfigurationSpec{
			HCL: "c",
			Backend: &v1beta2.Backend{
				BackendType: "kubernetes",
				Kubernetes: &v1beta2.KubernetesBackendConf{
					SecretSuffix: "a",
					Namespace:    &backendSecret.Namespace,
				},
			},
		},
		Status: v1beta2.ConfigurationStatus{
			Apply: v1beta2.ConfigurationApplyStatus{
				State: types.ConfigurationProvisioningAndChecking,
			},
		},
	}
	configuration4.Spec.ProviderReference = &crossplane.Reference{
		Name:      "default",
		Namespace: "default",
	}

	destroyJob4 := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "a-destroy",
			Namespace: req.Namespace,
		},
		Status: batchv1.JobStatus{
			Succeeded: int32(1),
		},
	}

	r4 := &ConfigurationReconciler{}
	r4.Client = fake.NewClientBuilder().WithScheme(s).WithObjects(secret, provider, configuration4, destroyJob4, backendSecret).Build()

	configuration5 := &v1beta2.Configuration{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "a",
			Namespace:         "b",
			DeletionTimestamp: &time,
			Finalizers:        []string{configurationFinalizer},
		},
		Spec: v1beta2.ConfigurationSpec{
			HCL: "c",
		},
	}
	configuration5.Spec.ProviderReference = &crossplane.Reference{
		Name:      "default",
		Namespace: "default",
	}

	destroyJob5 := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "a-destroy",
			Namespace: req.Namespace,
		},
		Status: batchv1.JobStatus{
			Succeeded: int32(1),
		},
	}

	r5 := &ConfigurationReconciler{}
	r5.Client = fake.NewClientBuilder().WithScheme(s).WithObjects(secret, provider, configuration5, destroyJob5).Build()

	// @step: create the setup for the job namespace tests
	configuration6 := &v1beta2.Configuration{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "a",
			Namespace:  "b",
			Finalizers: []string{configurationFinalizer},
			UID:        "12345",
		},
		Spec: v1beta2.ConfigurationSpec{
			HCL: "c",
		},
	}

	namespace := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "builds"}}
	r6 := &ConfigurationReconciler{ControllerNamespace: "builds"}
	r6.Client = fake.NewClientBuilder().
		WithScheme(s).
		WithObjects(namespace, secret, provider, configuration6).
		Build()

	// for case "Configuration changed, and reconcile"
	appliedConfigurationCM := &corev1.ConfigMap{
		ObjectMeta: v1.ObjectMeta{
			Name:      "tf-a",
			Namespace: req.Namespace,
		},
		Data: map[string]string{types.TerraformHCLConfigurationName: `Here is the original hcl

terraform {
  backend "kubernetes" {
    secret_suffix     = "a"
    in_cluster_config = true
    namespace         = "b"
  }
}
`,
		},
	}
	varMap := map[string]string{"name": "abc"}
	appliedEnvVariable := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf(TFVariableSecret, req.Name),
			Namespace: req.Namespace,
		},
		Data: map[string][]byte{
			"TF_VAR_name":             []byte(varMap["name"]),
			"ALICLOUD_ACCESS_KEY":     []byte(ak.AccessKeyID),
			"ALICLOUD_SECRET_KEY":     []byte(ak.AccessKeySecret),
			"ALICLOUD_REGION":         []byte(provider.Spec.Region),
			"ALICLOUD_SECURITY_TOKEN": []byte(""),
		},
		Type: corev1.SecretTypeOpaque,
	}
	appliedJobName := req.Name + "-" + string(TerraformApply)
	appliedJob := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      appliedJobName,
			Namespace: req.Namespace,
			UID:       "111",
		},
		Status: batchv1.JobStatus{
			Succeeded: int32(1),
		},
	}
	varData, _ := json.Marshal(varMap)
	configuration7 := &v1beta2.Configuration{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "a",
			Namespace: "b",
		},
		Spec: v1beta2.ConfigurationSpec{
			HCL:      "Here is the changed hcl",
			Variable: &runtime.RawExtension{Raw: varData},
			ProviderReference: &crossplane.Reference{
				Name:      "default",
				Namespace: "default",
			},
			WriteConnectionSecretToReference: &crossplane.SecretReference{
				Name:      "db-conn",
				Namespace: "default",
			},
		},
		Status: v1beta2.ConfigurationStatus{
			Apply: v1beta2.ConfigurationApplyStatus{
				State: types.Available,
			},
		},
	}
	r7 := &ConfigurationReconciler{}
	r7.Client = fake.NewClientBuilder().WithScheme(s).WithObjects(secret, provider, backendSecret,
		appliedJob, appliedEnvVariable, appliedConfigurationCM, configuration7).Build()

	type args struct {
		req reconcile.Request
		r   *ConfigurationReconciler
	}

	type want struct {
		errMsg string
	}

	testcases := []struct {
		name  string
		args  args
		want  want
		check func(t *testing.T, cc client.Client)
	}{
		{
			name: "Configuration is not found",
			args: args{
				req: req,
				r:   r1,
			},
		},
		{
			name: "Configuration exists, and it's available",
			args: args{
				req: req,
				r:   r2,
			},
		},
		{
			name: "Configuration is deleting",
			args: args{
				req: req,
				r:   r3,
			},
		},
		{
			name: "Configuration is deleting, but failed to delete",
			args: args{
				req: req,
				r:   r4,
			},
			want: want{
				errMsg: "Destroy could not complete and needs to wait for Provision to complete first: Cloud resources are being provisioned and provisioning status is checking...",
			},
		},
		{
			name: "Configuration is deleting, and succeeded to delete",
			args: args{
				req: req,
				r:   r5,
			},
		},
		{
			name: "Builds should be run in the controller namespace when defined",
			args: args{
				req: req,
				r:   r6,
			},
			check: func(t *testing.T, cc client.Client) {
				job := &batchv1.Job{}
				err := cc.Get(context.TODO(), k8stypes.NamespacedName{Name: "12345-apply", Namespace: "builds"}, job)
				if err != nil {
					t.Error("Failed to retrieve jobs from builds namespace")
				}
			},
		},
		{
			name: "Configuration changed, and reconcile",
			args: args{
				req: req,
				r:   r7,
			},
			check: func(t *testing.T, cc client.Client) {
				job := &batchv1.Job{}
				if err = cc.Get(context.TODO(), k8stypes.NamespacedName{Name: appliedJobName, Namespace: req.Namespace}, job); err != nil {
					t.Error("Failed to retrieve the new job")
				}
				assert.Equal(t, job.Name, appliedJob.Name, "Not expected job name")
				assert.Equal(t, job.Namespace, appliedJob.Namespace, "Not expected job namespace")
				assert.NotEqual(t, job.UID, appliedJob.UID, "No new job created")
				assert.NotEqual(t, job.Status.Succeeded, appliedJob.Status.Succeeded, "Not expected job status")
			},
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			for i := 0; i < 5; i++ {
				if _, err := tc.args.r.Reconcile(ctx, tc.args.req); tc.want.errMsg != "" &&
					!strings.Contains(err.Error(), tc.want.errMsg) {
					t.Errorf("Reconcile() error = %v, wantErr %v", err, tc.want.errMsg)
				}
			}
			if tc.check != nil {
				tc.check(t, tc.args.r.Client)
			}
		})
	}
}

func TestPreCheckResourcesSetting(t *testing.T) {
	r := &ConfigurationReconciler{}
	s := runtime.NewScheme()
	v1beta1.AddToScheme(s)
	v1beta2.AddToScheme(s)
	corev1.AddToScheme(s)
	corev1.AddToScheme(s)
	provider := &v1beta1.Provider{
		ObjectMeta: v1.ObjectMeta{
			Name:      "default",
			Namespace: "default",
		},
		Status: v1beta1.ProviderStatus{
			State: types.ProviderIsNotReady,
		},
	}
	r.Client = fake.NewClientBuilder().WithScheme(s).WithObjects(provider).Build()

	type args struct {
		r             *ConfigurationReconciler
		configuration *v1beta2.Configuration
		meta          *TFConfigurationMeta
	}

	type want struct {
		errMsg string
	}

	type prepare func(*testing.T)

	testcases := []struct {
		name string
		prepare
		args args
		want want
	}{
		{
			name: "wrong value in environment variable RESOURCES_LIMITS_CPU",
			prepare: func(t *testing.T) {
				t.Setenv("RESOURCES_LIMITS_CPU", "abcde")
			},
			args: args{
				r: r,
				configuration: &v1beta2.Configuration{
					ObjectMeta: v1.ObjectMeta{
						Name: "abc",
					},
					Spec: v1beta2.ConfigurationSpec{
						HCL: "bbb",
					},
				},
				meta: &TFConfigurationMeta{
					ConfigurationCMName: "abc",
					ProviderReference: &crossplane.Reference{
						Namespace: "default",
						Name:      "default",
					},
				},
			},
			want: want{
				errMsg: "failed to parse env variable RESOURCES_LIMITS_CPU into resource.Quantity",
			},
		},
		{
			name: "wrong value in environment variable RESOURCES_LIMITS_MEMORY",
			prepare: func(t *testing.T) {
				t.Setenv("RESOURCES_LIMITS_MEMORY", "xxxx5Gi")
			},
			args: args{
				r: r,
				configuration: &v1beta2.Configuration{
					ObjectMeta: v1.ObjectMeta{
						Name: "abc",
					},
					Spec: v1beta2.ConfigurationSpec{
						HCL: "bbb",
					},
				},
				meta: &TFConfigurationMeta{
					ConfigurationCMName: "abc",
					ProviderReference: &crossplane.Reference{
						Namespace: "default",
						Name:      "default",
					},
				},
			},
			want: want{
				errMsg: "failed to parse env variable RESOURCES_LIMITS_MEMORY into resource.Quantity",
			},
		},
		{
			name: "wrong value in environment variable RESOURCES_REQUESTS_CPU",
			prepare: func(t *testing.T) {
				t.Setenv("RESOURCES_REQUESTS_CPU", "ekiadasdflksas")
			},
			args: args{
				r: r,
				configuration: &v1beta2.Configuration{
					ObjectMeta: v1.ObjectMeta{
						Name: "abc",
					},
					Spec: v1beta2.ConfigurationSpec{
						HCL: "bbb",
					},
				},
				meta: &TFConfigurationMeta{
					ConfigurationCMName: "abc",
					ProviderReference: &crossplane.Reference{
						Namespace: "default",
						Name:      "default",
					},
				},
			},
			want: want{
				errMsg: "failed to parse env variable RESOURCES_REQUESTS_CPU into resource.Quantity",
			},
		},
		{
			name: "wrong value in environment variable RESOURCES_REQUESTS_MEMORY",
			prepare: func(t *testing.T) {
				t.Setenv("RESOURCES_REQUESTS_MEMORY", "123x456")
			},
			args: args{
				r: r,
				configuration: &v1beta2.Configuration{
					ObjectMeta: v1.ObjectMeta{
						Name: "abc",
					},
					Spec: v1beta2.ConfigurationSpec{
						HCL: "bbb",
					},
				},
				meta: &TFConfigurationMeta{
					ConfigurationCMName: "abc",
					ProviderReference: &crossplane.Reference{
						Namespace: "default",
						Name:      "default",
					},
				},
			},
			want: want{
				errMsg: "failed to parse env variable RESOURCES_REQUESTS_MEMORY into resource.Quantity",
			},
		},
		{
			name: "correct value of resources setting in environment variable",
			prepare: func(t *testing.T) {
				t.Setenv("RESOURCES_LIMITS_CPU", "10m")
				t.Setenv("RESOURCES_LIMITS_MEMORY", "10Mi")
				t.Setenv("RESOURCES_REQUESTS_CPU", "100")
				t.Setenv("RESOURCES_REQUESTS_MEMORY", "5Gi")
			},
			args: args{
				r: r,
				configuration: &v1beta2.Configuration{
					ObjectMeta: v1.ObjectMeta{
						Name: "abc",
					},
					Spec: v1beta2.ConfigurationSpec{
						HCL: "bbb",
					},
				},
				meta: &TFConfigurationMeta{
					ConfigurationCMName: "abc",
					ProviderReference: &crossplane.Reference{
						Namespace: "default",
						Name:      "default",
					},
				},
			},
			want: want{},
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.prepare != nil {
				tc.prepare(t)
			}
			if err := tc.args.r.preCheckResourcesSetting(tc.args.meta); (tc.want.errMsg != "") &&
				!strings.Contains(err.Error(), tc.want.errMsg) {
				t.Errorf("preCheckResourcesSetting() error = %v, wantErr %v", err, tc.want.errMsg)
			}
		})
	}
}

func TestPreCheck(t *testing.T) {
	r := &ConfigurationReconciler{}
	ctx := context.Background()
	s := runtime.NewScheme()
	v1beta1.AddToScheme(s)
	v1beta2.AddToScheme(s)
	corev1.AddToScheme(s)
	corev1.AddToScheme(s)
	provider := &v1beta1.Provider{
		ObjectMeta: v1.ObjectMeta{
			Name:      "default",
			Namespace: "default",
		},
		Status: v1beta1.ProviderStatus{
			State: types.ProviderIsNotReady,
		},
	}
	r.Client = fake.NewClientBuilder().WithScheme(s).WithObjects(provider).Build()

	type args struct {
		r             *ConfigurationReconciler
		configuration *v1beta2.Configuration
		meta          *TFConfigurationMeta
	}

	type want struct {
		errMsg string
	}

	type prepare func(*testing.T)

	testcases := []struct {
		name string
		prepare
		args args
		want want
	}{
		{
			name: "configuration is invalid",
			args: args{
				r: r,
				configuration: &v1beta2.Configuration{
					ObjectMeta: v1.ObjectMeta{
						Name: "abc",
					},
					Spec: v1beta2.ConfigurationSpec{
						Remote: "aaa",
						HCL:    "bbb",
					},
				},
				meta: &TFConfigurationMeta{},
			},
			want: want{
				errMsg: "spec.HCL and spec.Remote cloud not be set at the same time",
			},
		},
		{
			name: "HCL configuration is valid",
			args: args{
				r: r,
				configuration: &v1beta2.Configuration{
					ObjectMeta: v1.ObjectMeta{
						Name: "abc",
					},
					Spec: v1beta2.ConfigurationSpec{
						HCL: "bbb",
					},
				},
				meta: &TFConfigurationMeta{
					ConfigurationCMName: "abc",
					ProviderReference: &crossplane.Reference{
						Namespace: "default",
						Name:      "default",
					},
				},
			},
			want: want{},
		},
		{
			name: "Remote configuration is valid",
			args: args{
				r: r,
				configuration: &v1beta2.Configuration{
					ObjectMeta: v1.ObjectMeta{
						Name: "abc",
					},
					Spec: v1beta2.ConfigurationSpec{
						Remote: "https://github.com/a/b",
					},
				},
				meta: &TFConfigurationMeta{
					ConfigurationCMName: "abc",
					ProviderReference: &crossplane.Reference{
						Namespace: "default",
						Name:      "default",
					},
				},
			},
			want: want{},
		},
		{
			name: "could not find provider",
			args: args{
				r: r,
				configuration: &v1beta2.Configuration{
					ObjectMeta: v1.ObjectMeta{
						Name: "abc",
					},
					Spec: v1beta2.ConfigurationSpec{
						HCL: "bbb",
					},
				},
				meta: &TFConfigurationMeta{
					ConfigurationCMName: "abc",
					ProviderReference: &crossplane.Reference{
						Namespace: "d",
						Name:      "default",
					},
				},
			},
			want: want{
				errMsg: "provider not found",
			},
		},
		{
			name: "wrong value in environment variable RESOURCES_LIMITS_CPU",
			prepare: func(t *testing.T) {
				t.Setenv("RESOURCES_LIMITS_CPU", "abcde")
			},
			args: args{
				r: r,
				configuration: &v1beta2.Configuration{
					ObjectMeta: v1.ObjectMeta{
						Name: "abc",
					},
					Spec: v1beta2.ConfigurationSpec{
						HCL: "bbb",
					},
				},
				meta: &TFConfigurationMeta{
					ConfigurationCMName: "abc",
					ProviderReference: &crossplane.Reference{
						Namespace: "default",
						Name:      "default",
					},
				},
			},
			want: want{
				errMsg: "failed to parse env variable RESOURCES_LIMITS_CPU into resource.Quantity",
			},
		},
		{
			name: "wrong value in environment variable RESOURCES_LIMITS_MEMORY",
			prepare: func(t *testing.T) {
				t.Setenv("RESOURCES_LIMITS_MEMORY", "xxxx5Gi")
			},
			args: args{
				r: r,
				configuration: &v1beta2.Configuration{
					ObjectMeta: v1.ObjectMeta{
						Name: "abc",
					},
					Spec: v1beta2.ConfigurationSpec{
						HCL: "bbb",
					},
				},
				meta: &TFConfigurationMeta{
					ConfigurationCMName: "abc",
					ProviderReference: &crossplane.Reference{
						Namespace: "default",
						Name:      "default",
					},
				},
			},
			want: want{
				errMsg: "failed to parse env variable RESOURCES_LIMITS_MEMORY into resource.Quantity",
			},
		},
		{
			name: "wrong value in environment variable RESOURCES_REQUESTS_CPU",
			prepare: func(t *testing.T) {
				t.Setenv("RESOURCES_REQUESTS_CPU", "ekiadasdflksas")
			},
			args: args{
				r: r,
				configuration: &v1beta2.Configuration{
					ObjectMeta: v1.ObjectMeta{
						Name: "abc",
					},
					Spec: v1beta2.ConfigurationSpec{
						HCL: "bbb",
					},
				},
				meta: &TFConfigurationMeta{
					ConfigurationCMName: "abc",
					ProviderReference: &crossplane.Reference{
						Namespace: "default",
						Name:      "default",
					},
				},
			},
			want: want{
				errMsg: "failed to parse env variable RESOURCES_REQUESTS_CPU into resource.Quantity",
			},
		},
		{
			name: "wrong value in environment variable RESOURCES_REQUESTS_MEMORY",
			prepare: func(t *testing.T) {
				t.Setenv("RESOURCES_REQUESTS_MEMORY", "123x456")
			},
			args: args{
				r: r,
				configuration: &v1beta2.Configuration{
					ObjectMeta: v1.ObjectMeta{
						Name: "abc",
					},
					Spec: v1beta2.ConfigurationSpec{
						HCL: "bbb",
					},
				},
				meta: &TFConfigurationMeta{
					ConfigurationCMName: "abc",
					ProviderReference: &crossplane.Reference{
						Namespace: "default",
						Name:      "default",
					},
				},
			},
			want: want{
				errMsg: "failed to parse env variable RESOURCES_REQUESTS_MEMORY into resource.Quantity",
			},
		},
		{
			name: "correct value of resources setting in environment variable",
			prepare: func(t *testing.T) {
				t.Setenv("RESOURCES_LIMITS_CPU", "10m")
				t.Setenv("RESOURCES_LIMITS_MEMORY", "10Mi")
				t.Setenv("RESOURCES_REQUESTS_CPU", "100")
				t.Setenv("RESOURCES_REQUESTS_MEMORY", "5Gi")
			},
			args: args{
				r: r,
				configuration: &v1beta2.Configuration{
					ObjectMeta: v1.ObjectMeta{
						Name: "abc",
					},
					Spec: v1beta2.ConfigurationSpec{
						HCL: "bbb",
					},
				},
				meta: &TFConfigurationMeta{
					ConfigurationCMName: "abc",
					ProviderReference: &crossplane.Reference{
						Namespace: "default",
						Name:      "default",
					},
				},
			},
			want: want{},
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.prepare != nil {
				tc.prepare(t)
			}

			err := tc.args.r.preCheck(ctx, tc.args.configuration, tc.args.meta)
			if tc.want.errMsg != "" || err != nil {
				if !strings.Contains(err.Error(), tc.want.errMsg) {
					t.Errorf("preCheck() error = %v, wantErr %v", err, tc.want.errMsg)
				}
			}
		})
	}
}

func TestPreCheckWhenConfigurationIsChanged(t *testing.T) {
	r := &ConfigurationReconciler{}
	ctx := context.Background()
	s := runtime.NewScheme()
	v1beta1.AddToScheme(s)
	v1beta2.AddToScheme(s)
	corev1.AddToScheme(s)
	corev1.AddToScheme(s)
	provider := &v1beta1.Provider{
		ObjectMeta: v1.ObjectMeta{
			Name:      "default",
			Namespace: "default",
		},
		Status: v1beta1.ProviderStatus{
			State: types.ProviderIsNotReady,
		},
	}
	r.Client = fake.NewClientBuilder().WithScheme(s).WithObjects(provider).Build()

	provider3 := &v1beta1.Provider{
		ObjectMeta: v1.ObjectMeta{
			Name:      "default",
			Namespace: "default",
		},
		Status: v1beta1.ProviderStatus{
			State: types.ProviderIsNotReady,
		},
	}
	configurationCM3 := &corev1.ConfigMap{
		ObjectMeta: v1.ObjectMeta{
			Name:      "abc",
			Namespace: "default",
		},
	}
	r.Client = fake.NewClientBuilder().WithScheme(s).WithObjects(provider3, configurationCM3).Build()
	meta3 := &TFConfigurationMeta{
		ConfigurationCMName: "abc",
		ProviderReference: &crossplane.Reference{
			Namespace: "default",
			Name:      "default",
		},
		CompleteConfiguration: "d",
		Namespace:             "default",
	}

	patches := gomonkey.ApplyFunc(reflect.DeepEqual, func(x, y interface{}) bool {
		return true
	})
	defer patches.Reset()

	type args struct {
		r             *ConfigurationReconciler
		configuration *v1beta2.Configuration
		meta          *TFConfigurationMeta
	}

	type want struct {
		errMsg string
	}

	testcases := []struct {
		name string
		args args
		want want
	}{
		{
			name: "configuration is changed",
			args: args{
				r: r,
				configuration: &v1beta2.Configuration{
					ObjectMeta: v1.ObjectMeta{
						Name: "abc",
					},
					Spec: v1beta2.ConfigurationSpec{
						HCL: "bbb",
					},
				},
				meta: meta3,
			},
			want: want{},
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			if err := tc.args.r.preCheck(ctx, tc.args.configuration, tc.args.meta); (tc.want.errMsg != "") &&
				!strings.Contains(err.Error(), tc.want.errMsg) {
				t.Errorf("preCheck() error = %v, wantErr %v", err, tc.want.errMsg)
			}
		})
	}
}

func TestTerraformDestroy(t *testing.T) {
	const (
		// secrets/configmaps are dispatched to this ns
		controllerNamespace = "tf-controller-namespace"
		legacyNamespace     = "legacy-namespace"
		configurationCMName = "tf-config-cm"
		secretSuffix        = "secret-suffix"
		secretNS            = "default"
		destroyJobName      = "destroy-job"
		applyJobName        = "apply-job"

		connectionSecretName = "conn"
		connectionSecretNS   = "conn-ns"
	)
	ctx := context.Background()
	s := runtime.NewScheme()
	v1beta1.AddToScheme(s)
	v1beta2.AddToScheme(s)
	corev1.AddToScheme(s)
	batchv1.AddToScheme(s)
	rbacv1.AddToScheme(s)
	// this is default provider if not specified in configuration.spec.providerRef
	baseProvider := &v1beta1.Provider{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "default",
			Namespace: "default",
		},
	}
	readyProvider := baseProvider.DeepCopy()
	readyProvider.Status.State = types.ProviderIsReady
	notReadyProvider := baseProvider.DeepCopy()
	notReadyProvider.Status.State = types.ProviderIsNotReady

	baseApplyJob := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      applyJobName,
			Namespace: controllerNamespace,
		},
	}
	applyJobInLegacyNS := baseApplyJob.DeepCopy()
	applyJobInLegacyNS.Namespace = legacyNamespace

	baseDestroyJob := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      destroyJobName,
			Namespace: controllerNamespace,
		},
	}
	completeDestroyJob := baseDestroyJob.DeepCopy()
	completeDestroyJob.Status.Succeeded = int32(1)

	// Resources to be GC
	baseConfigurationCM := &corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ConfigMap",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      configurationCMName,
			Namespace: controllerNamespace,
		},
	}
	baseVariableSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf(TFVariableSecret, secretSuffix),
			Namespace: controllerNamespace,
		},
		Type: corev1.SecretTypeOpaque,
	}
	connectionSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      connectionSecretName,
			Namespace: connectionSecretNS,
		},
		Type: corev1.SecretTypeOpaque,
	}
	backendSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("tfstate-default-%s", secretSuffix),
			Namespace: "default",
		},
		Type: corev1.SecretTypeOpaque,
	}

	ConfigurationCMInLegacyNS := baseConfigurationCM.DeepCopy()
	ConfigurationCMInLegacyNS.Namespace = legacyNamespace
	variableSecretInLegacyNS := baseVariableSecret.DeepCopy()
	variableSecretInLegacyNS.Namespace = legacyNamespace

	baseMeta := TFConfigurationMeta{
		DestroyJobName:      destroyJobName,
		ControllerNamespace: controllerNamespace,
		ProviderReference: &crossplane.Reference{
			Name:      baseProvider.Name,
			Namespace: baseProvider.Namespace,
		},
		VariableSecretName:  fmt.Sprintf(TFVariableSecret, secretSuffix),
		ConfigurationCMName: configurationCMName,
		// True is default value if user ignores configuration.Spec.DeleteResource
		DeleteResource: true,
	}
	metaWithLegacyResource := baseMeta
	metaWithLegacyResource.LegacySubResources = LegacySubResources{
		Namespace:           legacyNamespace,
		ApplyJobName:        applyJobName,
		DestroyJobName:      destroyJobName,
		ConfigurationCMName: configurationCMName,
		VariableSecretName:  fmt.Sprintf(TFVariableSecret, secretSuffix),
	}

	metaWithDeleteResourceIsFalse := baseMeta
	metaWithDeleteResourceIsFalse.DeleteResource = false

	baseConfiguration := &v1beta2.Configuration{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "base-conf",
			Namespace: "default",
		},
	}
	configurationWithConnSecret := baseConfiguration.DeepCopy()
	configurationWithConnSecret.Spec.WriteConnectionSecretToReference = &crossplane.SecretReference{
		Name:      connectionSecretName,
		Namespace: connectionSecretNS,
	}
	configurationPrdNotFound := baseConfiguration.DeepCopy()
	configurationPrdNotFound.Spec.ProviderReference = &crossplane.Reference{
		Name:      "not-exist",
		Namespace: "default",
	}
	forceDeleteConfiguration := baseConfiguration.DeepCopy()
	forceDeleteConfiguration.Spec.ForceDelete = pointer.Bool(true)

	type args struct {
		namespace     string
		configuration *v1beta2.Configuration
		meta          *TFConfigurationMeta
	}
	type want struct {
		errMsg string
	}
	var testcases = []struct {
		name             string
		args             args
		want             want
		objects          []client.Object
		deletedResources []client.Object
		keptResources    []client.Object
	}{
		{
			name: "dispatch destroy job, subresource not cleanup, job not completed",
			args: args{
				configuration: baseConfiguration,
				meta:          &baseMeta,
			},
			want: want{
				errMsg: types.MessageDestroyJobNotCompleted,
			},
			objects:       []client.Object{readyProvider, baseConfiguration, baseConfigurationCM},
			keptResources: []client.Object{baseConfigurationCM},
		},
		{
			name: "provider is not ready, cloud resource couldn't be created, delete directly",
			args: args{
				configuration: baseConfiguration,
				meta:          &baseMeta,
			},
			objects:          []client.Object{notReadyProvider, baseConfiguration, baseConfigurationCM},
			deletedResources: []client.Object{baseConfigurationCM},
			want:             want{},
		},
		{
			name: "referenced provider not exist, allow to delete directly",
			args: args{
				configuration: configurationPrdNotFound,
				meta:          &baseMeta,
			},
			objects:          []client.Object{readyProvider, baseConfiguration, baseConfigurationCM, baseVariableSecret},
			want:             want{},
			deletedResources: []client.Object{baseConfigurationCM, baseVariableSecret},
		},
		{
			name: "destroy job has been completed, and cleanup resources",
			args: args{
				configuration: configurationWithConnSecret,
				meta:          &baseMeta,
			},
			want:             want{},
			objects:          []client.Object{readyProvider, configurationWithConnSecret, baseConfigurationCM, completeDestroyJob, baseVariableSecret, connectionSecret},
			deletedResources: []client.Object{baseConfigurationCM, completeDestroyJob, baseVariableSecret, connectionSecret},
		},
		{
			name: "destroy job has been completed, and cleanup resources except for the backend secret",
			args: args{
				configuration: configurationWithConnSecret,
				meta:          &metaWithDeleteResourceIsFalse,
			},
			want:             want{},
			objects:          []client.Object{readyProvider, configurationWithConnSecret, baseConfigurationCM, completeDestroyJob, baseVariableSecret, connectionSecret, backendSecret},
			deletedResources: []client.Object{baseConfigurationCM, completeDestroyJob, baseVariableSecret, connectionSecret},
			keptResources:    []client.Object{backendSecret},
		},
		{
			name: "force delete configuration",
			args: args{
				configuration: forceDeleteConfiguration,
				meta:          &baseMeta,
			},
			objects:          []client.Object{readyProvider, forceDeleteConfiguration, baseConfigurationCM, completeDestroyJob, baseVariableSecret},
			deletedResources: []client.Object{baseConfigurationCM, completeDestroyJob, baseVariableSecret},
		},
		{
			name: "compatible to clean up resources when upgrade controller with --controller-namespace",
			args: args{
				configuration: baseConfiguration,
				meta:          &metaWithLegacyResource,
			},
			objects:          []client.Object{baseConfiguration, variableSecretInLegacyNS, ConfigurationCMInLegacyNS, applyJobInLegacyNS},
			deletedResources: []client.Object{variableSecretInLegacyNS, ConfigurationCMInLegacyNS, applyJobInLegacyNS},
			want:             want{},
		},
	}
	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			// Prepare for test. 1. Build reconciler with fake client
			k8sClient := fake.NewClientBuilder().WithScheme(s).WithObjects(tc.objects...).Build()
			reconciler := &ConfigurationReconciler{
				Client: k8sClient,
			}
			// 2. Set meta backend
			tc.args.meta.Backend = &backend.K8SBackend{
				Client:       k8sClient,
				SecretSuffix: secretSuffix,
				SecretNS:     secretNS,
			}

			err := reconciler.terraformDestroy(ctx, *tc.args.configuration, tc.args.meta)
			if tc.want.errMsg != "" {
				if err == nil {
					t.Errorf("expected error: %s, got nil", tc.want.errMsg)
				} else if !strings.Contains(err.Error(), tc.want.errMsg) {
					t.Errorf("expected error containing %q, got %q", tc.want.errMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			}
			for _, rsc := range tc.keptResources {
				err = k8sClient.Get(ctx, client.ObjectKey{Namespace: rsc.GetNamespace(), Name: rsc.GetName()}, rsc)
				assert.NoError(t, err, "resource should be kept %s/%s", rsc.GetNamespace(), rsc.GetName())
			}
			for _, rsc := range tc.deletedResources {
				err = k8sClient.Get(ctx, client.ObjectKey{Namespace: rsc.GetNamespace(), Name: rsc.GetName()}, rsc)
				assert.True(t, apierrors.IsNotFound(err), "resource %s/%s should be deleted", rsc.GetNamespace(), rsc.GetName())
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
		RemoteGit:           "g",
	}
	job := meta.assembleTerraformJob(TerraformApply)
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
		RemoteGit:           "g",
		JobNodeSelector:     map[string]string{"ssd": "true"},
	}

	job := meta.assembleTerraformJob(TerraformApply)
	spec := job.Spec.Template.Spec
	assert.Equal(t, spec.NodeSelector, map[string]string{"ssd": "true"})
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
		RemoteGit:           "g",

		ResourceQuota: ResourceQuota{
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

	job := meta.assembleTerraformJob(TerraformApply)
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
		RemoteGit:           "g",
		GitCredentialsSecretReference: &corev1.SecretReference{
			Namespace: "default",
			Name:      "git-ssh",
		},
	}

	job := meta.assembleTerraformJob(TerraformApply)
	spec := job.Spec.Template.Spec

	var gitSecretDefaultMode int32 = 0400
	gitAuthSecretVolume := corev1.Volume{Name: GitAuthConfigVolumeName}
	gitAuthSecretVolume.Secret = &corev1.SecretVolumeSource{
		SecretName:  "git-ssh",
		DefaultMode: &gitSecretDefaultMode,
	}

	gitSecretVolumeMount := corev1.VolumeMount{
		Name:      GitAuthConfigVolumeName,
		MountPath: GitAuthConfigVolumeMountPath,
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
		RemoteGit:           "g",
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

	job := meta.assembleTerraformJob(TerraformApply)
	spec := job.Spec.Template.Spec

	var terraformSecretDefaultMode int32 = 0400

	terraformRegistryConfigMapVolume := corev1.Volume{Name: TerraformRCConfigVolumeName}
	terraformRegistryConfigMapVolume.ConfigMap = &corev1.ConfigMapVolumeSource{
		LocalObjectReference: corev1.LocalObjectReference{
			Name: "terraform-registry-config",
		},
		DefaultMode: &terraformSecretDefaultMode,
	}

	terraformRegistryConfigVolumeMount := corev1.VolumeMount{
		Name:      TerraformRCConfigVolumeName,
		MountPath: TerraformRCConfigVolumeMountPath,
	}
	terraformCredentialsSecretVolume := corev1.Volume{Name: TerraformCredentialsConfigVolumeName}
	terraformCredentialsSecretVolume.Secret = &corev1.SecretVolumeSource{
		SecretName:  "terraform-credentials",
		DefaultMode: &terraformSecretDefaultMode,
	}

	terraformCredentialsSecretVolumeMount := corev1.VolumeMount{
		Name:      TerraformCredentialsConfigVolumeName,
		MountPath: TerraformCredentialsConfigVolumeMountPath,
	}

	terraformCredentialsHelperConfigVolume := corev1.Volume{Name: TerraformCredentialsHelperConfigVolumeName}
	terraformCredentialsHelperConfigVolume.ConfigMap = &corev1.ConfigMapVolumeSource{
		LocalObjectReference: corev1.LocalObjectReference{
			Name: "terraform-credentials-helper",
		},
		DefaultMode: &terraformSecretDefaultMode,
	}

	terraformCredentialsHelperConfigVolumeMount := corev1.VolumeMount{
		Name:      TerraformCredentialsHelperConfigVolumeName,
		MountPath: TerraformCredentialsHelperConfigVolumeMountPath,
	}

	assert.Contains(t, spec.InitContainers[0].VolumeMounts, terraformCredentialsHelperConfigVolumeMount)
	assert.Contains(t, spec.Volumes, terraformCredentialsHelperConfigVolume)

	assert.Contains(t, spec.InitContainers[0].VolumeMounts, terraformRegistryConfigVolumeMount)
	assert.Contains(t, spec.Volumes, terraformRegistryConfigMapVolume)

	assert.Contains(t, spec.InitContainers[0].VolumeMounts, terraformCredentialsSecretVolumeMount)
	assert.Contains(t, spec.Volumes, terraformCredentialsSecretVolume)

	assert.Contains(t, spec.InitContainers[0].VolumeMounts, terraformRegistryConfigVolumeMount)
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
		RemoteGit:           "g",
		TerraformRCConfigMapReference: &corev1.SecretReference{
			Namespace: "default",
			Name:      "terraform-registry-config",
		},
		TerraformCredentialsHelperConfigMapReference: &corev1.SecretReference{
			Namespace: "default",
			Name:      "terraform-credentials-helper",
		},
	}

	job := meta.assembleTerraformJob(TerraformApply)
	spec := job.Spec.Template.Spec

	var terraformSecretDefaultMode int32 = 0400

	terraformRegistryConfigMapVolume := corev1.Volume{Name: TerraformRCConfigVolumeName}
	terraformRegistryConfigMapVolume.ConfigMap = &corev1.ConfigMapVolumeSource{
		LocalObjectReference: corev1.LocalObjectReference{
			Name: "terraform-registry-config",
		},
		DefaultMode: &terraformSecretDefaultMode,
	}

	terraformRegistryConfigVolumeMount := corev1.VolumeMount{
		Name:      TerraformRCConfigVolumeName,
		MountPath: TerraformRCConfigVolumeMountPath,
	}
	terraformCredentialsHelperConfigVolume := corev1.Volume{Name: TerraformCredentialsHelperConfigVolumeName}
	terraformCredentialsHelperConfigVolume.ConfigMap = &corev1.ConfigMapVolumeSource{
		LocalObjectReference: corev1.LocalObjectReference{
			Name: "terraform-credentials-helper",
		},
		DefaultMode: &terraformSecretDefaultMode,
	}

	terraformCredentialsHelperConfigVolumeMount := corev1.VolumeMount{
		Name:      TerraformCredentialsHelperConfigVolumeName,
		MountPath: TerraformCredentialsHelperConfigVolumeMountPath,
	}

	assert.Contains(t, spec.InitContainers[0].VolumeMounts, terraformRegistryConfigVolumeMount)
	assert.Contains(t, spec.Volumes, terraformRegistryConfigMapVolume)

	assert.Contains(t, spec.InitContainers[0].VolumeMounts, terraformCredentialsHelperConfigVolumeMount)
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
		ctx           context.Context
		k8sClient     client.Client
		configuration v1beta2.Configuration
		meta          *TFConfigurationMeta
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
		ObjectMeta: metav1.ObjectMeta{
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
		ObjectMeta: metav1.ObjectMeta{
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
		ObjectMeta: metav1.ObjectMeta{
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
			generated := tc.args.meta.isTFStateGenerated(tc.args.ctx)
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
		ObjectMeta: metav1.ObjectMeta{
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
		ObjectMeta: metav1.ObjectMeta{
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
		ObjectMeta: metav1.ObjectMeta{
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
		ObjectMeta: metav1.ObjectMeta{
			Name:      "tfstate-default-d",
			Namespace: "default",
		},
		Type: corev1.SecretTypeOpaque,
		Data: map[string][]byte{
			"tfstate": tfStateData,
		},
	}
	oldConnectionSecret5 := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "connection-secret-d",
			Namespace: "default",
			Labels: map[string]string{
				"terraform.core.oam.dev/created-by": "terraform-controller",
				"terraform.core.oam.dev/owned-by":   "configuration5",
			},
		},
		TypeMeta: metav1.TypeMeta{Kind: "Secret"},
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
		ObjectMeta: metav1.ObjectMeta{
			Name:      "tfstate-default-e",
			Namespace: "default",
		},
		Type: corev1.SecretTypeOpaque,
		Data: map[string][]byte{
			"tfstate": tfStateData,
		},
	}
	oldConnectionSecret6 := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "connection-secret-e",
			Namespace: "default",
			Labels: map[string]string{
				"terraform.core.oam.dev/created-by":      "terraform-controller",
				"terraform.core.oam.dev/owned-by":        "configuration5",
				"terraform.core.oam.dev/owned-namespace": "default",
			},
		},
		TypeMeta: metav1.TypeMeta{Kind: "Secret"},
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

	namespaceA := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "a"}}
	namespaceB := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "b"}}
	secret7 := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "tfstate-default-f",
			Namespace: "a",
		},
		Type: corev1.SecretTypeOpaque,
		Data: map[string][]byte{
			"tfstate": tfStateData,
		},
	}
	oldConnectionSecret7 := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "connection-secret-e",
			Namespace: "default",
			Labels: map[string]string{
				"terraform.core.oam.dev/created-by":      "terraform-controller",
				"terraform.core.oam.dev/owned-by":        "configuration6",
				"terraform.core.oam.dev/owned-namespace": "a",
			},
		},
		TypeMeta: metav1.TypeMeta{Kind: "Secret"},
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
		ObjectMeta: metav1.ObjectMeta{
			Name:      "tfstate-default-d",
			Namespace: "default",
		},
		Type: corev1.SecretTypeOpaque,
		Data: map[string][]byte{
			"tfstate": tfStateData,
		},
	}
	oldConnectionSecret8 := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "connection-secret-d",
			Namespace: "default",
			Labels: map[string]string{
				"terraform.core.oam.dev/created-by": "terraform-controller",
			},
		},
		TypeMeta: metav1.TypeMeta{Kind: "Secret"},
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
		k8sClient client.Client
		state     types.ConfigurationState
		message   string
		meta      *TFConfigurationMeta
	}
	type want struct {
		errMsg string
	}
	ctx := context.Background()
	s := runtime.NewScheme()
	v1beta2.AddToScheme(s)

	configuration := &v1beta2.Configuration{
		ObjectMeta: metav1.ObjectMeta{
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
			err := tc.args.meta.updateApplyStatus(ctx, k8sClient, tc.args.state, tc.args.message)
			if tc.want.errMsg != "" || err != nil {
				if !strings.Contains(err.Error(), tc.want.errMsg) {
					t.Errorf("updateApplyStatus() error = %v, wantErr %v", err, tc.want.errMsg)
				}
			}
		})
	}
}

func TestAssembleAndTriggerJob(t *testing.T) {
	type prepare func(t *testing.T)
	type args struct {
		k8sClient     client.Client
		executionType TerraformExecutionType
		prepare
	}
	type want struct {
		errMsg string
	}
	ctx := context.Background()
	k8sClient = fake.NewClientBuilder().Build()
	meta := &TFConfigurationMeta{
		Namespace: "b",
	}

	patches := gomonkey.ApplyFunc(apiutil.GVKForObject, func(obj runtime.Object, scheme *runtime.Scheme) (schema.GroupVersionKind, error) {
		return schema.GroupVersionKind{}, apierrors.NewNotFound(schema.GroupResource{}, "")
	})
	defer patches.Reset()

	testcases := map[string]struct {
		args args
		want want
	}{
		"failed to create ServiceAccount": {
			args: args{
				executionType: TerraformApply,
			},
			want: want{
				errMsg: "failed to create ServiceAccount for Terraform executor",
			},
		},
	}
	for name, tc := range testcases {
		t.Run(name, func(t *testing.T) {
			err := meta.assembleAndTriggerJob(ctx, k8sClient, tc.args.executionType)
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
		k8sClient         client.Client
		configurationType types.ConfigurationType
		meta              *TFConfigurationMeta
	}
	type want struct {
		errMsg string
	}
	ctx := context.Background()
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
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
		//legacyApplyJobName  = "legacy-job-name"
		//legacyJobNamespace  = "legacy-job-ns"
	)

	ctx := context.Background()
	s := runtime.NewScheme()
	v1beta1.AddToScheme(s)
	v1beta2.AddToScheme(s)
	batchv1.AddToScheme(s)
	baseMeta := TFConfigurationMeta{
		ApplyJobName:        applyJobName,
		ControllerNamespace: jobNamespace,
	}
	baseJob := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      applyJobName,
			Namespace: jobNamespace,
		},
	}
	jobInCtrlNS := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
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

			err := tc.meta.getApplyJob(ctx, k8sClient, &job)
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
		credentials           map[string]string
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
					ObjectMeta: metav1.ObjectMeta{
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
					ObjectMeta: metav1.ObjectMeta{
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
					ObjectMeta: metav1.ObjectMeta{
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
	corev1.AddToScheme(scheme)
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
				errMsg: fmt.Sprintf("'%s' not in git credentials secret", GitCredsKnownHosts),
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
	neededKeys := []string{GitCredsKnownHosts, corev1.SSHAuthPrivateKey}
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
	corev1.AddToScheme(scheme)
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
				errMsg: fmt.Sprintf("'%s' not in terraform credentials secret", TerraformCredentials),
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

	neededKeys := []string{TerraformCredentials}
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
	corev1.AddToScheme(scheme)
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
				errMsg: fmt.Sprintf("'%s' not in terraformrc configuration configmap", TerraformRegistryConfig),
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

	neededKeys := []string{TerraformRegistryConfig}
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
	corev1.AddToScheme(scheme)
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

func TestCheckValidateSecretAndConfigMap(t *testing.T) {
	ctx := context.Background()
	scheme := runtime.NewScheme()
	corev1.AddToScheme(scheme)
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
					RemoteGit:           "g",
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
					RemoteGit:           "g",
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
					RemoteGit:           "g",
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
					RemoteGit:           "g",
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
			err := tc.args.meta.validateSecretAndConfigMap(ctx, k8sClient)
			if err != nil {
				assert.EqualError(t, err, tc.want.errMsg)
			}
		})
	}

}
