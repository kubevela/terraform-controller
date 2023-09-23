package container

import (
	"fmt"
	"github.com/oam-dev/terraform-controller/api/types"
	v1 "k8s.io/api/core/v1"
)

const InputContainerName = "prepare-input-terraform-configurations"

// InputContainer prepare input .tf files, copy them to the working directory
func (a *Assembler) InputContainer() v1.Container {
	mounts := []v1.VolumeMount{

		{
			Name:      a.Name,
			MountPath: types.WorkingVolumeMountPath,
		},
		{
			Name:      types.InputTFConfigurationVolumeName,
			MountPath: types.InputTFConfigurationVolumeMountPath,
		},
	}
	return v1.Container{
		Name:            InputContainerName,
		Image:           a.BusyboxImage,
		ImagePullPolicy: v1.PullIfNotPresent,
		Command: []string{
			"sh",
			"-c",
			fmt.Sprintf("cp %s/* %s", types.InputTFConfigurationVolumeMountPath, types.WorkingVolumeMountPath),
		},
		VolumeMounts: mounts,
	}
}
