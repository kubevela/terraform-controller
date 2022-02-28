package terraform

import (
	"bytes"
	"context"
	"fmt"
	"io"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
)

func getPodLog(ctx context.Context, client kubernetes.Interface, namespace, jobName string) (string, error) {
	label := fmt.Sprintf("job-name=%s", jobName)
	pods, err := client.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{LabelSelector: label})
	if err != nil || pods == nil || len(pods.Items) == 0 {
		klog.InfoS("pods are not found", "Label", label, "Error", err)
		return "", nil
	}
	pod := pods.Items[0]

	if pod.Status.Phase == v1.PodPending {
		return "", nil
	}

	req := client.CoreV1().Pods(namespace).GetLogs(pod.Name, &v1.PodLogOptions{})
	logs, err := req.Stream(ctx)
	if err != nil {
		return "", err
	}
	defer func(logs io.ReadCloser) {
		err := logs.Close()
		if err != nil {
			return
		}
	}(logs)

	return flushStream(logs, pod.Name)
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
