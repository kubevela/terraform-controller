package e2e

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"gotest.tools/assert"
	"k8s.io/klog/v2"
)

// Available is the available status of Configuration
const Available = "Available"

// Regression test for the e2e.
func Regression(t *testing.T, testcases []string, retryTimes int) {
	klog.Info("1. Applying Configuration")
	pwd, _ := os.Getwd()
	for _, p := range testcases {
		configuration := filepath.Join(pwd, "..", p)
		cmd := fmt.Sprintf("kubectl apply -f %s", configuration)
		err := exec.Command("bash", "-c", cmd).Start() // #nosec
		assert.NilError(t, err)
	}

	klog.Info("2. Checking Configurations status")
	for i := 0; i < retryTimes; i++ {
		var fields []string
		output, err := exec.Command("bash", "-c", "kubectl get configuration -A").Output()
		assert.NilError(t, err)

		lines := strings.Split(string(output), "\n")
		// delete the last line which is empty
		lines = lines[:len(lines)-1]

		if len(lines) < len(testcases)+1 {
			continue
		}

		var available = true
		for i, line := range lines {
			if i == 0 {
				continue
			}

			fields = strings.Fields(line)
			if len(fields) == 4 {
				if fields[2] != Available {
					available = false
					t.Logf("Configuration %s is not available", fields[1])
					break
				}
			}
		}
		if available {
			goto deletion
		}
		if i == retryTimes-1 {
			t.Error("Not all configurations are ready")
		}
		time.Sleep(time.Second * 5)
	}

deletion:
	klog.Info("3. Deleting Configuration")
	for _, p := range testcases {
		configuration := filepath.Join(pwd, "..", p)
		cmd := fmt.Sprintf("kubectl delete -f %s", configuration)
		err := exec.Command("bash", "-c", cmd).Start() // #nosec
		assert.NilError(t, err)
	}

	klog.Info("4. Checking Configuration is deleted")
	for i := 0; i < retryTimes; i++ {
		var (
			fields  []string
			existed bool
		)
		output, err := exec.Command("bash", "-c", "kubectl get configuration -A").Output()
		assert.NilError(t, err)

		lines := strings.Split(string(output), "\n")

		for j, line := range lines {
			if j == 0 {
				continue
			}
			existed = true

			fields = strings.Fields(line)
			if len(fields) == 4 {
				t.Logf("Retrying %d times. Configuration %s is deleting.", i+1, fields[1])
			}
		}
		if existed {
			if i == retryTimes-1 {
				t.Error("Configuration are not deleted")
			}

			time.Sleep(time.Second * 5)
			continue
		} else {
			break
		}
	}
}
