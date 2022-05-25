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

package cmd

import (
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/oam-dev/terraform-controller/api/v1beta2"
	"github.com/oam-dev/terraform-controller/controllers/configuration/backend"
	"github.com/spf13/cobra"
	v1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer/json"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	stateJSONPath     string
	configurationPath string
	k8sClient         client.Client
)

// newRestoreCmd represents the restore command
func newRestoreCmd(kubeFlags *genericclioptions.ConfigFlags) *cobra.Command {
	restoreCmd := &cobra.Command{
		Use: "restore",
		RunE: func(cmd *cobra.Command, args []string) error {
			var err error
			k8sClient, err = buildClientSet(kubeFlags)
			if err != nil {
				return err
			}
			pwd, err := os.Getwd()
			if err != nil {
				return err
			}
			stateJSONPath = filepath.Join(pwd, stateJSONPath)
			configurationPath = filepath.Join(pwd, configurationPath)
			return restore(context.Background())
		},
	}
	restoreCmd.Flags().StringVar(&stateJSONPath, "state", "state.json", "the path of the backed up Terraform state file")
	restoreCmd.Flags().StringVar(&configurationPath, "configuration", "configuration.yaml", "the path of the backed up configuration objcet yaml file")
	return restoreCmd
}

func buildClientSet(kubeFlags *genericclioptions.ConfigFlags) (client.Client, error) {
	config, err := kubeFlags.ToRESTConfig()
	if err != nil {
		return nil, err
	}
	return client.New(config, client.Options{})
}

func restore(ctx context.Context) error {
	configuration, err := getConfiguration()
	if err != nil {
		return err
	}
	backendInterface, err := backend.ParseConfigurationBackend(configuration, k8sClient)
	if err != nil {
		return err
	}

	// restore the backend
	if err := resumeK8SBackend(ctx, backendInterface); err != nil {
		return err
	}

	// apply the configuration yaml
	// FIXME (loheagn) use the restClient to do this
	applyCmd := exec.Command("bash", "-c", fmt.Sprintf("kubectl apply -f %s", configurationPath))
	if err := applyCmd.Run(); err != nil {
		return err
	}
	return nil
}

func getConfiguration() (*v1beta2.Configuration, error) {
	configurationYamlBytes, err := os.ReadFile(configurationPath)
	if err != nil {
		return nil, err
	}
	configuration := &v1beta2.Configuration{}
	scheme := runtime.NewScheme()
	serializer := json.NewSerializerWithOptions(json.DefaultMetaFactory, scheme, scheme, json.SerializerOptions{Yaml: true})
	if _, _, err := serializer.Decode(configurationYamlBytes, nil, configuration); err != nil {
		return nil, err
	}
	return configuration, nil
}

func resumeK8SBackend(ctx context.Context, backendInterface backend.Backend) error {
	k8sBackend, ok := backendInterface.(*backend.K8SBackend)
	if !ok {
		log.Println("the configuration doesn't use the kubernetes backend, no need to restore the Terraform state")
		return nil
	}

	tfState, err := compressedTFState()
	if err != nil {
		return err
	}

	var gotSecret v1.Secret

	configureSecret := func() {
		if gotSecret.Annotations == nil {
			gotSecret.Annotations = make(map[string]string)
		}
		gotSecret.Annotations["encoding"] = "gzip"

		if gotSecret.Labels == nil {
			gotSecret.Labels = make(map[string]string)
		}
		gotSecret.Labels["app.kubernetes.io/managed-by"] = "terraform"
		gotSecret.Labels["tfstate"] = "true"
		gotSecret.Labels["tfstateSecretSuffix"] = k8sBackend.SecretSuffix
		gotSecret.Labels["tfstateWorkspace"] = "default"

		gotSecret.Type = v1.SecretTypeOpaque
	}

	if err := k8sClient.Get(ctx, client.ObjectKey{Name: "tfstate-default-" + k8sBackend.SecretSuffix, Namespace: k8sBackend.SecretNS}, &gotSecret); err != nil {
		if !kerrors.IsNotFound(err) {
			return err
		}
		// is not found, create the secret
		gotSecret = v1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "tfstate-default-" + k8sBackend.SecretSuffix,
				Namespace: k8sBackend.SecretNS,
			},
			Type: v1.SecretTypeOpaque,
			Data: map[string][]byte{
				"tfstate": tfState,
			},
		}
		configureSecret()
		if err := k8sClient.Create(ctx, &gotSecret); err != nil {
			return err
		}
	} else {
		// update the secret
		configureSecret()
		gotSecret.Data["tfstate"] = tfState
		if err := k8sClient.Update(ctx, &gotSecret); err != nil {
			return err
		}
	}
	return nil
}

func compressedTFState() ([]byte, error) {
	srcBytes, err := os.ReadFile(stateJSONPath)
	var buf bytes.Buffer
	writer := gzip.NewWriter(&buf)
	_, err = writer.Write(srcBytes)
	if err != nil {
		return nil, err
	}
	if err := writer.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
