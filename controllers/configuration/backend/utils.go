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
	"fmt"

	"github.com/hashicorp/hcl/v2"
	"github.com/zclconf/go-cty/cty"
	"github.com/zclconf/go-cty/cty/gocty"
	v1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type k8sContext struct {
	context.Context
	k8sClient client.Client
	namespace string
}

// OptionSource contains all the sources from where the backendInitFunc can get options to build the Backend
type OptionSource struct {
	Envs []v1.EnvVar
	// add more sources if needed
}

// ParsedBackendConfig is a struct parsed from the backend hcl block
type ParsedBackendConfig struct {
	// Name is the label of the backend hcl block
	// It means which backend type the configuration will use
	Name string `hcl:"name,label"`
	// Attrs are the key-value pairs in the backend hcl block
	Attrs hcl.Body `hcl:",remain"`
}

func (conf ParsedBackendConfig) getAttrValue(key string) (*cty.Value, error) {
	attrs, diags := conf.Attrs.JustAttributes()
	if diags.HasErrors() {
		return nil, diags
	}
	attr := attrs[key]
	if attr == nil {
		return nil, fmt.Errorf("cannot find attr %s", key)
	}
	v, diags := attr.Expr.Value(nil)
	if diags.HasErrors() {
		return nil, diags
	}
	return &v, nil
}

func (conf ParsedBackendConfig) getAttrString(key string) (string, error) {
	v, err := conf.getAttrValue(key)
	if err != nil {
		return "", err
	}
	result := ""
	err = gocty.FromCtyValue(*v, &result)
	return result, err
}

func (source OptionSource) getOption(ctx k8sContext, key string) (string, bool, error) {
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
		if err := ctx.k8sClient.Get(ctx, client.ObjectKey{Name: secretKeyRef.Name, Namespace: ctx.namespace}, secret); err != nil {
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
		if err := ctx.k8sClient.Get(ctx, client.ObjectKey{Name: configMapKeyRef.Name, Namespace: ctx.namespace}, configMap); err != nil {
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
