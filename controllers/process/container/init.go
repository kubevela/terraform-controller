package container

import (
	"github.com/oam-dev/terraform-controller/api/types"
	v1 "k8s.io/api/core/v1"
)

// InitContainer will run terraform init
func (a *Assembler) InitContainer() v1.Container {
	mounts := []v1.VolumeMount{
		{
			Name:      a.Name,
			MountPath: types.WorkingVolumeMountPath,
		},
	}
	if a.TerraformCredential {
		mounts = append(mounts,
			v1.VolumeMount{
				Name:      types.TerraformCredentialsConfigVolumeName,
				MountPath: types.TerraformCredentialsConfigVolumeMountPath,
			})
	}

	if a.TerraformRC {
		mounts = append(mounts,
			v1.VolumeMount{
				Name:      types.TerraformRCConfigVolumeName,
				MountPath: types.TerraformRCConfigVolumeMountPath,
			})
	}

	if a.TerraformCredentialsHelper {
		mounts = append(mounts,
			v1.VolumeMount{
				Name:      types.TerraformCredentialsHelperConfigVolumeName,
				MountPath: types.TerraformCredentialsHelperConfigVolumeMountPath,
			})
	}

	if a.GitCredential {
		mounts = append(mounts,
			v1.VolumeMount{
				Name:      types.GitAuthConfigVolumeName,
				MountPath: types.GitAuthConfigVolumeMountPath,
			})
	}

	c := v1.Container{
		Name:            types.TerraformInitContainerName,
		Image:           a.TerraformImage,
		ImagePullPolicy: v1.PullIfNotPresent,
		Command: a.getInitCommand(),
		VolumeMounts: mounts,
		Env:          a.Envs,
	}
	return c
}

// getInitCommand dynamically builds the terraform init command, injecting the SSH agent if needed.
func (a *Assembler) getInitCommand() []string {
	cmd := "terraform init"

	// If Git credentials exist, start the ssh-agent and add the key BEFORE running terraform init
	if a.GitCredential {
		sshCommand := fmt.Sprintf("eval `ssh-agent` && ssh-add %s/%s", types.GitAuthConfigVolumeMountPath, v1.SSHAuthPrivateKey)
		cmd = fmt.Sprintf("%s && %s", sshCommand, cmd)
	}

	command := []string{
		"sh",
		"-c",
		cmd,
	}
	return command
}
