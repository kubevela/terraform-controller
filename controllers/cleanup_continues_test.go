/*
Copyright 2026 The KubeVela Authors.

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
	"errors"
	"testing"

	crossplane "github.com/oam-dev/terraform-controller/api/types/crossplane-runtime"
	"github.com/stretchr/testify/require"
	batchv1 "k8s.io/api/batch/v1"
	v1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/oam-dev/terraform-controller/api/v1beta2"
	"github.com/oam-dev/terraform-controller/controllers/configuration/backend"
	"github.com/oam-dev/terraform-controller/controllers/process"
)

// unreadableSecret fails every read of the Secret name, as an unavailable API server would.
type unreadableSecret struct {
	client.Client
	name string
}

func (u unreadableSecret) Get(ctx context.Context, key client.ObjectKey, obj client.Object) error {
	if _, ok := obj.(*v1.Secret); ok && key.Name == u.name {
		return errors.New("apiserver unavailable")
	}
	return u.Client.Get(ctx, key, obj)
}

// recordingBackend records whether the Terraform state was cleaned up.
type recordingBackend struct {
	backend.Backend
	cleaned bool
}

func (b *recordingBackend) CleanUp(context.Context) error {
	b.cleaned = true
	return nil
}

// A connection Secret that cannot be read now does not stop the apply Job and its inputs going, but the
// destroy Job and the Terraform state stay, so the next reconcile comes back through the destroy
// path and tries the Secret again before the finalizer goes.
func TestCleanUpContinuesPastAnUnreadableConnectionSecret(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, clientgoscheme.AddToScheme(scheme))
	require.NoError(t, v1beta2.AddToScheme(scheme))
	cm := &v1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "tf-cm", Namespace: "default"}}
	destroyJob := &batchv1.Job{ObjectMeta: metav1.ObjectMeta{Name: "cfg-destroy", Namespace: "default"}}
	cli := unreadableSecret{Client: fake.NewClientBuilder().WithScheme(scheme).WithObjects(cm, destroyJob).Build(), name: "conn"}
	configuration := v1beta2.Configuration{
		ObjectMeta: metav1.ObjectMeta{Name: "cfg", Namespace: "default"},
		Spec: v1beta2.ConfigurationSpec{
			WriteConnectionSecretToReference: &crossplane.SecretReference{Name: "conn", Namespace: "default"},
		},
	}
	state := &recordingBackend{}
	meta := &process.TFConfigurationMeta{ConfigurationCMName: "tf-cm", DestroyJobName: "cfg-destroy", Namespace: "default", ControllerNamespace: "default", Backend: state, DeleteResource: true}
	r := &ConfigurationReconciler{Client: cli}

	err := r.cleanUpSubResources(context.Background(), configuration, meta)
	require.Error(t, err)
	require.Contains(t, err.Error(), "apiserver unavailable")
	require.True(t, kerrors.IsNotFound(cli.Get(context.Background(), client.ObjectKeyFromObject(cm), &v1.ConfigMap{})), "the apply Job's inputs are still cleaned up")
	require.NoError(t, cli.Get(context.Background(), client.ObjectKeyFromObject(destroyJob), &batchv1.Job{}), "the destroy Job stays")
	require.False(t, state.cleaned, "the Terraform state stays")
}
