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
	"github.com/oam-dev/terraform-controller/api/v1beta2"
	"github.com/pkg/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	backendTypeK8S = "kubernetes"
	backendTypeS3  = "s3"
)

// Backend is an abstraction of what all backend types can do
type Backend interface {
	// HCL can get the hcl code string
	HCL() string

	// GetTFStateJSON is used to get the Terraform state json from backend
	GetTFStateJSON(ctx context.Context) ([]byte, error)

	// CleanUp is used to clean up the backend when delete the configuration object
	// For example, if the configuration use kubernetes backend, CleanUp will delete the backend secret
	CleanUp(ctx context.Context) error
}

type backendInitFunc func(k8sClient client.Client, backendConf interface{}, credentials map[string]string) (Backend, error)

var backendInitFuncMap = map[string]backendInitFunc{
	backendTypeK8S: newK8SBackend,
	backendTypeS3:  newS3Backend,
}

// ParseConfigurationBackend parses backend Conf from the v1beta2.Configuration
func ParseConfigurationBackend(configuration *v1beta2.Configuration, k8sClient client.Client, credentials map[string]string) (Backend, error) {
	backend := configuration.Spec.Backend

	var (
		backendType string
		backendConf interface{}
		err         error
	)

	switch {
	case backend == nil || (backend.Inline == "" && backend.BackendType == ""):
		// use the default k8s backend
		return handleDefaultBackend(configuration, k8sClient)

	case backend.Inline != "" && backend.BackendType != "":
		return nil, errors.New("it's not allowed to set `spec.backend.inline` and `spec.backend.backendType` at the same time")

	case backend.Inline != "":
		// In this case, use the inline custom backend
		backendType, backendConf, err = handleInlineBackendHCL(backend.Inline)

	case backend.BackendType != "":
		// In this case, use the explicit custom backend
		backendType, backendConf, err = handleExplicitBackend(backend)
	}
	if err != nil {
		return nil, err
	}

	initFunc := backendInitFuncMap[backendType]
	if initFunc == nil {
		return nil, fmt.Errorf("backend type (%s) is not supported", backendType)
	}
	return initFunc(k8sClient, backendConf, credentials)
}

func handleDefaultBackend(configuration *v1beta2.Configuration, k8sClient client.Client) (Backend, error) {
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
	return newDefaultK8SBackend(configuration.Spec.Backend.SecretSuffix, k8sClient, configuration.Namespace), nil
}

func handleInlineBackendHCL(hclCode string) (string, interface{}, error) {
	type TerraformConfig struct {
		Terraform struct {
			Backend struct {
				Name  string   `hcl:"name,label"`
				Attrs hcl.Body `hcl:",remain"`
			} `hcl:"backend,block"`
		} `hcl:"terraform,block"`
	}

	hclFile, diags := hclparse.NewParser().ParseHCL([]byte(hclCode), "backend")
	if diags.HasErrors() {
		return "", nil, fmt.Errorf("there are syntax errors in the inline backend hcl code: %w", diags)
	}

	// try to parse hclFile to TerraformConfig or TerraformConfig.Terraform
	config := &TerraformConfig{}
	diags = gohcl.DecodeBody(hclFile.Body, nil, config)
	if diags.HasErrors() || config.Terraform.Backend.Name == "" {
		diags = gohcl.DecodeBody(hclFile.Body, nil, &config.Terraform)
		if diags.HasErrors() || config.Terraform.Backend.Name == "" {
			return "", nil, fmt.Errorf("the inline backend hcl code is not valid Terraform backend configuration: %w", diags)
		}
	}

	backendType := config.Terraform.Backend.Name

	var backendConf interface{}
	switch strings.ToLower(backendType) {
	case backendTypeK8S:
		backendConf = &v1beta2.KubernetesBackendConf{}
	case backendTypeS3:
		backendConf = &v1beta2.S3BackendConf{}
	default:
		return "", nil, fmt.Errorf("backend type (%s) is not supported", backendType)
	}
	diags = gohcl.DecodeBody(config.Terraform.Backend.Attrs, nil, backendConf)
	if diags.HasErrors() {
		return "", nil, fmt.Errorf("the inline backend hcl code is not valid Terraform backend configuration: %w", diags)
	}

	return backendType, backendConf, nil
}

func handleExplicitBackend(backend *v1beta2.Backend) (string, interface{}, error) {
	// check if is valid custom backend
	backendType := backend.BackendType

	// fetch backendConfValue using reflection
	backendStructValue := reflect.ValueOf(backend)
	if backendStructValue.Kind() == reflect.Ptr {
		backendStructValue = backendStructValue.Elem()
	}
	backendField := backendStructValue.FieldByNameFunc(func(name string) bool {
		return strings.EqualFold(name, backendType)
	})
	if backendField.Kind() != reflect.Ptr || backendField.IsNil() {
		return "", nil, fmt.Errorf("there is no configuration for backendType %s", backend.BackendType)
	}
	return backendType, backendField.Interface(), nil
}
