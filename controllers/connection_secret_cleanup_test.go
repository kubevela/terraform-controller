package controllers

import (
	"context"
	"testing"

	crossplane "github.com/oam-dev/terraform-controller/api/types/crossplane-runtime"
	"github.com/oam-dev/terraform-controller/api/v1beta2"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

// Deleting a Configuration deletes the connection Secret it created, and no other: a Secret its
// spec names that another Configuration, or nobody here, created is left in place.
func TestDeletingAConfigurationDeletesOnlyItsOwnConnectionSecret(t *testing.T) {
	ownedBy := func(name, namespace string) map[string]string {
		return map[string]string{
			"terraform.core.oam.dev/created-by":      "terraform-controller",
			"terraform.core.oam.dev/owned-by":        name,
			"terraform.core.oam.dev/owned-namespace": namespace,
		}
	}
	configuration := func(secret, namespace string) v1beta2.Configuration {
		return v1beta2.Configuration{
			ObjectMeta: metav1.ObjectMeta{Name: "attacker", Namespace: "team-a"},
			Spec: v1beta2.ConfigurationSpec{
				WriteConnectionSecretToReference: &crossplane.SecretReference{Name: secret, Namespace: namespace},
			},
		}
	}
	for name, tc := range map[string]struct {
		labels map[string]string
		kept   bool
	}{
		"a Secret nobody here created":               {labels: nil, kept: true},
		"another Configuration's Secret":             {labels: ownedBy("db", "victim"), kept: true},
		"a same-named Configuration's in another ns": {labels: ownedBy("attacker", "victim"), kept: true},
		"its own Secret":                             {labels: ownedBy("attacker", "team-a"), kept: false},
	} {
		t.Run(name, func(t *testing.T) {
			secret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "conn", Namespace: "victim", Labels: tc.labels}}
			k8sClient := fake.NewClientBuilder().WithObjects(secret).Build()
			assert.NoError(t, deleteConnectionSecret(context.Background(), k8sClient, configuration("conn", "victim")))
			err := k8sClient.Get(context.Background(), client.ObjectKeyFromObject(secret), &corev1.Secret{})
			assert.Equal(t, tc.kept, err == nil)
		})
	}
}
