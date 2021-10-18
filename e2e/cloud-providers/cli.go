package e2e

import (
	"os/exec"
	"time"

	"github.com/onsi/ginkgo"
	"github.com/onsi/gomega/gexec"
)

// Exec executes a command
func Exec(cmd string, args ...string) (string, error) {
	var output []byte
	session, err := asyncExec(cmd, args...)
	if err != nil {
		return string(output), err
	}
	s := session.Wait(60 * time.Second)
	return string(s.Out.Contents()) + string(s.Err.Contents()), nil
}

func asyncExec(cmd string, args ...string) (*gexec.Session, error) {
	command := exec.Command(cmd, args...)
	session, err := gexec.Start(command, ginkgo.GinkgoWriter, ginkgo.GinkgoWriter)
	return session, err
}
