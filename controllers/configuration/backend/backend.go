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
	"reflect"
	"strings"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/gohcl"
	"github.com/hashicorp/hcl/v2/hclparse"
	"github.com/hashicorp/hcl/v2/hclwrite"
	"github.com/oam-dev/terraform-controller/api/v1beta2"
	"github.com/pkg/errors"
	"github.com/zclconf/go-cty/cty"
	"github.com/zclconf/go-cty/cty/gocty"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var backendInitFuncMap = map[v1beta2.BackendType]*backendInitFunc{
	"kubernetes": {
		InitFuncFromHCL:  newK8SBackendFromHCL,
		InitFuncFromConf: newK8SBackendFromConf,
	},
}

type Backend interface {
	HCL() string
	GetTFStateJSON(ctx context.Context) ([]byte, error)
	CleanUp(ctx context.Context) error
}

type backendInitFunc struct {
	InitFuncFromHCL  func(string, *ParsedBackendConfig, client.Client) (Backend, error)
	InitFuncFromConf func(string, interface{}, client.Client) (Backend, error)
}

type ParsedBackendConfig struct {
	Name   string   `hcl:"name,label"`
	Remain hcl.Body `hcl:",remain"`
}

func (conf ParsedBackendConfig) getAttrValue(key string) (*cty.Value, error) {
	attrs, diags := conf.Remain.JustAttributes()
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

// ParseConfigurationBackend parses backend Conf from the v1beta2.Configuration
func ParseConfigurationBackend(configuration *v1beta2.Configuration, k8sClient client.Client) (Backend, error) {
	backend := configuration.Spec.Backend

	switch {

	case backend == nil || (backend.Inline == "" && backend.BackendType == ""):
		// use the default k8s backend
		if configuration.Spec.Backend != nil {
			if configuration.Spec.Backend.SecretSuffix == "" {
				configuration.Spec.Backend.SecretSuffix = configuration.Name
			}
			configuration.Spec.Backend.InClusterConfig = true
		} else {
			configuration.Spec.Backend = &v1beta2.Backend{
				SecretSuffix:    configuration.Name,
				InClusterConfig: true,
			}
		}
		return newDefaultK8SBackend(configuration.Spec.Backend.SecretSuffix, k8sClient), nil

	case backend.Inline != "" && backend.BackendType != "":
		return nil, errors.New("it's not allowed to set `spec.backend.inline` and `spec.backend.backendType` at the same time")

	case backend.Inline != "":
		// In this case, use the inline custom backend
		return handleInlineBackendHCL(backend.Inline, k8sClient)

	case backend.BackendType != "":
		// In this case, use the explicit custom backend

		// first, check if is valid custom backend
		backendType := backend.BackendType
		// fetch backendConfValue using reflection
		backendStructValue := reflect.ValueOf(backend)
		if backendStructValue.Kind() == reflect.Ptr {
			backendStructValue = backendStructValue.Elem()
		}
		backendField := backendStructValue.FieldByNameFunc(func(name string) bool {
			return strings.EqualFold(name, string(backendType))
		})
		if backendField.IsNil() {
			return nil, fmt.Errorf("there is no configuration for backendType %s", backend.BackendType)
		}
		backendConfValue := backendField.Interface()

		// second, handle the backendConf
		return handleExplicitBackend(backendConfValue, backendType, k8sClient)
	}

	return nil, nil
}

func handleInlineBackendHCL(code string, k8sClient client.Client) (Backend, error) {

	type BackendConfigWrap struct {
		Backend ParsedBackendConfig `hcl:"backend,block"`
	}

	type TerraformConfig struct {
		Remain    interface{} `hcl:",remain"`
		Terraform struct {
			Remain  interface{}         `hcl:",remain"`
			Backend ParsedBackendConfig `hcl:"backend,block"`
		} `hcl:"terraform,block"`
	}

	hclFile, diags := hclparse.NewParser().ParseHCL([]byte(code), "backend")
	if diags.HasErrors() {
		return nil, fmt.Errorf("there are syntax errors in the inline backend hcl code: %w", diags)
	}

	// try to parse hclFile to Config or BackendConfig
	config := &TerraformConfig{}
	// nolint:staticcheck
	backendConfig := &ParsedBackendConfig{}
	shouldWrap := false
	diags = gohcl.DecodeBody(hclFile.Body, nil, config)
	if diags.HasErrors() || config.Terraform.Backend.Name == "" {
		backendConfigWrap := &BackendConfigWrap{}
		diags = gohcl.DecodeBody(hclFile.Body, nil, backendConfigWrap)
		if diags.HasErrors() || backendConfigWrap.Backend.Name == "" {
			return nil, fmt.Errorf("the inline backend hcl code is not valid Terraform backend configuration: %w", diags)
		}
		shouldWrap = true
		backendConfig = &backendConfigWrap.Backend
	} else {
		backendConfig = &config.Terraform.Backend
	}

	if shouldWrap {
		code = fmt.Sprintf(`
terraform {
%s
}
`, code)
	}
	initFunc := backendInitFuncMap[v1beta2.BackendType(backendConfig.Name)]
	if initFunc == nil || initFunc.InitFuncFromHCL == nil {
		return nil, fmt.Errorf("backend type (%s) is not supported", backendConfig.Name)
	}
	return initFunc.InitFuncFromHCL(code, backendConfig, k8sClient)
}

func handleExplicitBackend(backendConf interface{}, backendType v1beta2.BackendType, k8sClient client.Client) (Backend, error) {
	hclFile := hclwrite.NewEmptyFile()
	gohcl.EncodeIntoBody(backendConf, hclFile.Body())

	backendHCL := fmt.Sprintf(`
terraform {
	backend "%s" {
%s
	}
}
`, backendType, hclFile.Bytes())

	initFunc := backendInitFuncMap[backendType]
	if initFunc == nil || initFunc.InitFuncFromConf == nil {
		return nil, fmt.Errorf("backend type (%s) is not supported", backendType)
	}
	return initFunc.InitFuncFromConf(backendHCL, backendConf, k8sClient)
}
