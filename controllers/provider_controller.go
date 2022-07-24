/*
Copyright 2021 The KubeVela Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controllers

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	crossplanetypes "github.com/oam-dev/terraform-controller/api/types/crossplane-runtime"
	"github.com/pkg/errors"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/oam-dev/terraform-controller/api/types"
	terraformv1beta1 "github.com/oam-dev/terraform-controller/api/v1beta1"
	providercred "github.com/oam-dev/terraform-controller/controllers/provider"
)

const (
	errGetCredentials = "failed to get credentials from the cloud provider"
	errSettingStatus  = "failed to set status"
)

// ProviderReconciler reconciles a Provider object
type ProviderReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=terraform.core.oam.dev,resources=providers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=terraform.core.oam.dev,resources=providers/status,verbs=get;update;patch

// Reconcile will reconcile periodically
func (r *ProviderReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	klog.InfoS("reconciling Terraform Provider...", "NamespacedName", req.NamespacedName)

	var provider terraformv1beta1.Provider

	if err := r.Get(ctx, req.NamespacedName, &provider); err != nil {
		if kerrors.IsNotFound(err) {
			err = nil
		}
		return ctrl.Result{}, err
	}

	err := func() error {
		switch provider.Spec.Credentials.Source {
		case crossplanetypes.CredentialsSourceInjectedIdentity:
			break
		case crossplanetypes.CredentialsSourceSecret:
			_, err := providercred.GetProviderCredentials(ctx, r.Client, &provider, provider.Spec.Region)
			return err
		default:
			return errors.Errorf("unsupported credentials source: %s", provider.Spec.Credentials.Source)
		}

		return nil
	}()
	if err != nil {
		klog.ErrorS(err, errGetCredentials, "Provider", req.NamespacedName)

		provider.Status.Message = fmt.Sprintf("%s: %s", errGetCredentials, err.Error())
		provider.Status = terraformv1beta1.ProviderStatus{State: types.ProviderIsNotReady}
	} else {
		provider.Status.Message = "Provider ready"
		provider.Status = terraformv1beta1.ProviderStatus{State: types.ProviderIsReady}
	}

	if updateErr := r.Status().Update(ctx, &provider); updateErr != nil {
		klog.ErrorS(updateErr, errSettingStatus, "Provider", req.NamespacedName)

		return ctrl.Result{}, errors.Wrap(updateErr, errSettingStatus)
	}

	return ctrl.Result{}, err
}

// SetupWithManager setups with a manager
func (r *ProviderReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&terraformv1beta1.Provider{}).
		Complete(r)
}
