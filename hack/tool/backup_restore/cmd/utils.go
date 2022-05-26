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
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	defaultNS string
	k8sClient client.Client
	clientSet *kubernetes.Clientset
)

func buildK8SClient(kubeFlags *genericclioptions.ConfigFlags) error {
	config, err := kubeFlags.ToRESTConfig()
	if err != nil {
		return err
	}
	defaultNS, _, err = kubeFlags.ToRawKubeConfigLoader().Namespace()
	if err != nil {
		defaultNS = "default"
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
