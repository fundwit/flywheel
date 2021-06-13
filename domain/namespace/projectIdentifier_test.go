package namespace_test

import (
	"flywheel/domain/namespace"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("RecommendProjectIdentifier", func() {
	It("should be able to generate a recommend project identifier", func() {
		Expect(namespace.RecommendProjectIdentifier("Some Test Project")).To(Equal("STP"))
	})
})
