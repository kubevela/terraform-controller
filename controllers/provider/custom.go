package provider

import (
	"github.com/go-yaml/yaml"
	"github.com/pkg/errors"
	"k8s.io/klog/v2"
)

// CustomCredentials are credentials for custom (you self)
type CustomCredentials map[string]string

func getCustomCredentials(secretData []byte, name, namespace string) (map[string]string, error) {
	var ck = make(CustomCredentials)
	if err := yaml.Unmarshal(secretData, &ck); err != nil {
		klog.ErrorS(err, errConvertCredentials, "Name", name, "Namespace", namespace)
		return nil, errors.Wrap(err, errConvertCredentials)
	}
	return ck, nil
}
