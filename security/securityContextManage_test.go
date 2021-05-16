package security_test

import (
	"flywheel/security"
	"github.com/gin-gonic/gin"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Security Context Manage", func() {
	Describe("FindSecurityContext", func() {
		It("should work as expected", func() {
			ctx := &gin.Context{}
			Expect(security.FindSecurityContext(ctx)).To(BeNil())

			ctx.Set(security.KeySecCtx, "string value")
			Expect(security.FindSecurityContext(ctx)).To(BeNil())

			ctx.Set(security.KeySecCtx, &security.Context{})
			Expect(security.FindSecurityContext(ctx)).To(BeNil())

			ctx.Set(security.KeySecCtx, &security.Context{Token: "a token"})
			Expect(*security.FindSecurityContext(ctx)).To(Equal(security.Context{Token: "a token"}))
		})
	})

	Describe("SaveSecurityContext", func() {
		It("should work as expected", func() {
			ctx := &gin.Context{}
			security.SaveSecurityContext(ctx, nil)
			_, found := ctx.Get(security.KeySecCtx)
			Expect(found).To(BeFalse())

			security.SaveSecurityContext(ctx, &security.Context{})
			_, found = ctx.Get(security.KeySecCtx)
			Expect(found).To(BeFalse())

			security.SaveSecurityContext(ctx, &security.Context{Token: "a token"})
			val, found := ctx.Get(security.KeySecCtx)
			Expect(found).To(BeTrue())
			secCtx, ok := val.(*security.Context)
			Expect(ok).To(BeTrue())
			Expect(*secCtx).To(Equal(security.Context{Token: "a token"}))
		})
	})
})
