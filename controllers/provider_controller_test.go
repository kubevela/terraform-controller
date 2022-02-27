package controllers

import (
	"context"
	"github.com/go-yaml/yaml"
	crossplanetypes "github.com/oam-dev/terraform-controller/api/types/crossplane-runtime"
	"github.com/oam-dev/terraform-controller/api/v1beta1"
	"github.com/oam-dev/terraform-controller/controllers/provider"
	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"strings"
	"testing"
	"time"
)

func TestReconcile(t *testing.T) {
	r1 := &ProviderReconciler{}
	ctx := context.Background()
	s := runtime.NewScheme()
	v1beta1.AddToScheme(s)
	v1.AddToScheme(s)
	r1.Client = fake.NewClientBuilder().WithScheme(s).Build()

	r2 := &ProviderReconciler{}
	provider2 := &v1beta1.Provider{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "aws",
			Namespace: "default",
		},
		Spec: v1beta1.ProviderSpec{
			Credentials: v1beta1.ProviderCredentials{
				Source: "Secret",
				SecretRef: &crossplanetypes.SecretKeySelector{
					SecretReference: crossplanetypes.SecretReference{
						Name:      "abc",
						Namespace: "default",
					},
					Key: "credentials",
				},
			},
			Provider: "aws",
		},
	}

	creds, _ := yaml.Marshal(&provider.AWSCredentials{
		AWSAccessKeyID:     "a",
		AWSSecretAccessKey: "b",
		AWSSessionToken:    "c",
	})
	secret2 := &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "abc",
			Namespace: "default",
		},
		Data: map[string][]byte{
			"credentials": creds,
		},
		Type: v1.SecretTypeOpaque,
	}

	r2.Client = fake.NewClientBuilder().WithScheme(s).WithObjects(secret2, provider2).Build()

	r3 := &ProviderReconciler{}
	provider3 := &v1beta1.Provider{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "aws",
			Namespace: "default",
		},
		Spec: v1beta1.ProviderSpec{
			Credentials: v1beta1.ProviderCredentials{
				Source: "Secret",
				SecretRef: &crossplanetypes.SecretKeySelector{
					SecretReference: crossplanetypes.SecretReference{
						Name:      "abc",
						Namespace: "default",
					},
					Key: "credentials",
				},
			},
			Provider: "aws",
		},
	}

	r3.Client = fake.NewClientBuilder().WithScheme(s).WithObjects(provider3).Build()

	type args struct {
		req reconcile.Request
		r   *ProviderReconciler
	}

	type want struct {
		errMsg string
	}

	req := ctrl.Request{}
	req.NamespacedName = types.NamespacedName{
		Name:      "aws",
		Namespace: "default",
	}

	testcases := []struct {
		name string
		args args
		want want
	}{
		{
			name: "Provider is not found",
			args: args{
				req: req,
				r:   r1,
			},
		},
		{
			name: "Provider is found",
			args: args{
				req: req,
				r:   r2,
			},
			want: want{},
		},
		{
			name: "Provider is found, but the secret is not available",
			args: args{
				req: req,
				r:   r3,
			},
			want: want{
				errMsg: errGetCredentials,
			},
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := tc.args.r.Reconcile(ctx, tc.args.req); (tc.want.errMsg != "") &&
				!strings.Contains(err.Error(), tc.want.errMsg) {
				t.Errorf("Reconcile() error = %v, wantErr %v", err, tc.want.errMsg)
			}
		})
	}
}

func TestSetupWithManager(t *testing.T) {
	syncPeriod := time.Duration(10) * time.Second
	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:             runtime.NewScheme(),
		MetricsBindAddress: ":1234",
		Port:               5678,
		LeaderElection:     false,
		SyncPeriod:         &syncPeriod,
	})
	assert.Nil(t, err)
	r := &ProviderReconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
	}
	err = r.SetupWithManager(mgr)
	assert.NotNil(t, err)
}
