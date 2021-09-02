package session_test

import (
	"flywheel/session"
	"testing"

	. "github.com/onsi/gomega"
)

func TestHasRole(t *testing.T) {
	RegisterTestingT(t)

	t.Run("should work correctly", func(t *testing.T) {
		c := session.Context{}
		Expect(c.Perms.HasRole("aaa")).To(BeFalse())

		c = session.Context{Perms: []string{}}
		Expect(c.Perms.HasRole("aaa")).To(BeFalse())

		c = session.Context{Perms: []string{"bbb", "ccc"}}
		Expect(c.Perms.HasRole("aaa")).To(BeFalse())

		c = session.Context{Perms: []string{"bbb", "ccc"}}
		Expect(c.Perms.HasRole("ccc")).To(BeTrue())
	})
}

func TestHasRolePrefix(t *testing.T) {
	RegisterTestingT(t)

	t.Run("should work correctly", func(t *testing.T) {
		c := session.Context{}
		Expect(c.Perms.HasRolePrefix("aaa")).To(BeFalse())

		c = session.Context{Perms: []string{}}
		Expect(c.Perms.HasRolePrefix("aaa")).To(BeFalse())

		c = session.Context{Perms: []string{"bbb", "ccc"}}
		Expect(c.Perms.HasRolePrefix("aaa")).To(BeFalse())

		c = session.Context{Perms: []string{"bbb_123", "ccc_123"}}
		Expect(c.Perms.HasRolePrefix("ccc")).To(BeTrue())
	})
}
