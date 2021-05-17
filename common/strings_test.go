package common_test

import (
	"flywheel/common"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Strings", func() {
	Describe("StringReader", func() {
		It("should be able to build a bytes.Reader from a string", func() {
			str := "test string"
			reader := common.StringReader(str)
			buf := make([]byte, len(str))
			n, err := reader.Read(buf)
			Expect(n).To(Equal(len(str)))
			Expect(err).To(BeNil())
			Expect(string(buf)).To(Equal(str))
		})
	})
})
