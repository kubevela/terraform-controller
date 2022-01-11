package controllers

import (
	"context"
	"github.com/oam-dev/terraform-controller/api/types"
	"reflect"
	"strings"
	"testing"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	crossplane "github.com/oam-dev/terraform-controller/api/types/crossplane-runtime"
	"github.com/oam-dev/terraform-controller/api/v1beta1"
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

	provider := &v1beta1.Provider{
		ObjectMeta: v1.ObjectMeta{
			Name:      "default",
			Namespace: "default",
		},
		Status: v1beta1.ProviderStatus{
			State: types.ProviderIsNotReady,
		},
	}
	k8sClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(provider).Build()

	meta := &TFConfigurationMeta{
		ProviderReference: &crossplane.Reference{
			Name:      "default",
			Namespace: "default",
		},
	}

	testcases := []struct {
		name string
		want string
	}{
		{
			name: "provider doesn't not exist",
			want: "failed to get Provider from Configuration",
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			if err := meta.checkProvider(ctx, k8sClient); err != nil &&
				!strings.Contains(err.Error(), tc.want) {
				t.Errorf("checkProvider = %v, want %v", err.Error(), tc.want)
			}
		})
	}
}
