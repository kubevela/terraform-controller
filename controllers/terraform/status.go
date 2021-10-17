package terraform

import (
	"context"
	"strings"

	"github.com/pkg/errors"
	"k8s.io/klog/v2"
)

// GetTerraformStatus will get Terraform execution status
func GetTerraformStatus(ctx context.Context, namespace, jobName string) error {
	klog.InfoS("checking Terraform execution status", "Namespace", namespace, "Job", jobName)
	clientSet, err := initClientSet()
	if err != nil {
		klog.ErrorS(err, "failed to init clientSet")
		return err
	}

	logs, err := getPodLog(ctx, clientSet, namespace, jobName)
	if err != nil {
		klog.ErrorS(err, "failed to get pod logs")
		return err
	}

	success, errMsg := analyzeTerraformLog(logs)
	if success {
		return nil
	}

	return errors.New(errMsg)
}

func analyzeTerraformLog(logs string) (bool, string) {
	lines := strings.Split(logs, "\n")
	for i, line := range lines {
		if strings.Contains(line, "31mError:") {
			errMsg := strings.Join(lines[i:], "\n")
			return false, errMsg
		}
	}
	return true, ""
}
