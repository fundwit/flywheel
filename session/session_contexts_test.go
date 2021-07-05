package session_test

import (
	"flywheel/session"
	"testing"

	"github.com/gin-gonic/gin"
	. "github.com/onsi/gomega"
)

func TestFindSecurityContext(t *testing.T) {
	RegisterTestingT(t)

	t.Run("should work correctly", func(t *testing.T) {
		ginCtx := &gin.Context{}
		Expect(session.FindSecurityContext(ginCtx)).To(BeNil())

		ginCtx.Set(session.KeySecCtx, "string value")
		Expect(session.FindSecurityContext(ginCtx)).To(BeNil())

		ginCtx.Set(session.KeySecCtx, &session.Context{})
		Expect(session.FindSecurityContext(ginCtx)).To(BeNil())

		ginCtx.Set(session.KeySecCtx, &session.Context{Token: "a token"})
		Expect(*session.FindSecurityContext(ginCtx)).To(Equal(session.Context{Token: "a token"}))
	})
}

func TestSaveSecurityContext(t *testing.T) {
	RegisterTestingT(t)

	t.Run("should work correctly", func(t *testing.T) {
		ginCtx := &gin.Context{}
		session.SaveSecurityContext(ginCtx, nil)
		_, found := ginCtx.Get(session.KeySecCtx)
		Expect(found).To(BeFalse())

		session.SaveSecurityContext(ginCtx, &session.Context{})
		_, found = ginCtx.Get(session.KeySecCtx)
		Expect(found).To(BeFalse())

		session.SaveSecurityContext(ginCtx, &session.Context{Token: "a token"})
		val, found := ginCtx.Get(session.KeySecCtx)
		Expect(found).To(BeTrue())
		secCtx, ok := val.(*session.Context)
		Expect(ok).To(BeTrue())
		Expect(*secCtx).To(Equal(session.Context{Token: "a token"}))
	})
}
