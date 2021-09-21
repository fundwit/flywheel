package session_test

import (
	"flywheel/session"
	"testing"

	. "github.com/onsi/gomega"
)

func TestHasRole(t *testing.T) {
	RegisterTestingT(t)

	t.Run("should work correctly", func(t *testing.T) {
		c := session.Session{}
		Expect(c.Perms.HasRole("aaa")).To(BeFalse())

		c = session.Session{Perms: []string{}}
		Expect(c.Perms.HasRole("aaa")).To(BeFalse())

		c = session.Session{Perms: []string{"bbb", "ccc"}}
		Expect(c.Perms.HasRole("aaa")).To(BeFalse())

		c = session.Session{Perms: []string{"bbb", "ccc"}}
		Expect(c.Perms.HasRole("ccc")).To(BeTrue())
	})
}

func TestHasRolePrefix(t *testing.T) {
	RegisterTestingT(t)

	t.Run("should work correctly", func(t *testing.T) {
		c := session.Session{}
		Expect(c.Perms.HasRolePrefix("aaa")).To(BeFalse())

		c = session.Session{Perms: []string{}}
		Expect(c.Perms.HasRolePrefix("aaa")).To(BeFalse())

		c = session.Session{Perms: []string{"bbb", "ccc"}}
		Expect(c.Perms.HasRolePrefix("aaa")).To(BeFalse())

		c = session.Session{Perms: []string{"bbb_123", "ccc_123"}}
		Expect(c.Perms.HasRolePrefix("ccc")).To(BeTrue())
	})
}
