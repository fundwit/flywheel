package bizerror_test

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestBizerror(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Bizerror Suite")
}
