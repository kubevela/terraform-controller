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

	c := v1.Container{
		Name:            types.TerraformInitContainerName,
		Image:           a.TerraformImage,
		ImagePullPolicy: v1.PullIfNotPresent,
		Command: []string{
			"sh",
			"-c",
			"terraform init",
		},
		VolumeMounts: mounts,
		Env:          a.Envs,
	}
	return c
}
