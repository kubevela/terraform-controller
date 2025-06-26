package controllernamespace_test

import (
	"testing"
	//revive:disable-next-line:dot-imports
	. "github.com/onsi/ginkgo/v2"
	//revive:disable-next-line:dot-imports
	. "github.com/onsi/gomega"
)

func TestE2e(t *testing.T) {
	RegisterFailHandler(Fail)
	defer GinkgoRecover()
	RunSpecs(t, "E2e Suite")
}
