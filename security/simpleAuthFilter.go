package security

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"flywheel/common"
	"flywheel/domain"
	"flywheel/persistence"
	"fmt"
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

func UserInfoQueryHandler(c *gin.Context) {
	secCtx := FindSecurityContext(c)
	if secCtx == nil {
		c.JSON(http.StatusUnauthorized, &common.ErrorBody{Code: "common.unauthenticated", Message: "login failed"})
		return
	}
	c.JSON(http.StatusOK, &secCtx)
}

func RegisterSessionHandler(r *gin.Engine) {
	g := r.Group("/v1/sessions")
	g.POST("", SimpleLoginHandler)
	g.DELETE("", SimpleLogoutHandler)
}

func SimpleLogoutHandler(c *gin.Context) {
	token, _ := c.Cookie(KeySecToken) // ErrNoCookie
	if token != "" {
		TokenCache.Delete(token)
	}
	c.SetCookie(KeySecToken, "", -1, "/", "", false, false)
	c.AbortWithStatus(http.StatusNoContent)
}

func SimpleLoginHandler(c *gin.Context) {
	login := LoginRequest{}
	if err := c.ShouldBindBodyWith(&login, binding.JSON); err != nil {
		c.JSON(http.StatusBadRequest, &common.ErrorBody{Code: "common.bad_param", Message: err.Error()})
		return
	}
	identity := Identity{}
	if err := persistence.ActiveDataSourceManager.GormDB().Model(&User{}).Where(&User{Name: login.Name, Secret: HashSha256(login.Password)}).Scan(&identity).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusUnauthorized, &common.ErrorBody{Code: "common.unauthenticated", Message: "unauthenticated"})
			return
		}
		c.JSON(http.StatusInternalServerError, &common.ErrorBody{Code: "common.internal_server_error", Message: err.Error()})
		return
	}
	token := uuid.New().String()
	perms, groupRoles := LoadPerms(identity.ID)
	securityContext := Context{Token: token, Identity: identity, Perms: perms, GroupRoles: groupRoles}
	TokenCache.Set(token, &securityContext, cache.DefaultExpiration)

	c.SetCookie(KeySecToken, token, int(TokenExpiration/time.Second), "/", "", false, false)
	c.JSON(http.StatusOK, &securityContext)
}

// as a simple initial solution, we use group member relationship as the metadata of permissions
func LoadPerms(uid types.ID) ([]string, []domain.GroupRole) {
	var gms []domain.GroupMember
	if err := persistence.ActiveDataSourceManager.GormDB().Model(&domain.GroupMember{}).Where(&domain.GroupMember{MemberId: uid}).Scan(&gms).Error; err != nil {
		panic(err)
	}
	var roles []string
	var groupRoles []domain.GroupRole
	var groupIds []types.ID
	for _, gm := range gms {
		roles = append(roles, fmt.Sprintf("%s_%d", gm.Role, gm.GroupID))
		groupRoles = append(groupRoles, domain.GroupRole{Role: gm.Role, GroupID: gm.GroupID})
		groupIds = append(groupIds, gm.GroupID)
	}

	m := map[types.ID]domain.Group{}
	if len(groupIds) > 0 {
		var groups []domain.Group
		if err := persistence.ActiveDataSourceManager.GormDB().Model(&domain.Group{}).Where("id in (?)", groupIds).Scan(&groups).Error; err != nil {
			panic(err)
		}
		for _, group := range groups {
			m[group.ID] = group
		}
	}

	for i := 0; i < len(groupRoles); i++ {
		groupRole := groupRoles[i]

		group := m[groupRole.GroupID]
		if (group == domain.Group{}) {
			panic(errors.New("group " + groupRole.GroupID.String() + " is not exist"))
		}

		groupRole.GroupName = group.Name
		groupRole.GroupIdentifier = group.Identifier

		groupRoles[i] = groupRole
	}

	if roles == nil {
		roles = []string{}
	}
	if groupRoles == nil {
		groupRoles = []domain.GroupRole{}
	}
	return roles, groupRoles
}

func SimpleAuthFilter() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		token, err := ctx.Cookie(KeySecToken)
		if err != nil {
			ctx.JSON(http.StatusUnauthorized, &common.ErrorBody{Code: "common.unauthenticated", Message: "unauthenticated"})
			ctx.Abort()
			return
		}
		securityContextValue, found := TokenCache.Get(token)
		if !found {
			ctx.JSON(http.StatusUnauthorized, &common.ErrorBody{Code: "common.unauthenticated", Message: "unauthenticated"})
			ctx.Abort()
			return
		}
		secCtx, ok := securityContextValue.(*Context)
		if !ok {
			ctx.JSON(http.StatusUnauthorized, &common.ErrorBody{Code: "common.unauthenticated", Message: "unauthenticated"})
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
