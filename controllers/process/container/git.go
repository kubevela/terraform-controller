package container

import (
	"fmt"
	"github.com/oam-dev/terraform-controller/api/types"
	"path/filepath"

	v1 "k8s.io/api/core/v1"
)

const GitContainerName = "git-configuration"

// GitContainer will clone the git repository, and copy the files to the working directory
func (a *Assembler) GitContainer() v1.Container {
	mounts := []v1.VolumeMount{
		{
			Name:      a.Name,
			MountPath: types.WorkingVolumeMountPath,
		},
		{
			Name:      types.BackendVolumeName,
			MountPath: types.BackendVolumeMountPath,
		},
	}

	if a.GitCredential {
		mounts = append(mounts,
			v1.VolumeMount{
				Name:      types.GitAuthConfigVolumeName,
				MountPath: types.GitAuthConfigVolumeMountPath,
			})
	}

	command := a.getCloneCommand()
	return v1.Container{
		Name:            GitContainerName,
		Image:           a.GitImage,
		ImagePullPolicy: v1.PullIfNotPresent,
		Command:         command,
		VolumeMounts:    mounts,
	}
}

func (a *Assembler) getCloneCommand() []string {
	hclPath := filepath.Join(types.BackendVolumeMountPath, a.Git.Path)
	cloneCommand := fmt.Sprintf("git clone %s %s && cp -r %s/* %s", a.Git.URL, types.BackendVolumeMountPath, hclPath, types.WorkingVolumeMountPath)

	// Check for git credentials, mount the SSH known hosts and private key, add private key into the SSH authentication agent
	if a.GitCredential {
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
