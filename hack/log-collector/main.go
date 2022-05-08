package main

import (
	"context"
	"os"
	"time"

	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	"k8s.io/client-go/rest"
)

func main() {
	if len(os.Args) < 6 {
		panic("no enough args")
	}
	secretName, secretNS, secretKey := os.Args[1], os.Args[2], os.Args[3]
	srcFileName, successFlagFileName := os.Args[4], os.Args[5]

	check := func(flagFileName string) bool {
		_, err := os.Stat(flagFileName)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return false
			}
			panic(err.Error())
		}
		data, err := os.ReadFile(flagFileName)
		if err != nil {
			panic(err.Error())
		}
		return len(data) > 0
	}

	for !check(successFlagFileName) {
		time.Sleep(1 * time.Second)
	}

	jsonBytes, err := os.ReadFile(srcFileName)
	if err != nil {
		panic(err.Error())
	}

	secret := corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: secretNS,
		},
		Data: map[string][]byte{secretKey: jsonBytes},
	}

	config, err := rest.InClusterConfig()
	if err != nil {
		panic(err.Error())
	}
	client, err := kubernetes.NewForConfig(config)
	if err != nil {
		panic(err.Error())
	}

	_, err = client.CoreV1().Secrets(secretNS).Create(context.Background(), &secret, metav1.CreateOptions{})
	if err != nil {
		panic(err.Error())
	}
}
