package controllers

import (
	"context"
	"fmt"
	"strings"
	"testing"

	. "github.com/agiledragon/gomonkey/v2"
	"github.com/pkg/errors"
	"gopkg.in/yaml.v2"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	crossplanetypes "github.com/oam-dev/terraform-controller/api/types/crossplane-runtime"
	"github.com/oam-dev/terraform-controller/api/v1beta1"
	"github.com/oam-dev/terraform-controller/controllers/provider"
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
			name: "Provider is found but the secret is not available",
			args: args{
				req: req,
				r:   r3,
			},
			want: want{
				errMsg: `failed to get the Secret from Provider: secrets "abc" not found`,
			},
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := tc.args.r.Reconcile(ctx, tc.args.req)
			if tc.want.errMsg != "" && !strings.Contains(err.Error(), tc.want.errMsg) {
				t.Errorf("Reconcile() error = %v, wantErr %v", err, tc.want.errMsg)
			}
		})
	}
}

func TestReconcileProviderIsReadyButFailedToUpdateStatus(t *testing.T) {
	ctx := context.Background()
	s := runtime.NewScheme()
	v1beta1.AddToScheme(s)
	v1.AddToScheme(s)

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

	patches := ApplyFunc(apiutil.GVKForObject, func(obj runtime.Object, scheme *runtime.Scheme) (schema.GroupVersionKind, error) {
		switch obj.(type) {
		case *v1beta1.Provider:
			p := obj.(*v1beta1.Provider)
			if p.Status.State != "" {
				return obj.GetObjectKind().GroupVersionKind(), errors.New("xxx")
			}
		}
		return apiutilGVKForObject(obj, scheme)
	})
	defer patches.Reset()

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
			name: "Provider is found",
			args: args{
				req: req,
				r:   r2,
			},
			want: want{
				errMsg: "failed to set status",
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

func TestReconcile3(t *testing.T) {
	ctx := context.Background()
	s := runtime.NewScheme()
	v1beta1.AddToScheme(s)
	v1.AddToScheme(s)

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
			Provider: errSettingStatus,
		},
	}

	r3.Client = fake.NewClientBuilder().WithScheme(s).WithObjects(provider3).Build()

	patches := ApplyFunc(apiutil.GVKForObject, func(obj runtime.Object, scheme *runtime.Scheme) (schema.GroupVersionKind, error) {
		switch obj.(type) {
		case *v1beta1.Provider:
			p := obj.(*v1beta1.Provider)
			if p.Status.State != "" {
				return obj.GetObjectKind().GroupVersionKind(), errors.New("xxx")
			}
		}
		return apiutilGVKForObject(obj, scheme)
	})
	defer patches.Reset()

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
			name: "Provider is found, but the secret is not available",
			args: args{
				req: req,
				r:   r3,
			},
			want: want{
				errMsg: errSettingStatus,
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

func apiutilGVKForObject(obj runtime.Object, scheme *runtime.Scheme) (schema.GroupVersionKind, error) {
	switch obj.(type) {
	case *v1beta1.Provider:
		p := obj.(*v1beta1.Provider)
		if p.Status.State != "" {
			return obj.GetObjectKind().GroupVersionKind(), errors.New("xxx")
		}
	}
	// a copy implementation of `apiutil.GVKForObject`
	_, isPartial := obj.(*metav1.PartialObjectMetadata) //nolint:ifshort
	_, isPartialList := obj.(*metav1.PartialObjectMetadataList)
	if isPartial || isPartialList {
		// we require that the GVK be populated in order to recognize the object
		gvk := obj.GetObjectKind().GroupVersionKind()
		if len(gvk.Kind) == 0 {
			return schema.GroupVersionKind{}, runtime.NewMissingKindErr("unstructured object has no kind")
		}
		if len(gvk.Version) == 0 {
			return schema.GroupVersionKind{}, runtime.NewMissingVersionErr("unstructured object has no version")
		}
		return gvk, nil
	}

	gvks, isUnversioned, err := scheme.ObjectKinds(obj)
	if err != nil {
		return schema.GroupVersionKind{}, err
	}
	if isUnversioned {
		return schema.GroupVersionKind{}, fmt.Errorf("cannot create group-version-kind for unversioned type %T", obj)
	}

	if len(gvks) < 1 {
		return schema.GroupVersionKind{}, fmt.Errorf("no group-version-kinds associated with type %T", obj)
	}
	if len(gvks) > 1 {
		// this should only trigger for things like metav1.XYZ --
		// normal versioned types should be fine
		return schema.GroupVersionKind{}, fmt.Errorf(
			"multiple group-version-kinds associated with type %T, refusing to guess at one", obj)
	}
	return gvks[0], nil
}
