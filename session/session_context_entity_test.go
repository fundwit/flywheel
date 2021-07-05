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
		Expect(c.HasRole("aaa")).To(BeFalse())

		c = session.Context{Perms: []string{}}
		Expect(c.HasRole("aaa")).To(BeFalse())

		c = session.Context{Perms: []string{"bbb", "ccc"}}
		Expect(c.HasRole("aaa")).To(BeFalse())

		c = session.Context{Perms: []string{"bbb", "ccc"}}
		Expect(c.HasRole("ccc")).To(BeTrue())
	})
}

func TestHasRolePrefix(t *testing.T) {
	RegisterTestingT(t)

	t.Run("should work correctly", func(t *testing.T) {
		c := session.Context{}
		Expect(c.HasRolePrefix("aaa")).To(BeFalse())

		c = session.Context{Perms: []string{}}
		Expect(c.HasRolePrefix("aaa")).To(BeFalse())

		c = session.Context{Perms: []string{"bbb", "ccc"}}
		Expect(c.HasRolePrefix("aaa")).To(BeFalse())

		c = session.Context{Perms: []string{"bbb_123", "ccc_123"}}
		Expect(c.HasRolePrefix("ccc")).To(BeTrue())
	})
}
