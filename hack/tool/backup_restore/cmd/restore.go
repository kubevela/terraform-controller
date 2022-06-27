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
	"backup_restore/internal/app"
	"context"
	"io"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/oam-dev/terraform-controller/api/types"
	"github.com/oam-dev/terraform-controller/api/v1beta2"
	"github.com/oam-dev/terraform-controller/controllers/configuration/backend"
	"github.com/spf13/cobra"
	batchv1 "k8s.io/api/batch/v1"
	v1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	StateJSONPath     string
	configurationPath string
)

// newRestoreCmd represents the restore command
func newRestoreCmd(kubeFlags *genericclioptions.ConfigFlags) *cobra.Command {
	restoreCmd := &cobra.Command{
		Use: "restore",
		RunE: func(cmd *cobra.Command, args []string) error {
			err := app.BuildK8SClient(kubeFlags)
			if err != nil {
				return err
			}
			pwd, err := os.Getwd()
			if err != nil {
				return err
			}
			StateJSONPath = filepath.Join(pwd, StateJSONPath)
			configurationPath = filepath.Join(pwd, configurationPath)
			return restore(context.Background())
		},
	}
	restoreCmd.Flags().StringVar(&StateJSONPath, "state", "state.json", "the path of the backed up Terraform state file")
	restoreCmd.Flags().StringVar(&configurationPath, "configuration", "configuration.yaml", "the path of the backed up configuration objcet yaml file")
	return restoreCmd
}

func restore(ctx context.Context) error {
	configuration, err := decodeConfigurationFromYAML()
	if err != nil {
		return err
	}
	backendInterface, err := backend.ParseConfigurationBackend(configuration, app.K8SClient, app.GetAllENVs())
	if err != nil {
		return err
	}

	// restore the backend
	if err := resumeK8SBackend(ctx, backendInterface); err != nil {
		return err
	}

	// apply the configuration yaml
	if err := applyConfiguration(ctx, configuration); err != nil {
		return err
	}
	return nil
}

func decodeConfigurationFromYAML() (*v1beta2.Configuration, error) {
	configurationYamlBytes, err := os.ReadFile(configurationPath)
	if err != nil {
		return nil, err
	}
	configuration := &v1beta2.Configuration{}
	serializer := app.BuildSerializer()
	if _, _, err := serializer.Decode(configurationYamlBytes, nil, configuration); err != nil {
		return nil, err
	}
	if configuration.Namespace == "" {
		configuration.Namespace = app.CurrentNS
	}
	app.CleanUpConfiguration(configuration)
	return configuration, nil
}

func applyConfiguration(ctx context.Context, configuration *v1beta2.Configuration) error {
	log.Println("try to restore the configuration......")

	if err := app.K8SClient.Create(ctx, configuration); err != nil {
		return err
	}

	log.Println("apply the configuration successfully, wait it to be available......")

	errCh := make(chan error)
	timeoutCtx, cancel := context.WithTimeout(ctx, 20*time.Minute)
	defer cancel()
	go func() {
		gotConf := &v1beta2.Configuration{}
		for {
			if err := app.K8SClient.Get(ctx, client.ObjectKey{Name: configuration.Name, Namespace: configuration.Namespace}, gotConf); err != nil {
				errCh <- err
				return
			}
			if gotConf.Status.Apply.State != types.Available {
				log.Printf("the state of configuration is %s, wait it to be available......\n", gotConf.Status.Apply.State)
				time.Sleep(2 * time.Second)
			} else {
				log.Println("the configuration is available now")
				break
			}
		}
		// refresh the configuration
		configuration = gotConf
		errCh <- nil
	}()
	select {
	case err := <-errCh:
		if err != nil {
			return err
		}

	case <-timeoutCtx.Done():
		log.Fatal("timeout waiting the configuration is available")
	}

	log.Printf("try to print the log of the `terraform apply`......\n\n")
	if err := printExecutorLog(ctx, configuration); err != nil {
		log.Fatalf("print the log of `terraform apply` error: %s\n", err.Error())
	}

	return nil
}

func printExecutorLog(ctx context.Context, configuration *v1beta2.Configuration) error {
	job := &batchv1.Job{}
	if err := app.K8SClient.Get(ctx, client.ObjectKey{Name: configuration.Name + "-apply", Namespace: configuration.Namespace}, job); err != nil {
		return err
	}
	podList, err := app.ClientSet.CoreV1().Pods(configuration.Namespace).List(ctx, metav1.ListOptions{LabelSelector: labels.FormatLabels(job.Spec.Selector.MatchLabels)})
	if err != nil {
		return err
	}
	pod := podList.Items[0]
	logReader, err := app.ClientSet.CoreV1().Pods(configuration.Namespace).GetLogs(pod.Name, &v1.PodLogOptions{Container: "terraform-executor"}).Stream(ctx)
	if err != nil {
		return err
	}
	defer logReader.Close()
	if _, err := io.Copy(os.Stdout, logReader); err != nil {
		return err
	}
	return nil
}

func resumeK8SBackend(ctx context.Context, backendInterface backend.Backend) error {
	k8sBackend, ok := backendInterface.(*backend.K8SBackend)
	if !ok {
		log.Println("the configuration doesn't use the kubernetes backend, no need to restore the Terraform state")
		return nil
	}

	tfState, err := app.CompressedTFState(StateJSONPath)
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

	if err := app.K8SClient.Get(ctx, client.ObjectKey{Name: app.GetSecretName(k8sBackend), Namespace: k8sBackend.SecretNS}, &gotSecret); err != nil {
		if !kerrors.IsNotFound(err) {
			return err
		}
		// is not found, create the secret
		gotSecret = v1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      app.GetSecretName(k8sBackend),
				Namespace: k8sBackend.SecretNS,
			},
			Type: v1.SecretTypeOpaque,
			Data: map[string][]byte{
				"tfstate": tfState,
			},
		}
		configureSecret()
		if err := app.K8SClient.Create(ctx, &gotSecret); err != nil {
			return err
		}
	} else {
		// update the secret
		configureSecret()
		gotSecret.Data["tfstate"] = tfState
		if err := app.K8SClient.Update(ctx, &gotSecret); err != nil {
			return err
		}
	}
	log.Println("the Terraform backend was restored successfully")
	return nil
}
