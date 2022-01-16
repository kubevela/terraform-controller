package e2e

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"golang.org/x/net/context"
	"gotest.tools/assert"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"

	"github.com/oam-dev/terraform-controller/controllers/client"
)

const (
	backendSecretNamespace = "vela-system"
)

var (
	testConfigurationsBasic      = "examples/alibaba/eip/configuration_eip.yaml"
	testConfigurationsRegression = []string{
		"examples/alibaba/eip/configuration_eip.yaml",
		"examples/alibaba/eip/configuration_eip_remote_in_another_namespace.yaml",
		"examples/alibaba/eip/configuration_eip_remote_subdirectory.yaml",
		// "examples/alibaba/rds/configuration_hcl_rds.yaml",
		"examples/alibaba/oss/configuration_hcl_bucket.yaml",
	}
)

func TestBasicConfiguration(t *testing.T) {
	clientSet, err := client.Init()
	assert.NilError(t, err)
	ctx := context.Background()

	klog.Info("1. Applying Configuration")
	pwd, _ := os.Getwd()
	configuration := filepath.Join(pwd, "..", testConfigurationsBasic)
	cmd := fmt.Sprintf("kubectl apply -f %s", configuration)
	err = exec.Command("bash", "-c", cmd).Start()
	assert.NilError(t, err)

	klog.Info("2. Checking Configuration status")
	for i := 0; i < 60; i++ {
		var fields []string
		output, err := exec.Command("bash", "-c", "kubectl get configuration").Output()
		assert.NilError(t, err)

		lines := strings.Split(string(output), "\n")
		for i, line := range lines {
			if i == 0 {
				continue
			}
			fields = strings.Fields(line)
			if len(fields) == 3 && fields[0] == "alibaba-eip" && fields[1] == Available {
				goto continueCheck
			}
		}
		if i == 59 {
			t.Error("Configuration is not ready")
		}
		time.Sleep(time.Second * 5)
	}

continueCheck:
	klog.Info("3. Checking Configuration status")

	klog.Info("- Checking ConfigMap which stores .tf")
	_, err = clientSet.CoreV1().ConfigMaps("default").Get(ctx, "tf-alibaba-eip", v1.GetOptions{})
	assert.NilError(t, err)

	klog.Info("- Checking Secret which stores Backend")
	_, err = clientSet.CoreV1().Secrets(backendSecretNamespace).Get(ctx, "tfstate-default-alibaba-eip", v1.GetOptions{})
	assert.NilError(t, err)

	klog.Info("- Checking Secret which stores outputs")
	_, err = clientSet.CoreV1().Secrets("default").Get(ctx, "eip-conn", v1.GetOptions{})
	assert.NilError(t, err)

	klog.Info("- Checking Secret which stores variables")
	_, err = clientSet.CoreV1().Secrets("default").Get(ctx, "variable-alibaba-eip", v1.GetOptions{})
	assert.NilError(t, err)

	klog.Info("4. Deleting Configuration")
	cmd = fmt.Sprintf("kubectl delete -f %s", configuration)
	err = exec.Command("bash", "-c", cmd).Start()
	assert.NilError(t, err)

	klog.Info("5. Checking Configuration is deleted")
	for i := 0; i < 60; i++ {
		var (
			fields  []string
			existed bool
		)
		output, err := exec.Command("bash", "-c", "kubectl get configuration").Output()
		assert.NilError(t, err)

		lines := strings.Split(string(output), "\n")

		for j, line := range lines {
			if j == 0 {
				continue
			}
			fields = strings.Fields(line)
			if len(fields) == 3 && fields[0] == "alibaba-eip" {
				existed = true
			}
		}
		if existed {
			if i == 59 {
				t.Error("Configuration is not ready")
			}

			time.Sleep(time.Second * 5)
			continue
		} else {
			break
		}
	}

	klog.Info("6. Checking Secrets and ConfigMap which should all be deleted")
	_, err = clientSet.CoreV1().Secrets("default").Get(ctx, "variable-alibaba-eip", v1.GetOptions{})
	assert.Equal(t, kerrors.IsNotFound(err), true)

	_, err = clientSet.CoreV1().Secrets("default").Get(ctx, "eip-conn", v1.GetOptions{})
	assert.Equal(t, kerrors.IsNotFound(err), true)

	_, err = clientSet.CoreV1().Secrets(backendSecretNamespace).Get(ctx, "tfstate-default-alibaba-eip", v1.GetOptions{})
	assert.Equal(t, kerrors.IsNotFound(err), true)

	_, err = clientSet.CoreV1().ConfigMaps("default").Get(ctx, "tf-alibaba-eip", v1.GetOptions{})
	assert.Equal(t, kerrors.IsNotFound(err), true)
}

func TestBasicConfigurationRegression(t *testing.T) {
	var retryTimes = 120

	klog.Info("0. Create namespace")
	err := exec.Command("bash", "-c", "kubectl create ns abc").Start()
	assert.NilError(t, err)

	Regression(t, testConfigurationsRegression, retryTimes)
}
