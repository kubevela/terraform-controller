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

package kubernetes

import (
	"bytes"
	"fmt"

	"github.com/mitchellh/go-homedir"
	"github.com/oam-dev/terraform-controller/controllers/configuration/backend/util"
	k8sSchema "k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	"k8s.io/klog/v2"
)

var (
	secretResource = k8sSchema.GroupVersionResource{
		Group:    "",
		Version:  "v1",
		Resource: "secrets",
	}
)

// Modified based on Hashicorp code base https://github.com/hashicorp/terraform/blob/1846a4752ee8056affad16c583b3072bc55feebd/internal/backend/remote-state/kubernetes/backend.go#L188
// Licensed under Mozilla Public License 2.0
type backend struct {
	// The fields below are set from configure
	config     *restclient.Config
	namespace  string
	labels     map[string]string
	nameSuffix string
}

func newBackend(conf util.ConfData) (*backend, error) {
	b := &backend{}
	if err := b.configure(conf); err != nil {
		return nil, err
	}
	return b, nil
}

// Modified based on Hashicorp code base https://github.com/hashicorp/terraform/blob/1846a4752ee8056affad16c583b3072bc55feebd/internal/backend/remote-state/kubernetes/backend.go#L200
// Licensed under Mozilla Public License 2.0
func (b *backend) kubernetesSecretClient() (dynamic.ResourceInterface, error) {
	client, err := dynamic.NewForConfig(b.config)
	if err != nil {
		return nil, fmt.Errorf("failed to configure: %w", err)
	}

	return client.Resource(secretResource).Namespace(b.namespace), nil
}

// Modified based on Hashicorp code base https://github.com/hashicorp/terraform/blob/1846a4752ee8056affad16c583b3072bc55feebd/internal/backend/remote-state/kubernetes/backend.go#L228
// Licensed under Mozilla Public License 2.0
func (b *backend) configure(data util.ConfData) error {
	cfg, err := getInitialConfig(data)
	if err != nil {
		return err
	}

	// Overriding with static configuration
	cfg.UserAgent = "HashiCorp/1.0 Terraform/1.3.0"

	if v, ok := data.GetOk("host"); ok {
		cfg.Host = v.(string)
	}
	if v, ok := data.GetOk("username"); ok {
		cfg.Username = v.(string)
	}
	if v, ok := data.GetOk("password"); ok {
		cfg.Password = v.(string)
	}
	if v, ok := data.GetOk("insecure"); ok {
		cfg.Insecure = v.(bool)
	}
	if v, ok := data.GetOk("cluster_ca_certificate"); ok {
		cfg.CAData = bytes.NewBufferString(v.(string)).Bytes()
	}
	if v, ok := data.GetOk("client_certificate"); ok {
		cfg.CertData = bytes.NewBufferString(v.(string)).Bytes()
	}
	if v, ok := data.GetOk("client_key"); ok {
		cfg.KeyData = bytes.NewBufferString(v.(string)).Bytes()
	}
	if v, ok := data.GetOk("token"); ok {
		cfg.BearerToken = v.(string)
	}

	if v, ok := data.GetOk("labels"); ok {
		labels := map[string]string{}
		for k, vv := range v.(map[string]interface{}) {
			labels[k] = vv.(string)
		}
		b.labels = labels
	}

	ns := data.Get("namespace").(string)
	b.namespace = ns
	b.nameSuffix = data.Get("secret_suffix").(string)
	b.config = cfg

	return nil
}

// Modified based on Hashicorp code base https://github.com/hashicorp/terraform/blob/1846a4752ee8056affad16c583b3072bc55feebd/internal/backend/remote-state/kubernetes/backend.go#L285
// Licensed under Mozilla Public License 2.0
func getInitialConfig(data util.ConfData) (*restclient.Config, error) {
	var cfg *restclient.Config
	var err error

	inCluster := data.Get("in_cluster_config").(bool)
	if inCluster {
		cfg, err = restclient.InClusterConfig()
		if err != nil {
			return nil, err
		}
	} else {
		cfg, err = tryLoadingConfigFile(data)
		if err != nil {
			return nil, err
		}
	}

	if cfg == nil {
		cfg = &restclient.Config{}
	}
	return cfg, err
}

// Modified based on Hashicorp code base https://github.com/hashicorp/terraform/blob/1846a4752ee8056affad16c583b3072bc55feebd/internal/backend/remote-state/kubernetes/backend.go#L308
// Licensed under Mozilla Public License 2.0
func tryLoadingConfigFile(d util.ConfData) (*restclient.Config, error) {
	loader := &clientcmd.ClientConfigLoadingRules{}

	configPath := d.Get("config_path").(string)
	expandedPath, err := homedir.Expand(configPath)
	if err != nil {
		klog.Errorf("[DEBUG] Could not expand path: %s", err)
		return nil, err
	}
	klog.InfoS("[DEBUG] Using kubeconfig: %s", expandedPath)
	loader.ExplicitPath = expandedPath

	overrides := &clientcmd.ConfigOverrides{}

	ctx, ctxOk := d.GetOk("config_context")
	authInfo, authInfoOk := d.GetOk("config_context_auth_info")
	cluster, clusterOk := d.GetOk("config_context_cluster")
	if ctxOk || authInfoOk || clusterOk {
		if ctxOk {
			overrides.CurrentContext = ctx.(string)
		}

		overrides.Context = clientcmdapi.Context{}
		if authInfoOk {
			overrides.Context.AuthInfo = authInfo.(string)
		}
		if clusterOk {
			overrides.Context.Cluster = cluster.(string)
		}
	}

	if v, ok := d.GetOk("exec"); ok {
		exec := &clientcmdapi.ExecConfig{}
		if spec, ok := v.([]interface{})[0].(map[string]interface{}); ok {
			exec.APIVersion = spec["api_version"].(string)
			exec.Command = spec["command"].(string)
			exec.Args = expandStringSlice(spec["args"].([]interface{}))
			for kk, vv := range spec["env"].(map[string]interface{}) {
				exec.Env = append(exec.Env, clientcmdapi.ExecEnvVar{Name: kk, Value: vv.(string)})
			}
		} else {
			return nil, fmt.Errorf("failed to parse exec")
		}
		overrides.AuthInfo.Exec = exec
	}

	cc := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loader, overrides)
	cfg, err := cc.ClientConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to initialize kubernetes configuration: %w", err)
	}

	klog.Info("[INFO] Successfully initialized config")
	return cfg, nil
}

// Modified based on Hashicorp code base https://github.com/hashicorp/terraform/blob/1846a4752ee8056affad16c583b3072bc55feebd/internal/backend/remote-state/kubernetes/backend.go#L394
// Licensed under Mozilla Public License 2.0
func expandStringSlice(s []interface{}) []string {
	result := make([]string, len(s))
	for k, v := range s {
		// Handle the Terraform parser bug which turns empty strings in lists to nil.
		if v == nil {
			result[k] = ""
		} else {
			result[k] = v.(string)
		}
	}
	return result
}
