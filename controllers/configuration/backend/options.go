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

package backend

import (
	"context"

	v1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// OptionSource contains all the sources from where the backendInitFunc can get options to build the Backend
type OptionSource struct {
	Envs []v1.EnvVar
	// add more sources if needed
}

type buildBackendOptions struct {
	configurationNS   string
	k8sClient         client.Client
	backendConf       interface{}
	extraOptionSource *OptionSource
}

func (source OptionSource) getOption(ctx context.Context, k8sClient client.Client, namespace string, key string) (string, bool, error) {
	var (
		option v1.EnvVar
		found  bool
	)
	for _, v := range source.Envs {
		if v.Name == key {
			option, found = v, true
			break
		}
	}
	if !found {
		// try other sources
		goto otherSources
	}

	if option.Value != "" {
		return option.Value, true, nil
	}

	switch {
	case option.ValueFrom.SecretKeyRef != nil:
		secretKeyRef := option.ValueFrom.SecretKeyRef
		secret := &v1.Secret{}
		if err := k8sClient.Get(ctx, client.ObjectKey{Name: secretKeyRef.Name, Namespace: namespace}, secret); err != nil {
			return "", false, err
		}
		value := secret.Data[secretKeyRef.Key]
		if len(value) == 0 {
			return "", false, nil
		}
		return string(value), true, nil

	case option.ValueFrom.ConfigMapKeyRef != nil:
		configMapKeyRef := option.ValueFrom.ConfigMapKeyRef
		configMap := &v1.ConfigMap{}
		if err := k8sClient.Get(ctx, client.ObjectKey{Name: configMapKeyRef.Name, Namespace: namespace}, configMap); err != nil {
			return "", false, err
		}
		value := configMap.Data[configMapKeyRef.Key]
		if value == "" {
			return "", false, nil
		}
		return value, true, nil

	default:
		return "", false, nil
	}

otherSources:
	return "", false, nil
}
