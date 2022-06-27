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
	"fmt"
	"log"
	"os"

	"github.com/oam-dev/terraform-controller/api/v1beta2"
	"github.com/oam-dev/terraform-controller/controllers/configuration/backend"
	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"
)

var (
	configurationNameList []string
	applicationName       string
	componentNameList     []string
)

// newBackupCmd represents the backup command
func newBackupCmd(kubeFlags *genericclioptions.ConfigFlags) *cobra.Command {
	backupCmd := &cobra.Command{
		Use: "backup",
		PreRunE: func(cmd *cobra.Command, args []string) error {
			return app.BuildK8SClient(kubeFlags)
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return backup(context.Background())
		},
	}
	backupCmd.Flags().StringArrayVar(&configurationNameList, "configuration", []string{}, "the name of the configurations which need to be backed up")
	backupCmd.Flags().StringVar(&applicationName, "application", "", "the name of the application which need to be backed up")
	backupCmd.Flags().StringArrayVar(&componentNameList, "component", []string{}, "the name of the cloud resource components of the application.")

	return backupCmd
}

func backup(ctx context.Context) error {
	configurationList := make([]*v1beta2.Configuration, 0)
	for _, configurationName := range configurationNameList {
		configuration, err := app.GetConfiguration(ctx, configurationName)
		if err != nil {
			return err
		}
		configurationList = append(configurationList, configuration)
	}

	configurationFromApplication, err := app.GetConfigurationsFromApplication(ctx, applicationName, componentNameList)
	if err != nil {
		return err
	}
	configurationList = append(configurationList, configurationFromApplication...)

	for _, configuration := range configurationList {
		// backup the Terraform state
		if err := backupTFState(ctx, configuration); err != nil {
			return err
		}

		// backup the configuration
		serializer := app.BuildSerializer()
		filename := fmt.Sprintf("%s_%s_configuration.yaml", configuration.Name, configuration.Namespace)
		f, err := os.Create(filename)
		if err != nil {
			return err
		}
		defer f.Close()
		if _, err := f.WriteString("apiVersion: terraform.core.oam.dev/v1beta2\nkind: Configuration\n"); err != nil {
			return err
		}
		app.CleanUpConfiguration(configuration)
		if err := serializer.Encode(configuration, f); err != nil {
			return err
		}
		log.Printf("back up Configuration{Name: %s, Namespace: %s} to %s", configuration.Name, configuration.Namespace, filename)
	}

	return nil
}

func backupTFState(ctx context.Context, configuration *v1beta2.Configuration) error {
	backendInterface, err := backend.ParseConfigurationBackend(configuration, app.K8SClient, app.GetAllENVs())
	if err != nil {
		return err
	}
	state, err := backendInterface.GetTFStateJSON(ctx)
	if err != nil {
		log.Printf("failed to backup the Terraform state of Configuration{Name: %s, Namespace: %s}", configuration.Name, configuration.Namespace)
		return err
	}
	stateFile := fmt.Sprintf("%s_%s_configuration.state.json", configuration.Name, configuration.Namespace)
	if err := os.WriteFile(stateFile, state, os.ModePerm); err != nil {
		log.Printf("failed to backup the Terraform state of Configuration{Name: %s, Namespace: %s}", configuration.Name, configuration.Namespace)
		return err
	}
	log.Printf("back up the Terraform state of Configuration{Name: %s, Namespace: %s} to %s", configuration.Name, configuration.Namespace, stateFile)

	return nil
}
