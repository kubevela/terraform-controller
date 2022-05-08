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

package configuration

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/gohcl"
	"github.com/hashicorp/hcl/v2/hclparse"
	"github.com/hashicorp/hcl/v2/hclwrite"
	crossplane "github.com/oam-dev/terraform-controller/api/types/crossplane-runtime"
	"github.com/oam-dev/terraform-controller/api/v1beta2"
	"github.com/pkg/errors"
	"github.com/zclconf/go-cty/cty"
	"github.com/zclconf/go-cty/cty/gocty"
	"k8s.io/klog/v2"
)

var backendSecretMap = map[string]map[string]string{
	"azurerm": {
		"client_certificate_path": "ClientCertificateSecret",
	},
	"consul": {
		"ca_file":   "CAFileSecret",
		"cert_file": "CertFileSecret",
		"key_file":  "KeyFileSecret",
	},
	"etcdv3": {
		"cacert_path": "CacertSecret",
		"cert_path":   "CertSecret",
		"key_path":    "KeySecret",
	},
	"gcs": {
		"credentials": "CredentialsSecret",
	},
	"kubernetes": {
		"config_path":  "ConfigSecret",
		"config_paths": "ConfigSecrets",
	},
	"oss": {
		"shared_credentials_file": "SharedCredentialsSecret",
	},
	"s3": {
		"shared_credentials_file": "SharedCredentialsSecret",
	},
	"swift": {
		"cacert_file": "CacertSecret",
		"cert":        "CertSecret",
		"key":         "KeySecret",
	},
}

var backendTypes = []string{
	"remote", "artifactory", "azurerm", "consul", "cos", "etcd", "etcdv3", "gcs", "http", "kubernetes",
	"manta", "oss", "pg", "s3", "swift",
}

// BackendConfSecretPodPathWithKey builds the file path in the executor pod for a BackendConfSecret
func BackendConfSecretPodPathWithKey(secretName, secretKey string) string {
	return fmt.Sprintf("/kubevela-terraform-controller-backend-secret/%s/%s", secretName, secretKey)
}

// BackendConfSecretPodPath builds the dir path in the executor pod for a BackendConfSecret
func BackendConfSecretPodPath(secretName string) string {
	return fmt.Sprintf("/kubevela-terraform-controller-backend-secret/%s", secretName)
}

func checkBackendTypeValid(backendType string) bool {
	for _, v := range backendTypes {
		if v == backendType {
			return true
		}
	}
	return false
}

// BackendConf contains all the backend information in the meta
type BackendConf struct {
	BackendType string
	HCL         string
	// UseCustom indicates whether to use custom kubernetes backend
	UseCustom bool
	// Secrets describes which secret and which key in the secret should be mounted to the executor pod
	Secrets map[string][]string
}

// BackendConfSecretRef describes which secret should be mounted to the executor pod
type BackendConfSecretRef struct {
	// Name is the name of the secret which will be mounted to a pod when running the job
	// If the secret referred by the SecretRef is in the same namespace as the Configuration(and the job)
	// the Name will be the same as the secret's.
	// If not, the Name will be the name of the secret appended with "-terraform-core-oam-dev" to avoid naming conflict.
	Name      string
	SecretRef *crossplane.SecretKeySelector
}

// parseConfigurationBackend returns backend hcl string, backend type, useCustom, secretRef list and error
func parseConfigurationBackend(configuration *v1beta2.Configuration, terraformBackendNamespace string) (string, string, bool, []*BackendConfSecretRef, error) {
	backend := configuration.Spec.Backend

	var (
		backendConfValue interface{}
		backendType      string
	)

	switch {

	case backend != nil && len(backend.Inline) > 0:
		// In this case, use the inline custom backend
		backendTF, backendType, err := handleInlineBackendHCL(backend.Inline)
		return backendTF, backendType, true, nil, err

	case backend != nil && len(backend.BackendType) > 0:
		// In this case, use the explicit custom backend

		// first, check if is valid custom backend

		backendType = strings.ToLower(backend.BackendType)
		// check if backendType is valid
		if !checkBackendTypeValid(backendType) {
			return "", "", false, nil, fmt.Errorf("%s is unsupported backendType", backend.BackendType)
		}
		// fetch backendConfValue using reflection
		backendStructValue := reflect.ValueOf(backend)
		if backendStructValue.Kind() == reflect.Ptr {
			backendStructValue = backendStructValue.Elem()
		}
		backendField := backendStructValue.FieldByNameFunc(func(name string) bool {
			return strings.EqualFold(name, backendType)
		})
		if backendField.IsNil() {
			return "", "", false, nil, fmt.Errorf("there is no configuration for backendType %s", backend.BackendType)
		}
		backendConfValue = backendField.Interface()

		// second, handle the backendConf
		backendHCL, backendType, secretList := handleExplicitBackend(backendConfValue, backendType, configuration.Namespace)
		return backendHCL, backendType, true, secretList, nil

	case backend != nil && len(backend.BackendType) == 0:
		// In this case, use the default k8s backend
		klog.Warningf("the spec.backend.backend_type of Configuration{Namespace: %s, Name: %s} is empty, use the default kubernetes backend", configuration.Namespace, configuration.Name)
		fallthrough // down to default

	default:
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
		backendTF, err := RenderTemplate(configuration.Spec.Backend, terraformBackendNamespace)
		if err != nil {
			return "", "", false, nil, errors.Wrap(err, "failed to prepare Terraform backend configuration")
		}
		return backendTF, "kubernetes", false, nil, nil
	}

}

