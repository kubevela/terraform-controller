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

package app

import (
	"github.com/oam-dev/kubevela/apis/core.oam.dev/v1beta1"
	"github.com/oam-dev/terraform-controller/api/v1beta2"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	currentNS string
	K8SClient client.Client
	clientSet *kubernetes.Clientset
)

func BuildK8SClient(kubeFlags *genericclioptions.ConfigFlags) error {
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

	K8SClient, err = client.New(config, client.Options{})
	if err != nil {
		return err
	}

	scheme := K8SClient.Scheme()
	// scheme of Configuration
	_ = v1beta2.AddToScheme(scheme)
	// scheme of Application
	_ = v1beta1.AddToScheme(scheme)

	return nil
}
