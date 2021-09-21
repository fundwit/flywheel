package session_test

import (
	"flywheel/session"
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"
	. "github.com/onsi/gomega"
)

func TestFindSecurityContext(t *testing.T) {
	RegisterTestingT(t)

	t.Run("should work correctly", func(t *testing.T) {
		ginCtx := &gin.Context{Request: &http.Request{}}
		Expect(*session.ExtractSessionFromGinContext(ginCtx)).To(Equal(session.Session{Context: ginCtx.Request.Context()}))

		ginCtx.Set(session.KeySecCtx, "string value")
		Expect(*session.ExtractSessionFromGinContext(ginCtx)).To(Equal(session.Session{Context: ginCtx.Request.Context()}))

		ginCtx.Set(session.KeySecCtx, &session.Session{})
		Expect(*session.ExtractSessionFromGinContext(ginCtx)).To(Equal(session.Session{Context: ginCtx.Request.Context()}))

		ginCtx.Set(session.KeySecCtx, &session.Session{Token: "a token"})
		Expect(*session.ExtractSessionFromGinContext(ginCtx)).To(Equal(session.Session{Token: "a token", Context: ginCtx.Request.Context()}))
	})
}

func TestSaveSecurityContext(t *testing.T) {
	RegisterTestingT(t)

	t.Run("should work correctly", func(t *testing.T) {
		ginCtx := &gin.Context{}
		session.InjectSessionIntoGinContext(ginCtx, nil)
		_, found := ginCtx.Get(session.KeySecCtx)
		Expect(found).To(BeFalse())

		session.InjectSessionIntoGinContext(ginCtx, &session.Session{})
		_, found = ginCtx.Get(session.KeySecCtx)
		Expect(found).To(BeFalse())

		session.InjectSessionIntoGinContext(ginCtx, &session.Session{Token: "a token"})
		val, found := ginCtx.Get(session.KeySecCtx)
		Expect(found).To(BeTrue())
		secCtx, ok := val.(*session.Session)
		Expect(ok).To(BeTrue())
		Expect(*secCtx).To(Equal(session.Session{Token: "a token"}))
	})
}
