package controllers

import (
	crossplane "github.com/oam-dev/terraform-controller/api/types/crossplane-runtime"
	"github.com/oam-dev/terraform-controller/api/v1beta1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"reflect"
	ctrl "sigs.k8s.io/controller-runtime"
	"testing"
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
