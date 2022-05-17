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
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/oam-dev/terraform-controller/controllers/configuration/backend/kubernetes"
	"github.com/oam-dev/terraform-controller/controllers/configuration/backend/util"
	"github.com/pkg/errors"
	"github.com/tmccombs/hcl2json/convert"
	v1 "k8s.io/api/core/v1"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var clientInitFuncMap = map[string]util.InitFunc{
	"kubernetes": kubernetes.NewClient,
}

// GetStateJSON fetches the state json from the Terraform backend
func GetStateJSON(ctx context.Context, k8sClient client.Client, namespace string, conf *Conf) ([]byte, error) {
	klog.Infof("try to fetch state json from the Terraform backend, using the backendConf{%#v}", conf)

	// 1. create a new work dir
	tmpDir, _ := os.MkdirTemp("", "backend")
	defer func() { _ = os.Remove(tmpDir) }()

	// 2. create the secret files
	for _, secret := range conf.Secrets {
		gotSecret := v1.Secret{}
		if err := k8sClient.Get(ctx, client.ObjectKey{Name: secret.Name, Namespace: namespace}, &gotSecret); err != nil {
			return nil, errors.Wrap(err, fmt.Sprintf("cannot find the secret{Name: %s, Namespace: %s}", secret.Name, namespace))
		}
		dirPath := filepath.Join(tmpDir, secret.Path)
		_ = os.MkdirAll(dirPath, os.ModePerm)
		for _, key := range secret.Keys {
			err := func() error {
				file, err := os.Create(filepath.Join(filepath.Clean(dirPath), filepath.Clean(key)))
				if err != nil {
					return err
				}
				defer func() { _ = file.Close() }()
				_, err = file.Write(gotSecret.Data[key])
				return err
			}()
			if err != nil {
				return nil, fmt.Errorf("get error when create backend secret files: %w", err)
			}
		}
	}

	// 3. parse the conf data
	confValueBytes, err := convert.Bytes([]byte(conf.HCL), "backend", convert.Options{})
	if err != nil {
		return nil, fmt.Errorf("convert hcl to json error: %w", err)
	}
	confValue := util.ConfData(make(map[string]interface{}))
	err = json.Unmarshal(confValueBytes, &confValue)
	if err != nil {
		return nil, fmt.Errorf("can not convert the backen hcl code to json format: %w", err)
	}
	confData := confValue["terraform"].([]interface{})[0].(map[string]interface{})["backend"].(map[string]interface{})[conf.BackendType].([]interface{})[0].(map[string]interface{})

	// 4. build client
	pwd, _ := os.Getwd()
	_ = os.Chdir(tmpDir)
	defer func() { _ = os.Chdir(pwd) }()
	f := clientInitFuncMap[conf.BackendType]
	if f == nil {
		return nil, fmt.Errorf("getting state json from the %s backend is not supported", conf.BackendType)
	}
	backendClient, err := f(confData)
	if err != nil {
		return nil, err
	}

	// 5. fetch state json
	return backendClient.Get(ctx)
}