func handleInlineBackendHCL(code string) (string, string, error) {
	type BackendConfig struct {
		Name   string   `hcl:"name,label"`
		Remain hcl.Body `hcl:",remain"`
	}

	type BackendConfigWrap struct {
		Backend BackendConfig `hcl:"backend,block"`
	}

	type TerraformConfig struct {
		Remain    interface{} `hcl:",remain"`
		Terraform struct {
			Remain  interface{}   `hcl:",remain"`
			Backend BackendConfig `hcl:"backend,block"`
		} `hcl:"terraform,block"`
	}

	hclFile, diags := hclparse.NewParser().ParseHCL([]byte(code), "backend")
	if diags.HasErrors() {
		return "", "", fmt.Errorf("there are syntax errors in the inline backend hcl code: %w", diags)
	}

	// try to parse hclFile to Config or BackendConfig
	config := &TerraformConfig{}
	// nolint:staticcheck
	backendConfig := &BackendConfig{}
	shouldWrap := false
	diags = gohcl.DecodeBody(hclFile.Body, nil, config)
	if diags.HasErrors() || config.Terraform.Backend.Name == "" {
		backendConfigWrap := &BackendConfigWrap{}
		diags = gohcl.DecodeBody(hclFile.Body, nil, backendConfigWrap)
		if diags.HasErrors() || backendConfigWrap.Backend.Name == "" {
			return "", "", fmt.Errorf("the inline backend hcl code is not valid Terraform backend configuration: %w", diags)
		}
		shouldWrap = true
		backendConfig = &backendConfigWrap.Backend
	} else {
		backendConfig = &config.Terraform.Backend
	}

	// check whether the backendType is valid
	if strings.EqualFold(backendConfig.Name, "local") {
		return "", "", fmt.Errorf("backendType \"local\" is not supported")
	}
	// check if there is inappropriate fields in the backendConfig
	checkList := backendSecretMap[strings.ToLower(backendConfig.Name)]
	attrMap, _ := backendConfig.Remain.JustAttributes()
	for field := range checkList {
		if _, ok := attrMap[field]; ok {
			return "", "", fmt.Errorf("%s is not supported in the inline backend hcl code as we cannot use local file paths in the kubernetes cluster", field)
		}
	}

	if shouldWrap {
		return fmt.Sprintf(`
terraform {
%s
}
`, code), backendConfig.Name, nil
	}
	return code, backendConfig.Name, nil
}

func handleExplicitBackend(backendConf interface{}, backendType string, namespace string) (string, string, []*BackendConfSecretRef) {
	hclFile := hclwrite.NewEmptyFile()
	gohcl.EncodeIntoBody(backendConf, hclFile.Body())
	backendHCLBlock := hclFile.Body()

	secretList := make([]*BackendConfSecretRef, 0)
	secretMap := backendSecretMap[backendType]
	backendConfValue := reflect.ValueOf(backendConf)
	if backendConfValue.Kind() == reflect.Ptr {
		backendConfValue = backendConfValue.Elem()
	}
	for dest, src := range secretMap {
		// get the src value (secret ref)
		secretField := backendConfValue.FieldByName(src)
		if !secretField.IsValid() || secretField.IsNil() {
			continue
		}
		secretRef := secretField.Interface().(*crossplane.SecretKeySelector)
		backendSecret := &BackendConfSecretRef{SecretRef: secretRef}
		if secretRef.Namespace == namespace {
			backendSecret.Name = secretRef.Name
		} else {
			backendSecret.Name = secretRef.Name + "-terraform-core-oam-dev"
		}
		secretList = append(secretList, backendSecret)

		// replace pre attr
		_ = backendHCLBlock.RemoveBlock(backendHCLBlock.FirstMatchingBlock(src, nil))
		ctyVal, _ := gocty.ToCtyValue(BackendConfSecretPodPathWithKey(backendSecret.Name, secretRef.Key), cty.String)
		_ = backendHCLBlock.SetAttributeValue(dest, ctyVal)
	}

	backendHCL := fmt.Sprintf(`
terraform {
	backend "%s" {
%s
	}
}
`, backendType, hclFile.Bytes())
	return backendHCL, backendType, secretList
}
