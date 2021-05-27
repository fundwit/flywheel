package security

import (
	"flywheel/bizerror"
	"time"

	"github.com/fundwit/go-commons/types"
	"github.com/gin-gonic/gin"
	"github.com/patrickmn/go-cache"
)

const TokenExpiration = 24 * time.Hour

var TokenCache = cache.New(TokenExpiration, 1*time.Minute)

type LoginRequest struct {
	Name     string `json:"name"`
	Password string `json:"password"`
}

type User struct {
	ID     types.ID `json:"id"`
	Name   string   `json:"name"`
	Secret string   `json:"secret"`
}

type UserInfo struct {
	ID   types.ID `json:"id"`
	Name string   `json:"name"`
}

const KeySecCtx = "SecCtx"
const KeySecToken = "sec_token"

func FindSecurityContext(ctx *gin.Context) *Context {
	value, found := ctx.Get(KeySecCtx)
	if !found {
		return nil
	}
	secCtx, ok := value.(*Context)
	if !ok || secCtx.Token == "" {
		return nil
	}
	return secCtx
}

func SaveSecurityContext(ctx *gin.Context, secCtx *Context) {
	if secCtx != nil && secCtx.Token != "" {
		ctx.Set(KeySecCtx, secCtx)
	}
}

func SimpleAuthFilter() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		token, err := ctx.Cookie(KeySecToken)
		if err != nil {
			panic(bizerror.ErrUnauthenticated)
		}
		securityContextValue, found := TokenCache.Get(token)
		if !found {
			panic(bizerror.ErrUnauthenticated)
		}
		secCtx, ok := securityContextValue.(*Context)
		if !ok {
			panic(bizerror.ErrUnauthenticated)
		}
		SaveSecurityContext(ctx, secCtx)
		ctx.Next()
	}
}
