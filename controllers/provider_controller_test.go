package controllers

import (
	"context"
	crossplanetypes "github.com/oam-dev/terraform-controller/api/types/crossplane-runtime"
	"github.com/oam-dev/terraform-controller/api/v1beta1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"strings"
	"testing"
)

func TestReconcile(t *testing.T) {
	r1 := &ProviderReconciler{}
	ctx := context.Background()
	s := runtime.NewScheme()
	v1beta1.AddToScheme(s)
	r1.Client = fake.NewClientBuilder().WithScheme(s).Build()

	r2 := &ProviderReconciler{}
	provider := &v1beta1.Provider{
		ObjectMeta: ctrl.ObjectMeta{
			Name:      "abc",
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
				},
			},
		},
	}
	r2.Client = fake.NewClientBuilder().WithScheme(s).WithObjects(provider).Build()

	type args struct {
		req reconcile.Request
		r   *ProviderReconciler
	}

	type want struct {
		errMsg string
	}

	req := ctrl.Request{}
	req.NamespacedName = types.NamespacedName{
		Name:      "abc",
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
