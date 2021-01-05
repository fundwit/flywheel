package security

import (
	"crypto/sha256"
	"encoding/hex"
	"flywheel/servehttp"
	"github.com/fundwit/go-commons/types"
	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"github.com/google/uuid"
	"github.com/jinzhu/gorm"
	"github.com/patrickmn/go-cache"
	"net/http"
	"time"
)

const TokenExpiration = 24 * time.Hour

var TokenCache = cache.New(TokenExpiration, 1*time.Minute)
var DB *gorm.DB

type LoginRequest struct {
	Name     string `json:"name"`
	Password string `json:"password"`
}

type User struct {
	ID     types.ID `json:"id"`
	Name   string   `json:"name"`
	Secret string   `json:"secret"`
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

func UserInfoQueryHandler(c *gin.Context) {
	secCtx := FindSecurityContext(c)
	if secCtx == nil {
		c.JSON(http.StatusUnauthorized, &servehttp.ErrorBody{Code: "common.unauthenticated", Message: "login failed"})
		return
	}
	c.JSON(http.StatusOK, &secCtx.Identity)
}

func SimpleLoginHandler(c *gin.Context) {
	login := LoginRequest{}
	if err := c.ShouldBindBodyWith(&login, binding.JSON); err != nil {
		c.JSON(http.StatusBadRequest, &servehttp.ErrorBody{Code: "common.bad_param", Message: err.Error()})
		return
	}
	identity := Identity{}
	if err := DB.Model(&User{}).Where(&User{Name: login.Name, Secret: HashSha256(login.Password)}).Scan(&identity).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusUnauthorized, &servehttp.ErrorBody{Code: "common.unauthenticated", Message: "unauthenticated"})
			return
		}
		c.JSON(http.StatusInternalServerError, &servehttp.ErrorBody{Code: "common.internal_server_error", Message: err.Error()})
		return
	}
	token := uuid.New().String()
	securityContext := Context{Token: token, Identity: identity}
	TokenCache.Set(token, &securityContext, cache.DefaultExpiration)

	c.SetCookie(KeySecToken, token, int(TokenExpiration/time.Second), "/", "", false, false)
	c.JSON(http.StatusOK, &identity)
}

func SimpleAuthFilter() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		token, err := ctx.Cookie(KeySecToken)
		if err != nil {
			ctx.JSON(http.StatusUnauthorized, &servehttp.ErrorBody{Code: "common.unauthenticated", Message: "unauthenticated"})
			ctx.Abort()
			return
		}
		securityContextValue, found := TokenCache.Get(token)
		if !found {
			ctx.JSON(http.StatusUnauthorized, &servehttp.ErrorBody{Code: "common.unauthenticated", Message: "unauthenticated"})
			ctx.Abort()
			return
		}
		secCtx, ok := securityContextValue.(*Context)
		if !ok {
			ctx.JSON(http.StatusUnauthorized, &servehttp.ErrorBody{Code: "common.unauthenticated", Message: "unauthenticated"})
			ctx.Abort()
			return
		}
		SaveSecurityContext(ctx, secCtx)
		ctx.Next()
	}
}

func HashSha256(raw string) string {
	h := sha256.New()
	h.Write([]byte(raw))
	sum := h.Sum(nil)
	return hex.EncodeToString(sum)
}
