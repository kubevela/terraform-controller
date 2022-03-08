package controllers

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/agiledragon/gomonkey/v2"
	"github.com/aliyun/alibaba-cloud-sdk-go/services/sts"
	"github.com/stretchr/testify/assert"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	k8stypes "k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/oam-dev/terraform-controller/api/types"
	crossplane "github.com/oam-dev/terraform-controller/api/types/crossplane-runtime"
	"github.com/oam-dev/terraform-controller/api/v1beta1"
	"github.com/oam-dev/terraform-controller/controllers/provider"
)

func TestInitTFConfigurationMeta(t *testing.T) {
	req := ctrl.Request{}
	req.Namespace = "default"
	req.Name = "abc"

	completeConfiguration := v1beta1.Configuration{
		ObjectMeta: v1.ObjectMeta{
			Name: "abc",
		},
		Spec: v1beta1.ConfigurationSpec{
			Path: "alibaba/rds",
			Backend: &v1beta1.Backend{
				SecretSuffix: "s1",
			},
		},
	}
	completeConfiguration.Spec.ProviderReference = &crossplane.Reference{
		Name:      "xxx",
		Namespace: "default",
	}

	testcases := []struct {
		name          string
		configuration v1beta1.Configuration
		want          *TFConfigurationMeta
	}{
		{
			name: "empty configuration",
			configuration: v1beta1.Configuration{
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
				BackendSecretName: "tfstate-default-abc",
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
				BackendSecretName: "tfstate-default-s1",
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

func TestCheckProvider(t *testing.T) {
	ctx := context.Background()
	scheme := runtime.NewScheme()
	v1beta1.AddToScheme(scheme)

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
			APIVersion: "terraform.core.oam.dev/v1beta1",
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
	configuration2 := &v1beta1.Configuration{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "a",
			Namespace: "b",
		},
		Spec: v1beta1.ConfigurationSpec{
			HCL:      "c",
			Variable: variables,
		},
		Status: v1beta1.ConfigurationStatus{
			Apply: v1beta1.ConfigurationApplyStatus{
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
			Name:      fmt.Sprintf(TFBackendSecret, terraformWorkspace, "a"),
			Namespace: "vela-system",
		},
		Data: map[string][]byte{
			TerraformStateNameInSecret: stateData,
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
	configuration3 := &v1beta1.Configuration{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "a",
			Namespace:         "b",
			DeletionTimestamp: &time,
		},
		Spec: v1beta1.ConfigurationSpec{
			HCL: "c",
		},
	}
	configuration2.Spec.ProviderReference = &crossplane.Reference{
		Name:      "default",
		Namespace: "default",
	}
	r3 := &ConfigurationReconciler{}
	r3.Client = fake.NewClientBuilder().WithScheme(s).WithObjects(secret, provider, configuration3).Build()

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

func TestPreCheck(t *testing.T) {
	r := &ConfigurationReconciler{}
	ctx := context.Background()
	s := runtime.NewScheme()
	v1beta1.AddToScheme(s)
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
		configuration *v1beta1.Configuration
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
			name: "configuration is invalid",
			args: args{
				r: r,
				configuration: &v1beta1.Configuration{
					ObjectMeta: v1.ObjectMeta{
						Name: "abc",
					},
					Spec: v1beta1.ConfigurationSpec{
						Remote: "aaa",
						HCL:    "bbb",
					},
				},
				meta: &TFConfigurationMeta{},
			},
			want: want{
				errMsg: "spec.JSON, spec.HCL and/or spec.Remote cloud not be set at the same time",
			},
		},
		{
			name: "configuration is valid",
			args: args{
				r: r,
				configuration: &v1beta1.Configuration{
					ObjectMeta: v1.ObjectMeta{
						Name: "abc",
					},
					Spec: v1beta1.ConfigurationSpec{
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
			name: "could not find provider",
			args: args{
				r: r,
				configuration: &v1beta1.Configuration{
					ObjectMeta: v1.ObjectMeta{
						Name: "abc",
					},
					Spec: v1beta1.ConfigurationSpec{
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

func TestPreCheckWhenConfigurationIsChanged(t *testing.T) {
	r := &ConfigurationReconciler{}
	ctx := context.Background()
	s := runtime.NewScheme()
	v1beta1.AddToScheme(s)
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
		configuration *v1beta1.Configuration
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
				configuration: &v1beta1.Configuration{
					ObjectMeta: v1.ObjectMeta{
						Name: "abc",
					},
					Spec: v1beta1.ConfigurationSpec{
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
	configuration := &v1beta1.Configuration{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "default",
			Name:      "b",
		},
	}
	k8sClient2 := fake.NewClientBuilder().WithScheme(s).WithObjects(provider1, configuration).Build()
	r2.Client = k8sClient2

	//r3 := &ConfigurationReconciler{}
	//provider1.Status.State = types.ProviderIsReady
	//job3 := &batchv1.Job{
	//	ObjectMeta: metav1.ObjectMeta{
	//		Name:      "a",
	//		Namespace: "default",
	//	},
	//	Status: batchv1.JobStatus{
	//		Succeeded: int32(1),
	//	},
	//}
	//configuration3 := &v1beta1.Configuration{
	//	ObjectMeta: metav1.ObjectMeta{
	//		Namespace: "default",
	//		Name:      "b",
	//	},
	//}
	//configuration3.Spec.WriteConnectionSecretToReference = &crossplane.SecretReference{
	//	Name:      "b",
	//	Namespace: "default",
	//}
	//k8sClient3 := fake.NewClientBuilder().WithScheme(s).WithObjects(provider1, job3, configuration3).Build()
	//r3.Client = k8sClient3
	//meta3 := &TFConfigurationMeta{
	//	DestroyJobName: "a",
	//	Namespace:      "b",
	//	DeleteResource: true,
	//}

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
	configuration4 := &v1beta1.Configuration{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "default",
			Name:      "b",
		},
		Spec: v1beta1.ConfigurationSpec{
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
	}

	type args struct {
		r             *ConfigurationReconciler
		namespace     string
		configuration *v1beta1.Configuration
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
				configuration: &v1beta1.Configuration{},
				meta: &TFConfigurationMeta{
					ConfigurationCMName: "tf-abc",
					Namespace:           "default",
				},
			},
			want: want{
				errMsg: "The referenced provider could not be retrieved",
			},
		},
		{
			name: "provider is ready",
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
				errMsg: "The referenced provider could not be retrieved",
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
	}
	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.args.r.terraformDestroy(ctx, tc.args.namespace, *tc.args.configuration, tc.args.meta)
			if err != nil {
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
		configuration v1beta1.Configuration
		meta          *TFConfigurationMeta
	}
	type want struct {
		property map[string]v1beta1.Property
		errMsg   string
	}

	ctx := context.Background()
	k8sClient1 := fake.NewClientBuilder().Build()
	meta1 := &TFConfigurationMeta{}

	//scheme := runtime.NewScheme()
	//v1beta1.AddToScheme(scheme)
	secret2 := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "a",
			Namespace: "default",
		},
		Type: corev1.SecretTypeOpaque,
	}
	k8sClient2 := fake.NewClientBuilder().WithObjects(secret2).Build()
	meta2 := &TFConfigurationMeta{
		BackendSecretName:         "a",
		TerraformBackendNamespace: "default",
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
