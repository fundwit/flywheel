package security_test

import (
	"flywheel/security"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("SecurityContext", func() {
	Describe("HasRole", func() {
		It("should work correctly", func() {
			c := security.Context{}
			Expect(c.HasRole("aaa")).To(BeFalse())

			c = security.Context{Perms: []string{}}
			Expect(c.HasRole("aaa")).To(BeFalse())

			c = security.Context{Perms: []string{"bbb", "ccc"}}
			Expect(c.HasRole("aaa")).To(BeFalse())

			c = security.Context{Perms: []string{"bbb", "ccc"}}
			Expect(c.HasRole("ccc")).To(BeTrue())
		})
	})
	Describe("HasRolePrefix", func() {
		It("should work correctly", func() {
			c := security.Context{}
			Expect(c.HasRolePrefix("aaa")).To(BeFalse())

			c = security.Context{Perms: []string{}}
			Expect(c.HasRolePrefix("aaa")).To(BeFalse())

			c = security.Context{Perms: []string{"bbb", "ccc"}}
			Expect(c.HasRolePrefix("aaa")).To(BeFalse())

			c = security.Context{Perms: []string{"bbb_123", "ccc_123"}}
			Expect(c.HasRolePrefix("ccc")).To(BeTrue())
		})
	})
})
