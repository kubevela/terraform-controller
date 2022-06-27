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

	"github.com/oam-dev/kubevela/apis/core.oam.dev/v1beta1"
	"github.com/oam-dev/terraform-controller/api/v1beta2"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func GetConfigurationsFromApplication(ctx context.Context, applicationName string, componentNameList []string) ([]*v1beta2.Configuration, error) {
	if applicationName == "" {
		return nil, nil
	}
	application := &v1beta1.Application{}
	if err := K8SClient.Get(ctx, client.ObjectKey{Name: applicationName, Namespace: CurrentNS}, application); err != nil {
		return nil, err
	}

	componentNameListHas := func(name string) bool {
		for _, v := range componentNameList {
			if v == name {
				return true
			}
		}
		return false
	}

	configurationList := make([]*v1beta2.Configuration, 0)
	for _, component := range application.Spec.Components {
		if len(componentNameList) == 0 || componentNameListHas(component.Name) {
			configuration, err := GetConfiguration(ctx, component.Name)
			if err != nil {
				if kerrors.IsNotFound(err) {
					continue
				} else {
					return nil, err
				}
			}
			configurationList = append(configurationList, configuration)
		}
	}

	return configurationList, nil
}
