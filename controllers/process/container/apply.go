// Package container assembles and applies Terraform containers.
package container

import (
	"fmt"

	"github.com/oam-dev/terraform-controller/api/types"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

// ApplyContainer returns the main container used to apply or destroy Terraform configuration.
func (a *Assembler) ApplyContainer(executionType types.TerraformExecutionType, resourceQuota types.ResourceQuota) v1.Container {

	c := v1.Container{
		Name:            types.TerraformContainerName,
		Image:           a.TerraformImage,
		ImagePullPolicy: v1.PullIfNotPresent,
		Command: []string{
			"bash",
			"-c",
			fmt.Sprintf("terraform %s -lock=false -auto-approve", executionType),
		},
		VolumeMounts: []v1.VolumeMount{
			{
				Name:      a.Name,
				MountPath: types.WorkingVolumeMountPath,
			},
			{
				Name:      types.InputTFConfigurationVolumeName,
				MountPath: types.InputTFConfigurationVolumeMountPath,
			},
		},
		Env: a.Envs,
	}

	if resourceQuota.ResourcesLimitsCPU != "" || resourceQuota.ResourcesLimitsMemory != "" ||
		resourceQuota.ResourcesRequestsCPU != "" || resourceQuota.ResourcesRequestsMemory != "" {
		resourceRequirements := v1.ResourceRequirements{}
		if resourceQuota.ResourcesLimitsCPU != "" || resourceQuota.ResourcesLimitsMemory != "" {
			resourceRequirements.Limits = map[v1.ResourceName]resource.Quantity{}
			if resourceQuota.ResourcesLimitsCPU != "" {
				resourceRequirements.Limits["cpu"] = resourceQuota.ResourcesLimitsCPUQuantity
			}
			if resourceQuota.ResourcesLimitsMemory != "" {
				resourceRequirements.Limits["memory"] = resourceQuota.ResourcesLimitsMemoryQuantity
			}
		}
		if resourceQuota.ResourcesRequestsCPU != "" || resourceQuota.ResourcesLimitsMemory != "" {
			resourceRequirements.Requests = map[v1.ResourceName]resource.Quantity{}
			if resourceQuota.ResourcesRequestsCPU != "" {
				resourceRequirements.Requests["cpu"] = resourceQuota.ResourcesRequestsCPUQuantity
			}
			if resourceQuota.ResourcesRequestsMemory != "" {
				resourceRequirements.Requests["memory"] = resourceQuota.ResourcesRequestsMemoryQuantity
			}
		}
		c.Resources = resourceRequirements
	}

	return c
}
