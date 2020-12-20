package servehttp_test

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestServehttp(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Servehttp Suite")
}
