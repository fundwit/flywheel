package work_test

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestFlywheel(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "domain Suite")
}
