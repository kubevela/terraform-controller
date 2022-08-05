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

package app

import (
	"context"
	"io"
	"log"
	"os"
	"time"

	"github.com/oam-dev/terraform-controller/api/types"
	crossplane "github.com/oam-dev/terraform-controller/api/types/crossplane-runtime"
	"github.com/oam-dev/terraform-controller/api/v1beta2"
	"github.com/oam-dev/terraform-controller/controllers/configuration/backend"
	"github.com/pkg/errors"
	batchv1 "k8s.io/api/batch/v1"
	v1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type ConfigurationWrapper struct {
	configuration *v1beta2.Configuration
}

func (c ConfigurationWrapper) Apply(ctx context.Context) error {
	log.Println("try to restore the configuration......")

	if err := K8SClient.Create(ctx, c.configuration); err != nil {
		return err
	}

	log.Println("apply the configuration successfully, wait it to be available......")
	return nil
}

func (c ConfigurationWrapper) GetK8SBackend() (*backend.K8SBackend, error) {
	backendInterface, err := backend.ParseConfigurationBackend(c.configuration, K8SClient, GetAllENVs())
	if err != nil {
		return nil, err
	}
	k8sBackend, ok := backendInterface.(*backend.K8SBackend)
	if !ok {
		return nil, errors.New("the configuration doesn't use the kubernetes backend, no need to restore the Terraform state")
	}
	return k8sBackend, nil
}

func (c ConfigurationWrapper) GetConfigurationNamespacedName() *crossplane.Reference {
	return &crossplane.Reference{
		Name:      c.configuration.Name,
		Namespace: c.configuration.Namespace,
	}
}

func NewConfigurationWrapperFromYAML(yamlPath string) (*ConfigurationWrapper, error) {
	configurationYamlBytes, err := os.ReadFile(yamlPath)
	if err != nil {
		return nil, err
	}
	configuration := &v1beta2.Configuration{}
	serializer := BuildSerializer()
	if _, _, err := serializer.Decode(configurationYamlBytes, nil, configuration); err != nil {
		return nil, err
	}
	if configuration.Namespace == "" {
		configuration.Namespace = currentNS
	}
	CleanUpObjectMeta(&configuration.ObjectMeta)
	return &ConfigurationWrapper{configuration: configuration}, nil
}

func GetConfiguration(ctx context.Context, configurationName string) (*v1beta2.Configuration, error) {
	configuration := &v1beta2.Configuration{}
	if err := K8SClient.Get(ctx, client.ObjectKey{Name: configurationName, Namespace: currentNS}, configuration); err != nil {
		return nil, err
	}
	return configuration, nil
}

func CleanUpObjectMeta(objectMeta *metav1.ObjectMeta) {
	objectMeta.ManagedFields = nil
	objectMeta.CreationTimestamp = metav1.Time{}
	objectMeta.Finalizers = nil
	objectMeta.Generation = 0
	objectMeta.ResourceVersion = ""
	objectMeta.UID = ""
}

func WaitConfiguration(ctx context.Context, namespacedName *crossplane.Reference) error {
	var configuration *v1beta2.Configuration
	errCh := make(chan error)
	timeoutCtx, cancel := context.WithTimeout(ctx, 20*time.Minute)
	defer cancel()
	go func() {
		gotConf := &v1beta2.Configuration{}
		for {
			if err := K8SClient.Get(ctx, client.ObjectKey{Name: namespacedName.Name, Namespace: namespacedName.Namespace}, gotConf); err != nil {
				if kerrors.IsNotFound(err) {
					log.Printf("can not find the configuration({Name: %s, Namespace: %s}), waiting......", namespacedName.Name, namespacedName.Namespace)
					time.Sleep(500 * time.Millisecond)
					continue
				}
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
	if err := K8SClient.Get(ctx, client.ObjectKey{Name: configuration.Name + "-apply", Namespace: configuration.Namespace}, job); err != nil {
		return err
	}
	podList, err := clientSet.CoreV1().Pods(configuration.Namespace).List(ctx, metav1.ListOptions{LabelSelector: labels.FormatLabels(job.Spec.Selector.MatchLabels)})
	if err != nil {
		return err
	}
	pod := podList.Items[0]
	logReader, err := clientSet.CoreV1().Pods(configuration.Namespace).GetLogs(pod.Name, &v1.PodLogOptions{Container: "terraform-executor"}).Stream(ctx)
	if err != nil {
		return err
	}
	defer logReader.Close()
	if _, err := io.Copy(os.Stdout, logReader); err != nil {
		return err
	}
	return nil
}

func ResumeK8SBackend(ctx context.Context, k8sBackend *backend.K8SBackend, stateJSONPath string) error {
	tfState, err := compressedTFState(stateJSONPath)
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

	if err := K8SClient.Get(ctx, client.ObjectKey{Name: getSecretName(k8sBackend), Namespace: k8sBackend.SecretNS}, &gotSecret); err != nil {
		if !kerrors.IsNotFound(err) {
			return err
		}
		// is not found, create the secret
		gotSecret = v1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      getSecretName(k8sBackend),
				Namespace: k8sBackend.SecretNS,
			},
			Type: v1.SecretTypeOpaque,
			Data: map[string][]byte{
				"tfstate": tfState,
			},
		}
		configureSecret()
		if err := K8SClient.Create(ctx, &gotSecret); err != nil {
			return err
		}
	} else {
		// update the secret
		configureSecret()
		gotSecret.Data["tfstate"] = tfState
		if err := K8SClient.Update(ctx, &gotSecret); err != nil {
			return err
		}
	}
	log.Println("the Terraform backend was restored successfully")
	return nil
}
