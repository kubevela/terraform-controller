package controllernamespace_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestE2e(t *testing.T) {
	RegisterFailHandler(Fail)
	defer GinkgoRecover()
	RunSpecs(t, "E2e Suite")
}
