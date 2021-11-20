package e2e

import (
	"fmt"
	"golang.org/x/net/context"
	"gotest.tools/assert"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/oam-dev/terraform-controller/controllers/client"
)

func TestConfigurationResult(t *testing.T) {
	clientSet, err := client.InitClientSet()
	assert.NilError(t, err)
	ctx := context.Background()

	klog.Info("Applying Configuration")
	pwd, _ := os.Getwd()
	cmd := fmt.Sprintf("kubectl apply -f %s", filepath.Join(pwd, "..", "..", "examples/alibaba/eip/configuration_eip.yaml"))
	err = exec.Command("bash", "-c", cmd).Start()
	assert.NilError(t, err)

	klog.Info("Checking Configuration status")
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
			if len(fields) == 0 {
				continue
			}
			if !(len(fields) == 3 && fields[1] == "Available") {
				break
			}
		}
		time.Sleep(time.Second * 5)
	}

	klog.Info("Checking Secret which stores variables")
	_, err = clientSet.CoreV1().Secrets("default").Get(ctx, "variable-alibaba-eip", v1.GetOptions{})
	assert.NilError(t, err)

	klog.Info("Checking Secret which stores outputs")
	_, err = clientSet.CoreV1().Secrets("default").Get(ctx, "eip-conn", v1.GetOptions{})
	assert.NilError(t, err)

	klog.Info("Checking Secret which stores Backend")
	_, err = clientSet.CoreV1().Secrets("terraform").Get(ctx, "tfstate-default-alibaba-eip", v1.GetOptions{})
	assert.NilError(t, err)

	klog.Info("Checking ConfigMap which stores .tf")
	_, err = clientSet.CoreV1().ConfigMaps("default").Get(ctx, "tf-alibaba-eip", v1.GetOptions{})
	assert.NilError(t, err)
}
