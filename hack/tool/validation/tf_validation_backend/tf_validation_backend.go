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

package main

import (
	"errors"
	"log"
	"os"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/gohcl"
	"github.com/hashicorp/hcl/v2/hclparse"
)

// Config describes the schema of a whole tf file.
type Config struct {
	Remain    interface{} `hcl:",remain"`
	Terraform struct {
		Remain  interface{}    `hcl:",remain"`
		Backend *BackendConfig `hcl:"backend,block"`
	} `hcl:"terraform,block"`
}

// BackendConfig describes the schema of the backend block.
type BackendConfig struct {
	Name   string   `hcl:"name,label"`
	Remain hcl.Body `hcl:",remain"`
}

func fetchBackendName(body hcl.Body) (string, error) {
	var config Config
	diags := gohcl.DecodeBody(body, nil, &config)
	if diags.HasErrors() {
		return "", diags
	}
	backendConf := config.Terraform.Backend
	if backendConf == nil {
		return "", errors.New("can not find the \"backend block\"")
	}
	return backendConf.Name, nil
}

func main() {
	if len(os.Args) < 2 {
		log.Fatalln("can not parse filename from args")
	}
	filename := os.Args[1]
	file, diags := hclparse.NewParser().ParseHCLFile(filename)
	if diags.HasErrors() {
		log.Fatalln(diags.Error())
	}
	backendName, err := fetchBackendName(file.Body)
	if err != nil {
		log.Fatalln(err.Error())
	}
	log.Printf("fetch the name of backend: %s\n", backendName)
}
