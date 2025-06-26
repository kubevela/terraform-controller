package util

import (
	"testing"

	ginkgo "github.com/onsi/ginkgo/v2"
	//revive:disable-next-line:dot-imports
	. "github.com/onsi/gomega"
)

func TestUtils(t *testing.T) {
	gomega.RegisterFailHandler(ginkgo.Fail)
	ginkgo.RunSpecs(t, "Utils Suite")
}
