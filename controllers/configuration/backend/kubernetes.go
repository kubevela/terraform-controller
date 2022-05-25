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
	"os"

	"github.com/oam-dev/terraform-controller/api/v1beta2"
	"github.com/oam-dev/terraform-controller/controllers/util"
	"github.com/pkg/errors"
	v1 "k8s.io/api/core/v1"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	terraformWorkspace = "default"
	// TerraformStateNameInSecret is the key name to store Terraform state
	TerraformStateNameInSecret = "tfstate"
	// TFBackendSecret is the Secret name for Kubernetes backend
	TFBackendSecret = "tfstate-%s-%s"
)

// K8SBackend is used to interact with the Terraform kubernetes backend
type K8SBackend struct {
	// Client is used to interact with the Kubernetes apiServer
	Client client.Client
	// HCLCode stores the backend hcl code string
	HCLCode string
	// SecretSuffix is the suffix of the name of the Terraform backend secret
	SecretSuffix string
	// SecretNS is the namespace of the Terraform backend secret
	SecretNS string
}

func getDefaultK8sBackendSecretNS() string {
	ns := os.Getenv("TERRAFORM_BACKEND_NAMESPACE")
	if ns == "" {
		ns = "vela-system"
	}
	return ns
}

func newDefaultK8SBackend(suffix string, client client.Client) *K8SBackend {
	ns := getDefaultK8sBackendSecretNS()
	hcl := renderK8SBackendHCL(suffix, ns)
	return &K8SBackend{
		Client:       client,
		HCLCode:      hcl,
		SecretSuffix: suffix,
		SecretNS:     ns,
	}
}

func newK8SBackendFromInline(backendConfig *ParsedBackendConfig, client client.Client) (Backend, error) {
	suffix, err := backendConfig.getAttrString("secret_suffix")
	if err != nil {
		return nil, err
	}
	ns, err := backendConfig.getAttrString("namespace")
	if err != nil {
		return nil, err
	}
	if ns == "" {
		ns = getDefaultK8sBackendSecretNS()
	}
	return &K8SBackend{
		Client:       client,
		HCLCode:      renderK8SBackendHCL(suffix, ns),
		SecretSuffix: suffix,
		SecretNS:     ns,
	}, nil
}

func newK8SBackendFromExplicit(backendConfig interface{}, client client.Client) (Backend, error) {
	conf, ok := backendConfig.(*v1beta2.KubernetesBackendConf)
	if !ok || conf == nil {
		return nil, errors.New("invalid backendConf")
	}
	ns := ""
	if conf.Namespace != nil {
		ns = *conf.Namespace
	} else {
		ns = getDefaultK8sBackendSecretNS()
	}
	return &K8SBackend{
		Client:       client,
		HCLCode:      renderK8SBackendHCL(conf.SecretSuffix, ns),
		SecretSuffix: conf.SecretSuffix,
		SecretNS:     ns,
	}, nil
}

func renderK8SBackendHCL(suffix, ns string) string {
	fmtStr := `
terraform {
  backend "kubernetes" {
    secret_suffix     = "%s"
    in_cluster_config = true
    namespace         = "%s"
  }
}
`
	return fmt.Sprintf(fmtStr, suffix, ns)
}

func (k K8SBackend) secretName() string {
	return fmt.Sprintf(TFBackendSecret, terraformWorkspace, k.SecretSuffix)
}

// GetTFStateJSON gets Terraform state json from the Terraform kubernetes backend
func (k *K8SBackend) GetTFStateJSON(ctx context.Context) ([]byte, error) {
	var s = v1.Secret{}
	if err := k.Client.Get(ctx, client.ObjectKey{Name: k.secretName(), Namespace: k.SecretNS}, &s); err != nil {
		return nil, errors.Wrap(err, "terraform state file backend secret is not generated")
	}
	tfStateData, ok := s.Data[TerraformStateNameInSecret]
	if !ok {
		return nil, fmt.Errorf("failed to get %s from Terraform State secret %s", TerraformStateNameInSecret, s.Name)
	}

	tfStateJSON, err := util.DecompressTerraformStateSecret(string(tfStateData))
	if err != nil {
		return nil, errors.Wrap(err, "failed to decompress state secret data")
	}
	return tfStateJSON, nil
}

// CleanUp will delete the Terraform kubernetes backend secret when deleting the configuration object
func (k *K8SBackend) CleanUp(ctx context.Context) error {
	klog.InfoS("Deleting the secret which stores Kubernetes backend", "Name", k.secretName())
	var kubernetesBackendSecret v1.Secret
	if err := k.Client.Get(ctx, client.ObjectKey{Name: k.secretName(), Namespace: k.SecretNS}, &kubernetesBackendSecret); err == nil {
		if err := k.Client.Delete(ctx, &kubernetesBackendSecret); err != nil {
			return err
		}
	}
	return nil
}

// HCL returns the backend hcl code string
func (k *K8SBackend) HCL() string {
	if k.HCLCode == "" {
		k.HCLCode = renderK8SBackendHCL(k.SecretSuffix, k.SecretNS)
	}
	return k.HCLCode
}
