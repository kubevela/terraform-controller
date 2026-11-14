package container

import (
	"strings"
	"testing"

	"github.com/oam-dev/terraform-controller/api/types"
)

// Test_getCloneCommand_GitCredential guards against regressing to the bare
// `eval `ssh-agent`` form: on OpenSSH 9.x+ (e.g. alpine/git:latest) ssh-agent
// defaults its socket to $HOME/.ssh/agent, but that path is a Secret volume
// mount and always read-only, so ssh-agent fails to start. Binding to an
// explicit socket path under /tmp avoids that.
func Test_getCloneCommand_GitCredential(t *testing.T) {
	a := &Assembler{
		GitCredential: true,
		Git: types.Git{
			URL: "git@git-server:simple-terraform-module.git",
		},
	}

	got := a.getCloneCommand()
	command := strings.Join(got, " ")

	if !strings.Contains(command, "ssh-agent -a /tmp/ssh-agent.sock") {
		t.Errorf("expected clone command to bind ssh-agent to an explicit writable socket path, got: %s", command)
	}
	if strings.Contains(command, "eval `ssh-agent`") {
		t.Errorf("clone command must not use bare `ssh-agent` (breaks under read-only /root/.ssh Secret mount with OpenSSH 9.x+), got: %s", command)
	}
}

func Test_getCheckoutObj(t *testing.T) {
	tests := []struct {
		name string
		ref  types.GitRef
		want string
	}{
		{
			name: "only branch",
			ref: types.GitRef{
				Branch: "feature",
			},
			want: "feature",
		},
		{
			name: "tag take precedence over branch",
			ref: types.GitRef{
				Branch: "feature",
				Tag:    "v1.0.0",
			},
			want: "v1.0.0",
		},
		{
			name: "commit take precedence over tag",
			ref: types.GitRef{
				Branch: "feature",
				Tag:    "v1.0.0",
				Commit: "123456",
			},
			want: "123456",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := getCheckoutObj(tt.ref); got != tt.want {
				t.Errorf("getCheckoutObj() = %v, want %v", got, tt.want)
			}
		})
	}
}
