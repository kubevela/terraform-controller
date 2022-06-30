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

	"github.com/oam-dev/kubevela/apis/core.oam.dev/v1beta1"
	crossplane "github.com/oam-dev/terraform-controller/api/types/crossplane-runtime"
	"github.com/oam-dev/terraform-controller/api/v1beta2"
	"github.com/oam-dev/terraform-controller/controllers/configuration/backend"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type ApplicationComponent struct {
	application   *v1beta1.Application
	componentName string
}

func (a ApplicationComponent) Apply(ctx context.Context) error {
	log.Println("try to restore the application......")

	if err := K8SClient.Create(ctx, a.application); err != nil {
		return err
	}

	ref := a.GetConfigurationNamespacedName()
	log.Printf("apply the application successfully, wait the configuration{Name: %s, Namespace: %s} to be available......", ref.Name, ref.Namespace)
	return nil
}

func (a ApplicationComponent) GetK8SBackend() (*backend.K8SBackend, error) {
	ns := os.Getenv("TERRAFORM_BACKEND_NAMESPACE")
	if ns == "" {
		ns = a.application.Namespace
	}
	return &backend.K8SBackend{
		Client:       K8SClient,
		HCLCode:      "",
		SecretSuffix: a.componentName,
		SecretNS:     ns,
	}, nil
}

func (a ApplicationComponent) GetConfigurationNamespacedName() *crossplane.Reference {
	return &crossplane.Reference{
		Name:      a.componentName,
		Namespace: a.application.Namespace,
	}
}

func NewApplicationComponentFromYAML(yamlPath, componentName string) (*ApplicationComponent, error) {
	applicationBytes, err := os.ReadFile(yamlPath)
	if err != nil {
		return nil, err
	}
	application := &v1beta1.Application{}
	serializer := BuildSerializer()
	if _, _, err := serializer.Decode(applicationBytes, nil, application); err != nil {
		return nil, err
	}
	if application.Namespace == "" {
		application.Namespace = currentNS
	}
	CleanUpObjectMeta(&application.ObjectMeta)
	var component string
	components := application.Spec.Components
	if componentName == "" {
		component = components[0].Name
	} else {
		for _, v := range components {
			if v.Name == componentName {
				component = v.Name
				break
			}
		}
	}
	if component == "" {
		log.Fatalf("can not find component(%s) in Applicaton{Name: %s, Namespace: %s}", componentName, application.Name, application.Namespace)
	}
	return &ApplicationComponent{
		application:   application,
		componentName: component,
	}, nil
}

func GetConfigurationsFromApplication(ctx context.Context, applicationName string, componentNameList []string) ([]*v1beta2.Configuration, error) {
	if applicationName == "" {
		return nil, nil
	}
	application := &v1beta1.Application{}
	if err := K8SClient.Get(ctx, client.ObjectKey{Name: applicationName, Namespace: currentNS}, application); err != nil {
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
