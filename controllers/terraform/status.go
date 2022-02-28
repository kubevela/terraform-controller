package terraform

import (
	"context"
	"strings"

	"github.com/pkg/errors"
	"k8s.io/klog/v2"

	"github.com/oam-dev/terraform-controller/api/types"
	"github.com/oam-dev/terraform-controller/controllers/client"
)

// GetTerraformStatus will get Terraform execution status
func GetTerraformStatus(ctx context.Context, namespace, jobName, containerName string) (types.ConfigurationState, error) {
	klog.InfoS("checking Terraform execution status", "Namespace", namespace, "Job", jobName)
	clientSet, err := client.Init()
	if err != nil {
		klog.ErrorS(err, "failed to init clientSet")
		return types.ConfigurationProvisioningAndChecking, err
	}

	logs, err := getPodLog(ctx, clientSet, namespace, jobName, containerName)
	if err != nil {
		klog.ErrorS(err, "failed to get pod logs")
		return types.ConfigurationProvisioningAndChecking, err
	}

	success, state, errMsg := analyzeTerraformLog(logs)
	if success {
		return state, nil
	}

	return state, errors.New(errMsg)
}

func analyzeTerraformLog(logs string) (bool, types.ConfigurationState, string) {
	lines := strings.Split(logs, "\n")
	for i, line := range lines {
		if strings.Contains(line, "31mError:") {
			errMsg := strings.Join(lines[i:], "\n")
			if strings.Contains(errMsg, "Invalid Alibaba Cloud region") {
				return false, types.InvalidRegion, line
			}
			return false, types.ConfigurationApplyFailed, errMsg
		}
	}
	return true, types.ConfigurationProvisioningAndChecking, ""
}
