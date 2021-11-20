package client

import (
	"k8s.io/client-go/kubernetes"
	config2 "sigs.k8s.io/controller-runtime/pkg/client/config"
)

// InitClientSet initializes the Go client set
func InitClientSet() (*kubernetes.Clientset, error) {
	config, err := config2.GetConfig()
	if err != nil {
		return nil, err
	}
	return kubernetes.NewForConfig(config)
}

