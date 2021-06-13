package work_test

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestDomainWork(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "domain Suite")
}
