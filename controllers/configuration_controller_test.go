package controllers

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/agiledragon/gomonkey/v2"
	"github.com/aliyun/alibaba-cloud-sdk-go/services/sts"
	"github.com/oam-dev/terraform-controller/api/types"
	crossplane "github.com/oam-dev/terraform-controller/api/types/crossplane-runtime"
	"github.com/oam-dev/terraform-controller/api/v1beta1"
	"github.com/oam-dev/terraform-controller/api/v1beta2"
	"github.com/oam-dev/terraform-controller/controllers/configuration/backend"
	"github.com/oam-dev/terraform-controller/controllers/process"
	"github.com/oam-dev/terraform-controller/controllers/provider"
	"github.com/stretchr/testify/assert"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	k8stypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/pointer"
	"reflect"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"strings"
	"testing"
	"time"
)

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
	meta := process.New(req, configuration, r.Client)
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
		meta          *process.TFConfigurationMeta
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
				meta: &process.TFConfigurationMeta{},
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
				meta: &process.TFConfigurationMeta{
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
				meta: &process.TFConfigurationMeta{
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
				meta: &process.TFConfigurationMeta{
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
				meta: &process.TFConfigurationMeta{
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
				meta: &process.TFConfigurationMeta{
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
				meta: &process.TFConfigurationMeta{
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
				meta: &process.TFConfigurationMeta{
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
				meta: &process.TFConfigurationMeta{
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
	meta3 := &process.TFConfigurationMeta{
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
		meta          *process.TFConfigurationMeta
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
		meta          *process.TFConfigurationMeta
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
				meta: &process.TFConfigurationMeta{
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
				meta: &process.TFConfigurationMeta{
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
				meta: &process.TFConfigurationMeta{
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
				meta: &process.TFConfigurationMeta{
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
				meta: &process.TFConfigurationMeta{
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
	ConfigurationCMInLegacyNS := baseConfigurationCM.DeepCopy()
	ConfigurationCMInLegacyNS.Namespace = legacyNamespace
	variableSecretInLegacyNS := baseVariableSecret.DeepCopy()
	variableSecretInLegacyNS.Namespace = legacyNamespace

	baseMeta := process.TFConfigurationMeta{
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
	metaWithLegacyResource.LegacySubResources = process.LegacySubResources{
		Namespace:           legacyNamespace,
		ApplyJobName:        applyJobName,
		DestroyJobName:      destroyJobName,
		ConfigurationCMName: configurationCMName,
		VariableSecretName:  fmt.Sprintf(TFVariableSecret, secretSuffix),
	}

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
		meta          *process.TFConfigurationMeta
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
			name: "destroy job has completes, cleanup resources",
			args: args{
				configuration: configurationWithConnSecret,
				meta:          &baseMeta,
			},
			want:             want{},
			objects:          []client.Object{readyProvider, configurationWithConnSecret, baseConfigurationCM, completeDestroyJob, baseVariableSecret, connectionSecret},
			deletedResources: []client.Object{baseConfigurationCM, completeDestroyJob, baseVariableSecret, connectionSecret},
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
