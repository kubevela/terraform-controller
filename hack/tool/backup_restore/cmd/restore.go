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
	"log"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"
)

var (
	stateJSONPath     string
	configurationPath string
	applicationPath   string
	componentName     string
)

// newRestoreCmd represents the restore command
func newRestoreCmd(kubeFlags *genericclioptions.ConfigFlags) *cobra.Command {
	restoreCmd := &cobra.Command{
		Use: "restore",
		PreRunE: func(cmd *cobra.Command, args []string) error {
			err := app.BuildK8SClient(kubeFlags)
			if err != nil {
				return err
			}
			pwd, err := os.Getwd()
			if err != nil {
				return err
			}
			if stateJSONPath == "" {
				log.Fatal("`--state` should not be empty")
			}
			stateJSONPath = filepath.Join(pwd, stateJSONPath)
			if configurationPath == "" && applicationPath == "" {
				log.Fatal("`--configuration` and `--application` should not be empty at the same time")
			}
			if configurationPath != "" && applicationPath != "" {
				log.Fatal("`--configuration` and `--application` should not be set at the same time")
			}
			if configurationPath != "" {
				configurationPath = filepath.Join(pwd, configurationPath)
			} else {
				applicationPath = filepath.Join(pwd, applicationPath)
				if componentName == "" {
					log.Print("WARN: `--component` is empty. Will take the first component of the Application as the cloud resource which should be restored")
				}
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return restore(context.Background())
		},
	}
	restoreCmd.Flags().StringVar(&stateJSONPath, "state", "state.json", "the path of the backed up Terraform state file")
	restoreCmd.Flags().StringVar(&configurationPath, "configuration", "configuration.yaml", "the path of the backed up configuration object yaml file. This argument should not be used with `--application`")
	restoreCmd.Flags().StringVar(&applicationPath, "application", "application.yaml", "the path of the backed up application object yaml file. This argument should not be used with `--configuration`")
	restoreCmd.Flags().StringVar(&componentName, "component", "cloud-resource", "the component which should be restored in the application. This argument should be used with `--application`")
	return restoreCmd
}

func restore(ctx context.Context) error {
	var (
		resourceOwner app.CloudResourceOwner
		err           error
	)
	if configurationPath != "" {
		resourceOwner, err = app.NewConfigurationWrapperFromYAML(configurationPath)
	} else {
		resourceOwner, err = app.NewApplicationComponentFromYAML(applicationPath, componentName)
	}
	if err != nil {
		return err
	}

	k8sBackend, err := resourceOwner.GetK8SBackend()
	if err != nil {
		return err
	}

	// restore the backend
	if err := app.ResumeK8SBackend(ctx, k8sBackend, stateJSONPath); err != nil {
		return err
	}

	// apply the configuration or application
	if err := resourceOwner.Apply(ctx); err != nil {
		return err
	}

	return app.WaitConfiguration(ctx, resourceOwner.GetConfigurationNamespacedName())
}
