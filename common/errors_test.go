package common_test

import (
	"flywheel/common"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Errors", func() {
	Describe("ErrBadParam", func() {
		Describe("Error", func() {
			It("should return default message if cause is nil", func() {
				err := common.ErrBadParam{}
				Expect(err.Error()).To(Equal("common.bad_param"))
			})
			It("should invoke the Error() function of cause property if cause is not nil", func() {
				err := common.ErrBadParam{Cause: common.ErrForbidden}
				Expect(err.Error()).To(Equal("forbidden"))
			})
		})
	})
})
