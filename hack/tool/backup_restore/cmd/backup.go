package cmd

import (
	"context"
	"log"
	"os"

	"github.com/oam-dev/terraform-controller/api/v1beta2"
	"github.com/oam-dev/terraform-controller/controllers/configuration/backend"
	"github.com/spf13/cobra"
	v1 "k8s.io/api/core/v1"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var configurationName string

// newBackupCmd represents the backup command
func newBackupCmd(kubeFlags *genericclioptions.ConfigFlags) *cobra.Command {
	restoreCmd := &cobra.Command{
		Use: "backup",
		PreRunE: func(cmd *cobra.Command, args []string) error {
			if configurationName == "" {
				log.Fatal("please provide the name of the configuration which need to be backed up")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			err := buildK8SClient(kubeFlags)
			if err != nil {
				return err
			}
			return backup(context.Background())
		},
	}
	restoreCmd.Flags().StringVar(&configurationName, "name", "", "the name of the configuration which needs to be backed up")
	return restoreCmd
}

func backup(ctx context.Context) error {
	configuration := &v1beta2.Configuration{}
	if err := k8sClient.Get(ctx, client.ObjectKey{Name: configurationName, Namespace: currentNS}, configuration); err != nil {
		return err
	}

	// backup the state
	if err := backupTFState(ctx, configuration); err != nil {
		log.Fatalf("back up the Terraform state failed: %s \n", err.Error())
	}

	// backup the configuration
	serializer := buildSerializer()
	f, err := os.Create("configuration.yaml")
	if err != nil {
		return err
	}
	defer f.Close()
	if _, err := f.WriteString("apiVersion: terraform.core.oam.dev/v1beta2\nkind: Configuration\n"); err != nil {
		return err
	}
	cleanUpConfiguration(configuration)
	if err := serializer.Encode(configuration, f); err != nil {
		return err
	}
	log.Println("back up the Terraform state to configuration.yaml")
	return nil
}

func backupTFState(ctx context.Context, configuration *v1beta2.Configuration) error {
	backendInterface, err := backend.ParseConfigurationBackend(configuration, k8sClient)
	if err != nil {
		return err
	}
	k8sBackend, ok := backendInterface.(*backend.K8SBackend)
	if !ok {
		log.Println("the configuration doesn't use the kubernetes backend, the Terraform state won't be backed up")
		return nil
	}

	secret := &v1.Secret{}
	if err := k8sClient.Get(ctx, client.ObjectKey{Name: getSecretName(k8sBackend), Namespace: currentNS}, secret); err != nil {
		return err
	}
	stateData := string(secret.Data["tfstate"])
	state, err := decompressTRState(stateData)
	if err != nil {
		return nil
	}
	if err := os.WriteFile("state.json", state, os.ModePerm); err != nil {
		return err
	}
	log.Println("back up the Terraform state to state.json")
	return nil
}
