package process

import (
	"context"
	"encoding/base64"
	"testing"

	crossplane "github.com/oam-dev/terraform-controller/api/types/crossplane-runtime"
	"github.com/oam-dev/terraform-controller/api/v1beta2"
	"github.com/oam-dev/terraform-controller/controllers/configuration/backend"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

// A Configuration writes its outputs only into a connection Secret it created: one that exists
// without this Configuration's ownership labels belongs to someone else and is left alone.
func TestOutputsAreNotWrittenIntoASecretAConfigurationDoesNotOwn(t *testing.T) {
	tfStateData, _ := base64.StdEncoding.DecodeString("H4sIAAAAAAAA/4SQzarbMBCF934KoXUdPKNf+1VKCWNp5AocO8hyaSl592KlcBd3cZfnHPHpY/52QshfXI68b3IS+tuVK5dCaS+P+8ci4TbcULb94JJplZPAFte8MS18PQrKBO8Q+xk59SHa1AMA9M4YmoN3FGJ8M/azPs96yElcCkLIsG+V8sblnqOc3uXlRuvZ0GxSSuiCRUYbw2gGHRFGPxitEgJYQDQ0a68I2ChNo1cAZJ2bR20UtW8bsv55NuJRS94W2erXe5X5QQs3A/FZ4fhJaOwUgZTVMRjto1HGpSGSQuuD955hdDDPcR6NY1ZpQJ/YwagTRAvBpsi8LXn7Pa1U+ahfWHX/zWThYz9L4Otg3390r+5fAAAA//8hmcuNuQEAAA==")
	state := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "tfstate-default-a", Namespace: "team-a"},
		Data:       map[string][]byte{"tfstate": tfStateData},
	}
	victim := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "protected", Namespace: "victim"},
		Data:       map[string][]byte{"password": []byte("keep-me")},
	}
	k8sClient := fake.NewClientBuilder().WithObjects(state, victim).Build()
	configuration := v1beta2.Configuration{
		ObjectMeta: metav1.ObjectMeta{Name: "attacker", Namespace: "team-a"},
		Spec: v1beta2.ConfigurationSpec{
			WriteConnectionSecretToReference: &crossplane.SecretReference{Name: "protected", Namespace: "victim"},
		},
	}
	meta := &TFConfigurationMeta{Backend: &backend.K8SBackend{Client: k8sClient, SecretSuffix: "a", SecretNS: "team-a"}}

	_, err := meta.getTFOutputs(context.Background(), k8sClient, configuration)
	assert.ErrorContains(t, err, "cannot update secret")
	got := &corev1.Secret{}
	assert.NoError(t, k8sClient.Get(context.Background(), client.ObjectKeyFromObject(victim), got))
	assert.Equal(t, "keep-me", string(got.Data["password"]))
}

func TestAConnectionSecretIsOwnedByTheConfigurationNamedInItsLabels(t *testing.T) {
	configuration := &v1beta2.Configuration{ObjectMeta: metav1.ObjectMeta{Name: "db", Namespace: "team-a"}}
	labelled := func(labels map[string]string) *corev1.Secret {
		return &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Labels: labels}}
	}
	owner := map[string]string{
		"terraform.core.oam.dev/created-by":      "terraform-controller",
		"terraform.core.oam.dev/owned-by":        "db",
		"terraform.core.oam.dev/owned-namespace": "team-a",
	}
	assert.True(t, OwnsConnectionSecret(configuration, labelled(owner)))
	assert.False(t, OwnsConnectionSecret(configuration, labelled(nil)), "an unlabelled Secret is no Configuration's")
	for key, other := range map[string]string{
		"terraform.core.oam.dev/created-by":      "someone-else",
		"terraform.core.oam.dev/owned-by":        "other",
		"terraform.core.oam.dev/owned-namespace": "team-b",
	} {
		l := map[string]string{}
		for k, v := range owner {
			l[k] = v
		}
		l[key] = other
		assert.False(t, OwnsConnectionSecret(configuration, labelled(l)), key)
	}
}
