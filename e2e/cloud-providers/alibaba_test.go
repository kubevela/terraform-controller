package e2e

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestConfiguration(t *testing.T) {
	It("All Configurations should become `Available`", func() {
		pwd, _ := os.Getwd()
		configurations := []string{
			"examples/alibaba/eip/configuration_eip.yaml",
			"examples/alibaba/eip/configuration_eip_remote.yaml",
		}
		for _, c := range configurations {
			cmd := fmt.Sprintf("kubectl apply -f %s", filepath.Join(pwd, "..", "..", c))
			err := exec.Command("bash", "-c", cmd).Start()
			Expect(err).To(BeNil())
		}

		Eventually(func() bool {
			var fields []string
			output, err := Exec("bash", "-c", "kubectl get configuration")
			Expect(err).To(BeNil())
			for i, line := range strings.Split(output, "\n") {
				if i == 0 {
					continue
				}
				fields = strings.Fields(line)
				if len(fields) == 0 {
					continue
				}
				if !(len(fields) == 3 && fields[1] == "Available") {
					return false
				}
			}
			return true
		}, 180*time.Second, 1*time.Second).Should(BeTrue())

		for _, c := range configurations {
			cmd := fmt.Sprintf("kubectl delete -f %s", filepath.Join(pwd, "..", "..", c))
			err := exec.Command("bash", "-c", cmd).Start()
			Expect(err).To(BeNil())
		}
	})

}
