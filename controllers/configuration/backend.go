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
	"github.com/zclconf/go-cty/cty"
	"github.com/zclconf/go-cty/cty/gocty"
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

// BackendSecretRef describes which secret should be mounted to the executor pod
type BackendSecretRef struct {
	// Name is the name of the secret which will be mounted to a pod when running the job
	// If the secret referred by the SecretRef is in the same namespace as the Configuration(and the job)
	// the Name will be the same as the secret's.
	// If not, the Name will be the name of the secret appended with "-terraform-core-oam-dev" to avoid naming conflict.
	Name      string
	SecretRef *crossplane.SecretKeySelector
}

func parseConfigurationBackend(configuration *v1beta2.Configuration, terraformBackendNamespace string) (string, []*BackendSecretRef, error) {
	backend := configuration.Spec.Backend

	var backendConf interface{}
	var backendType string

	if backend != nil {
		if len(backend.Inline) > 0 {
			backendTF, err := handleInlineBackendHCL(backend.Inline)
			return backendTF, nil, err
		}

		// check if is custom backend
		backendStructValue := reflect.ValueOf(backend)
		if backendStructValue.Kind() == reflect.Ptr {
			backendStructValue = backendStructValue.Elem()
		}
		for _, typeName := range backendTypes {
			tName := typeName
			field := backendStructValue.FieldByNameFunc(func(name string) bool {
				return strings.EqualFold(name, tName)
			})
			if !field.IsNil() {
				backendConf, backendType = field.Interface(), tName
				break
			}
		}
	}

	if backendConf == nil {
		// use the default kubernetes backend
		var secretSuffix string
		if backend != nil {
			secretSuffix = backend.SecretSuffix
		}
		if len(secretSuffix) == 0 {
			secretSuffix = configuration.Name
		}
		backendConf = &v1beta2.KubernetesBackendConf{
			SecretSuffix:    secretSuffix,
			InClusterConfig: bool2Ptr(true),
			Namespace:       &terraformBackendNamespace,
		}
		backendType = "kubernetes"
	}

	return handleExplicitBackend(backendConf, backendType, terraformBackendNamespace)

}

func handleInlineBackendHCL(code string) (string, error) {
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
		return "", fmt.Errorf("there are syntax errors in the inline backend hcl code: %w", diags)
	}

	// try to parse hclFile to Config or BackendConfig
	config := &TerraformConfig{}
	backendConfig := &BackendConfig{}
	shouldWrap := false
	diags = gohcl.DecodeBody(hclFile.Body, nil, config)
	if diags.HasErrors() || config.Terraform.Backend.Name == "" {
		backendConfigWrap := &BackendConfigWrap{}
		diags = gohcl.DecodeBody(hclFile.Body, nil, backendConfigWrap)
		if diags.HasErrors() || backendConfigWrap.Backend.Name == "" {
			return "", fmt.Errorf("the inline backend hcl code is not valid Terraform backend configuration: %w", diags)
		}
		shouldWrap = true
		backendConfig = &backendConfigWrap.Backend
	} else {
		backendConfig = &config.Terraform.Backend
	}

	// check if there is inappropriate fields in the backendConfig
	checkList := backendSecretMap[strings.ToLower(backendConfig.Name)]
	attrMap, _ := backendConfig.Remain.JustAttributes()
	for field := range checkList {
		if _, ok := attrMap[field]; ok {
			return "", fmt.Errorf("%s is not supported in the inline backend hcl code as we cannot use local file paths in the kubernetes cluster", field)
		}
	}

	if shouldWrap {
		return fmt.Sprintf(`
terraform {
%s
}
`, code), nil
	}
	return code, nil
}

func handleExplicitBackend(backendConf interface{}, backendType string, namespace string) (string, []*BackendSecretRef, error) {
	hclFile := hclwrite.NewEmptyFile()
	gohcl.EncodeIntoBody(backendConf, hclFile.Body())
	backendHCLBlock := hclFile.Body()

	secretList := make([]*BackendSecretRef, 0)
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
		backendSecret := &BackendSecretRef{SecretRef: secretRef}
		if secretRef.Namespace == namespace {
			backendSecret.Name = secretRef.Name
		} else {
			backendSecret.Name = secretRef.Name + "-terraform-core-oam-dev"
		}
		secretList = append(secretList, backendSecret)

		// replace pre attr
		_ = backendHCLBlock.RemoveBlock(backendHCLBlock.FirstMatchingBlock(src, nil))
		filePathInPod := fmt.Sprintf("/var/%s/%s", backendSecret.Name, secretRef.Key)
		ctyVal, _ := gocty.ToCtyValue(filePathInPod, cty.String)
		_ = backendHCLBlock.SetAttributeValue(dest, ctyVal)
	}

	return fmt.Sprintf(`
terraform {
	backend "%s" {
%s
	}
}
`, backendType, hclFile.Bytes()), secretList, nil
}

func bool2Ptr(x bool) *bool {
	return &x
}
