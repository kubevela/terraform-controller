package controllernamespace_test

import (
	"testing"

	ginkgo "github.com/onsi/ginkgo/v2"
	gomega "github.com/onsi/gomega"
)

func TestE2e(t *testing.T) {
	gomega.RegisterFailHandler(ginkgo.Fail)
	defer ginkgo.GinkgoRecover()
	ginkgo.RunSpecs(t, "E2e Suite")
}
