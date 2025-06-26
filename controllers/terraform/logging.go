// Package terraform provides Terraform execution status monitoring and logging utilities.
package terraform

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"regexp"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"

	"github.com/oam-dev/terraform-controller/api/types"
)

func getPods(ctx context.Context, client kubernetes.Interface, namespace, jobName string) (*v1.PodList, error) {
	label := fmt.Sprintf("job-name=%s", jobName)
	pods, err := client.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{LabelSelector: label})
	if err != nil {
		klog.InfoS("pods are not found", "Label", label, "Error", err)

		return nil, err
	}

	return pods, nil
}

func getPodLog(ctx context.Context, client kubernetes.Interface, namespace, jobName, containerName, initContainerName string) (types.Stage, string, error) {
	var (
		targetContainer = containerName
		stage           = types.ApplyStage
	)
	pods, err := getPods(ctx, client, namespace, jobName)
	if err != nil || pods == nil || len(pods.Items) == 0 {
		klog.V(4).InfoS("pods are not found", "PodName", jobName, "Namepspace", namespace, "Error", err)
		return stage, "", nil
	}
	pod := pods.Items[0]

	// Here are two cases for Pending phase: 1) init container `terraform init` is not finished yet, 2) pod is not ready yet.
	if pod.Status.Phase == v1.PodPending {
		for _, c := range pod.Status.InitContainerStatuses {
			if c.Name == initContainerName && !c.Ready {
				targetContainer = initContainerName
				stage = types.InitStage
				break
			}
		}
	}

	req := client.CoreV1().Pods(namespace).GetLogs(pod.Name, &v1.PodLogOptions{Container: targetContainer})
	logs, err := req.Stream(ctx)
	if err != nil {
		return stage, "", err
	}
	defer func(logs io.ReadCloser) {
		err := logs.Close()
		if err != nil {
			return
		}
	}(logs)

	log, err := flushStream(logs, pod.Name)
	if err != nil {
		return stage, "", err
	}

	// To learn how it works, please refer to https://github.com/zzxwill/terraform-log-stripper.
	strippedLog := stripColor(log)
	return stage, strippedLog, nil
}

func flushStream(rc io.ReadCloser, podName string) (string, error) {
	var buf = &bytes.Buffer{}
	_, err := io.Copy(buf, rc)
	if err != nil {
		return "", err
	}
	logContent := buf.String()
	klog.V(4).Info("pod logs", "Pod", podName, "Logs", logContent)
	return logContent, nil
}

func stripColor(log string) string {
	var re = regexp.MustCompile(`\x1b\[[0-9;]*m`)
	str := re.ReplaceAllString(log, "")
	return str
}
