package configuration

import (
	"testing"
)

func TestReplaceTerraformSource(t *testing.T) {
	testcases := []struct {
		remote        string
		githubBlocked string
		expected      string
	}{
		{
			remote:        "",
			expected:      "",
			githubBlocked: "xxx",
		},
		{
			remote:        "https://github.com/kubevela-contrib/terraform-modules.git",
			expected:      "https://github.com/kubevela-contrib/terraform-modules.git",
			githubBlocked: "false",
		},
		{
			remote:        "https://github.com/kubevela-contrib/terraform-modules.git",
			githubBlocked: "true",
			expected:      "https://gitee.com/kubevela-contrib/terraform-modules.git",
		},
		{
			remote:        "abc",
			githubBlocked: "true",
			expected:      "abc",
		},
		{
			remote:        "",
			githubBlocked: "true",
			expected:      "",
		},
		{
			remote:        "https://github.com-https://github.com",
			githubBlocked: "true",
			expected:      "https://gitee.com-https://github.com",
		},
	}

	for _, tc := range testcases {
		t.Run(tc.remote, func(t *testing.T) {
			actual := ReplaceTerraformSource(tc.remote, tc.githubBlocked)
			if actual != tc.expected {
				t.Errorf("expected %s, got %s", tc.expected, actual)
			}
		})
	}
}
