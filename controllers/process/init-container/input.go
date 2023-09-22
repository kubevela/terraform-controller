package init_container

import (
	"fmt"
	"github.com/oam-dev/terraform-controller/api/types"
	"github.com/oam-dev/terraform-controller/controllers/process"
	v1 "k8s.io/api/core/v1"
)

const InputContainerName = "prepare-input-terraform-configurations"

// InputInitContainer prepare input .tf files
func InputInitContainer(meta *process.TFConfigurationMeta) v1.Container {
	mounts := []v1.VolumeMount{

		{
			Name:      meta.Name,
			MountPath: types.WorkingVolumeMountPath,
		},
		{
			Name:      types.InputTFConfigurationVolumeName,
			MountPath: types.InputTFConfigurationVolumeMountPath,
		},
	}
	return v1.Container{
		Name:            InputContainerName,
		Image:           meta.BusyboxImage,
		ImagePullPolicy: v1.PullIfNotPresent,
		Command: []string{
			"sh",
			"-c",
			fmt.Sprintf("cp %s/* %s", types.InputTFConfigurationVolumeMountPath, types.WorkingVolumeMountPath),
		},
		VolumeMounts: mounts,
	}
}
