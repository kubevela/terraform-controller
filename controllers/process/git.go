package process

import (
	"fmt"
	"path/filepath"

	v1 "k8s.io/api/core/v1"
)

func (meta *TFConfigurationMeta) GetCloneCommand() []string {
	hclPath := filepath.Join(BackendVolumeMountPath, meta.Git.Path)
	cloneCommand := fmt.Sprintf("git clone %s %s && cp -r %s/* %s", meta.Git.URL, BackendVolumeMountPath, hclPath, WorkingVolumeMountPath)

	// Check for git credentials, mount the SSH known hosts and private key, add private key into the SSH authentication agent
	if meta.GitCredentialsSecretReference != nil {
		sshCommand := fmt.Sprintf("eval `ssh-agent` && ssh-add %s/%s", GitAuthConfigVolumeMountPath, v1.SSHAuthPrivateKey)
		cloneCommand = fmt.Sprintf("%s && %s", sshCommand, cloneCommand)
	}

	command := []string{
		"sh",
		"-c",
		cloneCommand,
	}
	return command
}
