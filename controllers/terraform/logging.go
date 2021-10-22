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
	config2 "sigs.k8s.io/controller-runtime/pkg/client/config"
)

func initClientSet() (*kubernetes.Clientset, error) {
	config, err := config2.GetConfig()
	if err != nil {
		return nil, err
	}
	return kubernetes.NewForConfig(config)
}

func getPodLog(ctx context.Context, client *kubernetes.Clientset, namespace, jobName string) (string, error) {
	label := fmt.Sprintf("job-name=%s", jobName)
	pods, err := client.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{LabelSelector: label})
	if err != nil || pods == nil || len(pods.Items) == 0 {
		klog.InfoS("pods are not found", "Label", label)
		return "", nil //nolint:nilerr
	}
	pod := pods.Items[0]

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

	var buf = &bytes.Buffer{}
	_, err = io.Copy(buf, logs)
	if err != nil {
		return "", err
	}
	logContent := buf.String()
	klog.Info("pod logs", "Pod", pod.Name, "Logs", logContent)
	return logContent, nil
}
