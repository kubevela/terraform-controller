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
	"context"
	"log"
	"os"
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const TFBackendNS = "TERRAFORM_BACKEND_NAMESPACE"

func GetAllENVs() map[string]string {
	envs := make(map[string]string)
	for _, envStr := range os.Environ() {
		kv := strings.Split(envStr, "=")
		envs[kv[0]] = kv[1]
	}
	return envs
}

func GetTFBackendNSFromDeployment() string {
	deployment := appsv1.Deployment{}
	if err := K8SClient.Get(context.Background(), client.ObjectKey{Name: "terraform-controller", Namespace: "vela-system"}, &deployment); err != nil {
		log.Printf("WARN: get terraform-controller deployment in the vela-system namesapce failed, %v", err)
		return ""
	}
	envs := deployment.Spec.Template.Spec.Containers[0].Env
	for _, env := range envs {
		if env.Name == "TERRAFORM_BACKEND_NAMESPACE" {
			return env.Value
		}
	}
	return ""
}
