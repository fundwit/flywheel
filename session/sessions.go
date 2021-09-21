package session

import (
	"flywheel/bizerror"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/patrickmn/go-cache"
)

const TokenExpiration = 24 * time.Hour

var TokenCache = cache.New(TokenExpiration, 1*time.Minute)

type LoginRequest struct {
	Name     string `json:"name"`
	Password string `json:"password"`
}

const KeySecCtx = "SecCtx"
const KeySecToken = "sec_token"

func ExtractSessionFromGinContext(ctx *gin.Context) *Session {
	value, found := ctx.Get(KeySecCtx)
	if !found {
		return &Session{Context: ctx.Request.Context()}
	}
	s0, ok := value.(*Session)
	if !ok || s0.Token == "" {
		return &Session{Context: ctx.Request.Context()}
	}
	s := s0.Clone()
	s.Context = ctx.Request.Context() // trace context
	return &s
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
		secCtx, ok := securityContextValue.(*Session)
		if !ok {
			panic(bizerror.ErrUnauthenticated)
		}
		InjectSessionIntoGinContext(ctx, secCtx)
		ctx.Next()
	}
}

func InjectSessionIntoGinContext(ctx *gin.Context, secCtx *Session) {
	if secCtx != nil && secCtx.Token != "" {
		ctx.Set(KeySecCtx, secCtx)
	}
}
