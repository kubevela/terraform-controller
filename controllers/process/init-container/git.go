package init_container

import (
	"fmt"
	"github.com/oam-dev/terraform-controller/api/types"
	"github.com/oam-dev/terraform-controller/controllers/process"
	"path/filepath"

	v1 "k8s.io/api/core/v1"
)

const GitContainerName = "git-configuration"

func GitInitContainer(meta *process.TFConfigurationMeta) v1.Container {
	mounts := []v1.VolumeMount{
		{
			Name:      meta.Name,
			MountPath: types.WorkingVolumeMountPath,
		},
		{
			Name:      types.BackendVolumeName,
			MountPath: types.BackendVolumeMountPath,
		},
	}

	if meta.GitCredentialsSecretReference != nil {
		mounts = append(mounts,
			v1.VolumeMount{
				Name:      types.GitAuthConfigVolumeName,
				MountPath: types.GitAuthConfigVolumeMountPath,
			})
	}

	command := getCloneCommand(meta)
	return v1.Container{
		Name:            GitContainerName,
		Image:           meta.GitImage,
		ImagePullPolicy: v1.PullIfNotPresent,
		Command:         command,
		VolumeMounts:    mounts,
	}
}

func getCloneCommand(meta *process.TFConfigurationMeta) []string {
	hclPath := filepath.Join(types.BackendVolumeMountPath, meta.Git.Path)
	cloneCommand := fmt.Sprintf("git clone %s %s && cp -r %s/* %s", meta.Git.URL, types.BackendVolumeMountPath, hclPath, types.WorkingVolumeMountPath)

	// Check for git credentials, mount the SSH known hosts and private key, add private key into the SSH authentication agent
	if meta.GitCredentialsSecretReference != nil {
		sshCommand := fmt.Sprintf("eval `ssh-agent` && ssh-add %s/%s", types.GitAuthConfigVolumeMountPath, v1.SSHAuthPrivateKey)
		cloneCommand = fmt.Sprintf("%s && %s", sshCommand, cloneCommand)
	}

	command := []string{
		"sh",
		"-c",
		cloneCommand,
	}
	return command
}
