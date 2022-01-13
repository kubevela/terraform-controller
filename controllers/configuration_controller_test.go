package controllers

import (
	"context"
	"encoding/json"
	batchv1 "k8s.io/api/batch/v1"
	"reflect"
	"strings"
	"testing"

	"github.com/agiledragon/gomonkey/v2"
	"github.com/aliyun/alibaba-cloud-sdk-go/services/sts"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
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

	configuration := &v1beta1.Configuration{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "a",
			Namespace: "b",
		},
		Spec: v1beta1.ConfigurationSpec{
			HCL: "c",
		},
	}
	configuration.Spec.ProviderReference = &crossplane.Reference{
		Name:      "default",
		Namespace: "default",
	}

	patches := gomonkey.ApplyMethod(reflect.TypeOf(&sts.Client{}), "GetCallerIdentity", func(_ *sts.Client, request *sts.GetCallerIdentityRequest) (response *sts.GetCallerIdentityResponse, err error) {
		response = nil
		err = nil
		return
	})
	defer patches.Reset()

	r2 := &ConfigurationReconciler{}
	r2.Client = fake.NewClientBuilder().WithScheme(s).WithObjects(secret, provider, configuration).Build()

	type args struct {
		req reconcile.Request
		r   *ConfigurationReconciler
	}

	type want struct {
		errMsg string
	}

	req := ctrl.Request{}
	req.NamespacedName = k8stypes.NamespacedName{
		Name:      "a",
		Namespace: "b",
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
			name: "Configuration exists",
			args: args{
				req: req,
				r:   r2,
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
	r := &ConfigurationReconciler{}
	ctx := context.Background()
	s := runtime.NewScheme()
	v1beta1.AddToScheme(s)
	corev1.AddToScheme(s)
	provider := &v1beta1.Provider{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "default",
			Namespace: "default",
		},
		Status: v1beta1.ProviderStatus{
			State: types.ProviderIsNotReady,
		},
	}
	k8sClient := fake.NewClientBuilder().WithScheme(s).WithObjects(provider).Build()
	r.Client = k8sClient
	type args struct {
		namespace     string
		configuration v1beta1.Configuration
		k8sClient     client.Client
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
				k8sClient:     k8sClient,
				configuration: v1beta1.Configuration{},
				meta: &TFConfigurationMeta{
					ConfigurationCMName: "tf-abc",
					Namespace:           "default",
				},
			},
			want: want{
				errMsg: "The referenced provider could not be retrieved",
			},
		},
	}
	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			err := r.terraformDestroy(ctx, tc.args.namespace, tc.args.configuration, tc.args.meta)
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
