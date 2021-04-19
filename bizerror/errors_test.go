package bizerror_test

import (
	"flywheel/bizerror"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Errors", func() {
	Describe("ErrBadParam", func() {
		Describe("Error", func() {
			It("should return default message if cause is nil", func() {
				err := bizerror.ErrBadParam{}
				Expect(err.Error()).To(Equal("common.bad_param"))
			})
			It("should invoke the Error() function of cause property if cause is not nil", func() {
				err := bizerror.ErrBadParam{Cause: bizerror.ErrForbidden}
				Expect(err.Error()).To(Equal("forbidden"))
			})
		})
	})
})
