package container

import (
	"fmt"
	"path/filepath"

	"github.com/oam-dev/terraform-controller/api/types"

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
	var cmd string
	hclPath := filepath.Join(types.BackendVolumeMountPath, a.Git.Path)
	copyCommand := fmt.Sprintf("cp -r %s/* %s", hclPath, types.WorkingVolumeMountPath)

	checkoutCommand := ""
	checkoutObject := getCheckoutObj(a.Git.Ref)
	if checkoutObject != "" {
		checkoutCommand = fmt.Sprintf("git checkout %s", checkoutObject)
	}
	cloneCommand := fmt.Sprintf("git clone %s %s", a.Git.URL, types.BackendVolumeMountPath)

	// Check for git credentials, mount the SSH known hosts and private key, add private key into the SSH authentication agent
	if a.GitCredential {
		sshCommand := fmt.Sprintf("eval `ssh-agent` && ssh-add %s/%s", types.GitAuthConfigVolumeMountPath, v1.SSHAuthPrivateKey)
		cloneCommand = fmt.Sprintf("%s && %s", sshCommand, cloneCommand)
	}

	cmd = cloneCommand

	if checkoutCommand != "" {
		cmd = fmt.Sprintf("%s && %s", cmd, checkoutCommand)
	}
	cmd = fmt.Sprintf("%s && %s", cmd, copyCommand)

	command := []string{
		"sh",
		"-c",
		cmd,
	}
	return command
}

func getCheckoutObj(ref types.GitRef) string {
	if ref.Commit != "" {
		return ref.Commit
	} else if ref.Tag != "" {
		return ref.Tag
	}
	return ref.Branch
}
