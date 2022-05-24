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
	"github.com/oam-dev/terraform-controller/controllers/configuration/backend"
	"github.com/stretchr/testify/assert"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
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
			meta := initTFConfigurationMeta(req, tc.configuration)
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
			name: "DeleteResource is false",
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
			name: "DeleteResource is true",
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
			meta := initTFConfigurationMeta(req, tc.configuration)
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
	meta := initTFConfigurationMeta(req, configuration)
	assert.Equal(t, meta.JobNodeSelector, map[string]string{"ssd": "true"})
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

	stateData, _ := base64.StdEncoding.DecodeString("H4sIAAAAAAAA/0SMwa7CIBBF9/0KMutH80ArDb9ijKHDYEhqMQO4afrvBly4POfc3H0QAt7EOaYNrDj/NS7E7ELi5/1XQI3/o4beM3F0K1ihO65xI/egNsLThLPRWi6agkR/CVIppaSZJrfgbBx6//1ItbxqyWDFfnTBlFNlpKaut+EYPgEAAP//xUXpvZsAAAA=")

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
	r4.Client = fake.NewClientBuilder().WithScheme(s).WithObjects(secret, provider, configuration4, destroyJob4).Build()

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

	type args struct {
		req reconcile.Request
		r   *ConfigurationReconciler
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
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			for i := 0; i < 5; i++ {
				if _, err := tc.args.r.Reconcile(ctx, tc.args.req); tc.want.errMsg != "" &&
					!strings.Contains(err.Error(), tc.want.errMsg) {
					t.Errorf("Reconcile() error = %v, wantErr %v", err, tc.want.errMsg)
				}
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
	r1 := &ConfigurationReconciler{}
	ctx := context.Background()
	s := runtime.NewScheme()
	v1beta1.AddToScheme(s)
	v1beta2.AddToScheme(s)
	corev1.AddToScheme(s)
	batchv1.AddToScheme(s)
	rbacv1.AddToScheme(s)
	provider1 := &v1beta1.Provider{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "default",
			Namespace: "default",
		},
		Status: v1beta1.ProviderStatus{
			State: types.ProviderIsNotReady,
		},
	}
	k8sClient1 := fake.NewClientBuilder().WithScheme(s).WithObjects(provider1).Build()
	r1.Client = k8sClient1

	r2 := &ConfigurationReconciler{}
	provider1.Status.State = types.ProviderIsReady
	configuration := &v1beta2.Configuration{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "default",
			Name:      "b",
		},
	}
	k8sClient2 := fake.NewClientBuilder().WithScheme(s).WithObjects(provider1, configuration).Build()
	r2.Client = k8sClient2

	r4 := &ConfigurationReconciler{}
	provider1.Status.State = types.ProviderIsReady
	job4 := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "a",
			Namespace: "default",
		},
		Status: batchv1.JobStatus{
			Succeeded: int32(1),
		},
	}
	data, _ := json.Marshal(map[string]interface{}{
		"name": "abc",
	})
	variables := &runtime.RawExtension{Raw: data}
	configuration4 := &v1beta2.Configuration{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "default",
			Name:      "b",
		},
		Spec: v1beta2.ConfigurationSpec{
			Variable: variables,
		},
	}
	configuration4.Spec.WriteConnectionSecretToReference = &crossplane.SecretReference{
		Name:      "b",
		Namespace: "default",
	}
	secret4 := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "b",
			Namespace: "default",
		},
		Type: corev1.SecretTypeOpaque,
	}
	variableSecret4 := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "c",
			Namespace: "default",
		},
		Type: corev1.SecretTypeOpaque,
	}
	k8sClient4 := fake.NewClientBuilder().WithScheme(s).WithObjects(provider1, job4, secret4, variableSecret4, configuration4).Build()
	r4.Client = k8sClient4
	meta4 := &TFConfigurationMeta{
		DestroyJobName: "a",
		Namespace:      "default",
		DeleteResource: true,
		ProviderReference: &crossplane.Reference{
			Name:      "b",
			Namespace: "default",
		},
		VariableSecretName: "c",
		Backend: &backend.K8SBackend{
			Client:       k8sClient4,
			SecretSuffix: "a",
			SecretNS:     "default",
		},
	}

	r5 := &ConfigurationReconciler{}
	forceDeleteConfig5 := &v1beta2.Configuration{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "default",
			Name:      "force-delete-config-5",
		},
		Spec: v1beta2.ConfigurationSpec{},
	}
	var forceDelete = true
	forceDeleteConfig5.Spec.ForceDelete = &forceDelete
	k8sClient5 := fake.NewClientBuilder().WithScheme(s).WithObjects(forceDeleteConfig5).Build()
	r5.Client = k8sClient5

	type args struct {
		r             *ConfigurationReconciler
		namespace     string
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
			name: "provider is not ready",
			args: args{
				r:             r1,
				configuration: &v1beta2.Configuration{},
				meta: &TFConfigurationMeta{
					ConfigurationCMName: "tf-abc",
					Namespace:           "default",
				},
			},
			want: want{
				errMsg: "jobs.batch \"\" not found",
			},
		},
		{
			name: "referenced provider is not available",
			args: args{
				r:             r2,
				configuration: configuration,
				meta: &TFConfigurationMeta{
					ConfigurationCMName: "tf-abc",
					Namespace:           "default",
					DeleteResource:      true,
				},
			},
			want: want{
				errMsg: "jobs.batch \"\" not found",
			},
		},
		{
			name: "could not directly remove resources, and destroy job completes",
			args: args{
				r:             r4,
				configuration: configuration4,
				meta:          meta4,
			},
			want: want{},
		},
		{
			name: "force delete configuration",
			args: args{
				r:             r5,
				configuration: forceDeleteConfig5,
				meta:          &TFConfigurationMeta{},
			},
		},
	}
	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.args.r.terraformDestroy(ctx, tc.args.namespace, *tc.args.configuration, tc.args.meta)
			if err != nil || tc.want.errMsg != "" {
				if !strings.Contains(err.Error(), tc.want.errMsg) {
					t.Errorf("terraformDestroy() error = %v, wantErr %v", err, tc.want.errMsg)
					return
				}
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

		ResourcesLimitsCPU:              "10m",
		ResourcesLimitsCPUQuantity:      quantityLimitsCPU,
		ResourcesLimitsMemory:           "10Mi",
		ResourcesLimitsMemoryQuantity:   quantityLimitsMemory,
		ResourcesRequestsCPU:            "100m",
		ResourcesRequestsCPUQuantity:    quantityRequestsCPU,
		ResourcesRequestsMemory:         "5Gi",
		ResourcesRequestsMemoryQuantity: quantityRequestsMemory,
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
		return schema.GroupVersionKind{}, kerrors.NewNotFound(schema.GroupResource{}, "")
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
				},
				configurationType: "xxx",
			},
			want: want{
				errMsg: "not found",
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
		})
	}
}
