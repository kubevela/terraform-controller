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

type K8SBackend struct {
	Client       client.Client
	HCLCode      string
	SecretSuffix string
	SecretNS     string
}

func getTerraformBackendSecretNS() string {
	ns := os.Getenv("TERRAFORM_BACKEND_NAMESPACE")
	if ns == "" {
		ns = "vela-system"
	}
	return ns
}

func newDefaultK8SBackend(suffix string, client client.Client) *K8SBackend {
	ns := getTerraformBackendSecretNS()
	hcl := renderK8SBackendHCL(suffix, ns)
	return &K8SBackend{
		Client:       client,
		HCLCode:      hcl,
		SecretSuffix: suffix,
		SecretNS:     ns,
	}
}

func newK8SBackendFromHCL(hclCode string, backendConfig *ParsedBackendConfig, client client.Client) (Backend, error) {
	suffix, err := backendConfig.getAttrString("secret_suffix")
	if err != nil {
		return nil, err
	}
	return &K8SBackend{
		Client:       client,
		HCLCode:      hclCode,
		SecretSuffix: suffix,
		SecretNS:     getTerraformBackendSecretNS(),
	}, nil
}

func newK8SBackendFromConf(hclCode string, backendConfig interface{}, client client.Client) (Backend, error) {
	conf, ok := backendConfig.(*v1beta2.KubernetesBackendConf)
	if !ok || conf == nil {
		return nil, errors.New("invalid backendConf")
	}
	return &K8SBackend{
		Client:       client,
		HCLCode:      hclCode,
		SecretSuffix: conf.SecretSuffix,
		SecretNS:     getTerraformBackendSecretNS(),
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

func (k K8SBackend) SecretName() string {
	return fmt.Sprintf(TFBackendSecret, terraformWorkspace, k.SecretSuffix)
}

func (k *K8SBackend) GetTFStateJSON(ctx context.Context) ([]byte, error) {
	var s = v1.Secret{}
	if err := k.Client.Get(ctx, client.ObjectKey{Name: k.SecretName(), Namespace: k.SecretNS}, &s); err != nil {
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

func (k *K8SBackend) CleanUp(ctx context.Context) error {
	klog.InfoS("Deleting the secret which stores Kubernetes backend", "Name", k.SecretName())
	var kubernetesBackendSecret v1.Secret
	if err := k.Client.Get(ctx, client.ObjectKey{Name: k.SecretName(), Namespace: k.SecretNS}, &kubernetesBackendSecret); err == nil {
		if err := k.Client.Delete(ctx, &kubernetesBackendSecret); err != nil {
			return err
		}
	}
	return nil
}

func (k *K8SBackend) HCL() string {
	if k.HCLCode == "" {
		k.HCLCode = renderK8SBackendHCL(k.SecretSuffix, k.SecretNS)
	}
	return k.HCLCode
}
