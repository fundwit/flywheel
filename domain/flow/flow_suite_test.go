package flow_test

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestFlow(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Flow Suite")
}
