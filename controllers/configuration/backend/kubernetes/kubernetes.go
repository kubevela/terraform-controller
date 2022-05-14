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
	"compress/gzip"
	"context"
	"encoding/base64"
	"fmt"
	"strings"

	"github.com/oam-dev/terraform-controller/controllers/configuration/backend/util"
	"github.com/pkg/errors"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/client-go/dynamic"
)

const (
	tfstateKey = "tfstate"
	workspace  = "default"
)

// Client is the client used to communicate with the Terraform kubernetes backend
// Modified based on Hashicorp code base https://github.com/hashicorp/terraform/blob/1846a4752ee8056affad16c583b3072bc55feebd/internal/backend/remote-state/kubernetes/backend.go#L188
// Licensed under Mozilla Public License 2.0
type Client struct {
	// The fields below are set from configure
	kubernetesSecretClient dynamic.ResourceInterface
	namespace              string
	labels                 map[string]string
	nameSuffix             string
}

// Get fetches the state json from the Terraform kubernetes backend
// Modified based on Hashicorp code base https://github.com/hashicorp/terraform/blob/1846a4752ee8056affad16c583b3072bc55feebd/internal/backend/remote-state/kubernetes/backend.go#L188
// Licensed under Mozilla Public License 2.0
func (c *Client) Get(ctx context.Context) ([]byte, error) {
	secretName, err := c.createSecretName()
	if err != nil {
		return nil, err
	}
	secret, err := c.kubernetesSecretClient.Get(ctx, secretName, metav1.GetOptions{})
	if err != nil {
		if k8serrors.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}

	secretData := getSecretData(secret)
	stateRaw, ok := secretData[tfstateKey]
	if !ok {
		// The secret exists but there is no state in it
		return nil, nil
	}

	stateRawString := stateRaw.(string)

	return uncompressState(stateRawString)
}

// Modified based on Hashicorp code base https://github.com/hashicorp/terraform/blob/1846a4752ee8056affad16c583b3072bc55feebd/internal/backend/remote-state/kubernetes/client.go#L323
// Licensed under Mozilla Public License 2.0
func (c *Client) createSecretName() (string, error) {
	secretName := strings.Join([]string{tfstateKey, workspace, c.nameSuffix}, "-")

	errs := validation.IsDNS1123Subdomain(secretName)
	if len(errs) > 0 {
		k8sInfo := `
This is a requirement for Kubernetes secret names. 
The workspace name and key must adhere to Kubernetes naming conventions.`
		msg := fmt.Sprintf("the secret name %v is invalid, ", secretName)
		return "", errors.New(msg + strings.Join(errs, ",") + k8sInfo)
	}

	return secretName, nil
}

// Modified based on Hashicorp code base https://github.com/hashicorp/terraform/blob/1846a4752ee8056affad16c583b3072bc55feebd/internal/backend/remote-state/kubernetes/client.go#L376
// Licensed under Mozilla Public License 2.0
func getSecretData(secret *unstructured.Unstructured) map[string]interface{} {
	if m, ok := secret.Object["data"].(map[string]interface{}); ok {
		return m
	}
	return map[string]interface{}{}
}

// Modified based on Hashicorp code base https://github.com/hashicorp/terraform/blob/1846a4752ee8056affad16c583b3072bc55feebd/internal/backend/remote-state/kubernetes/client.go#L358
// Licensed under Mozilla Public License 2.0
func uncompressState(data string) ([]byte, error) {
	decode, err := base64.StdEncoding.DecodeString(data)
	if err != nil {
		return nil, err
	}

	b := new(bytes.Buffer)
	gz, err := gzip.NewReader(bytes.NewReader(decode))
	if err != nil {
		return nil, err
	}
	_, _ = b.ReadFrom(gz)
	if err := gz.Close(); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}

// NewClient builds a Client from the util.ConfData
func NewClient(conf util.ConfData) (util.Client, error) {
	b, err := newBackend(conf)
	if err != nil {
		return nil, err
	}
	secretClient, err := b.kubernetesSecretClient()
	if err != nil {
		return nil, err
	}

	client := &Client{
		kubernetesSecretClient: secretClient,
		namespace:              b.namespace,
		labels:                 b.labels,
		nameSuffix:             b.nameSuffix,
	}

	return client, nil
}
