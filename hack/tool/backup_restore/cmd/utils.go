/*
Copyright 2022 The KubeVela Authors.

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

package cmd

import (
	"bytes"
	"compress/gzip"
	"os"

	"github.com/oam-dev/terraform-controller/api/v1beta2"
	"github.com/oam-dev/terraform-controller/controllers/configuration/backend"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer/json"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	currentNS string
	k8sClient client.Client
	clientSet *kubernetes.Clientset
)

func buildK8SClient(kubeFlags *genericclioptions.ConfigFlags) error {
	config, err := kubeFlags.ToRESTConfig()
	if err != nil {
		return err
	}
	currentNS, _, err = kubeFlags.ToRawKubeConfigLoader().Namespace()
	if err != nil {
		currentNS = "default"
	}

	clientSet, err = kubernetes.NewForConfig(config)
	if err != nil {
		return err
	}

	k8sClient, err = client.New(config, client.Options{})
	if err != nil {
		return err
	}
	schema := k8sClient.Scheme()
	_ = v1beta2.AddToScheme(schema)

	return nil
}

func cleanUpConfiguration(configuration *v1beta2.Configuration) {
	configuration.ManagedFields = nil
	configuration.CreationTimestamp = v1.Time{}
	configuration.Finalizers = nil
	configuration.Generation = 0
	configuration.ResourceVersion = ""
	configuration.UID = ""
}

func getSecretName(k *backend.K8SBackend) string {
	return "tfstate-default-" + k.SecretSuffix
}

func buildSerializer() *json.Serializer {
	scheme := runtime.NewScheme()
	return json.NewSerializerWithOptions(json.DefaultMetaFactory, scheme, scheme, json.SerializerOptions{Yaml: true})
}

// Modified based on Hashicorp code base https://github.com/hashicorp/terraform/blob/fabdf0bea1fa2bf6a9d56cc3ea0f28242bf5e812/backend/remote-state/kubernetes/client.go#L355
// Licensed under Mozilla Public License 2.0
func decompressTRState(data string) ([]byte, error) {
	b := new(bytes.Buffer)
	gz, err := gzip.NewReader(bytes.NewReader([]byte(data)))
	if err != nil {
		return nil, err
	}
	if _, err := b.ReadFrom(gz); err != nil {
		return nil, err
	}
	if err := gz.Close(); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}

// Modified based on Hashicorp code base https://github.com/hashicorp/terraform/blob/fabdf0bea1fa2bf6a9d56cc3ea0f28242bf5e812/backend/remote-state/kubernetes/client.go#L343
// Licensed under Mozilla Public License 2.0
func compressedTFState() ([]byte, error) {
	srcBytes, err := os.ReadFile(stateJSONPath)
	var buf bytes.Buffer
	writer := gzip.NewWriter(&buf)
	_, err = writer.Write(srcBytes)
	if err != nil {
		return nil, err
	}
	if err := writer.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
