package configurations

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
			"examples/alibaba/eip/configuration_eip_remote_subdirectory.yaml",
			"examples/alibaba/rds/configuration_hcl_rds.yaml",
		}
		for _, c := range configurations {
			cmd := fmt.Sprintf("kubectl apply -f %s", filepath.Join(pwd, "..", "..", c))
			err := exec.Command("bash", "-c", cmd).Start()
			Expect(err).To(BeNil())
		}

		Eventually(func() bool {
			var fields []string
			var available = true
			output, err := exec.Command("bash", "-c", "kubectl get configuration").Output()
			Expect(err).To(BeNil())
			fmt.Println("Checking Configuration status")
			fmt.Println(string(output))
			lines := strings.Split(string(output), "\n")
			if len(lines) != len(configurations)+2 {
				return false
			}
			for i, line := range lines {
				if i == 0 {
					continue
				}
				fields = strings.Fields(line)
				if len(fields) == 0 {
					continue
				}
				if !(len(fields) == 3 && fields[1] == "Available") {
					available = false
					return false
				}
			}
			return available
		}, 600*time.Second, 1*time.Second).Should(BeTrue())

		for _, c := range configurations {
			cmd := fmt.Sprintf("kubectl delete -f %s", filepath.Join(pwd, "..", "..", c))
			err := exec.Command("bash", "-c", cmd).Start()
			Expect(err).To(BeNil())
		}
	})

}
