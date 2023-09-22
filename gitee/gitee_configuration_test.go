package gitee

import (
	"testing"

	"github.com/oam-dev/terraform-controller/e2e/normal"
)

var (
	giteeConfigurationsRegression = []string{
		"gitee/alibaba/cs/serverless-kubernetes/configuration_ask.yaml",
		"gitee/alibaba/cs/dedicated-kubernetes/configuration_ack.yaml",
	}
)

func TestGiteeConfigurationRegression(t *testing.T) {
	var retryTimes = 240

	normal.Regression(t, giteeConfigurationsRegression, retryTimes)
}
