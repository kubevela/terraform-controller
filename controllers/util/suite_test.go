package util

import (
	"testing"

	//revive:disable-next-line:dot-imports
	. "github.com/onsi/ginkgo/v2"
	//revive:disable-next-line:dot-imports
	. "github.com/onsi/gomega"
)

func TestUtils(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Utils Suite")
}
