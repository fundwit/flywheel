package security

import (
	"strings"
	"time"

	"github.com/fundwit/go-commons/types"
)

type Context struct {
	Token        string           `json:"token"`
	Identity     Identity         `json:"identity"`
	Perms        Permissions      `json:"perms"`
	ProjectRoles VisiableProjects `json:"projectRoles"`

	SigningTime time.Time `json:"-"`
}

type Identity struct {
	ID       types.ID `json:"id"`
	Name     string   `json:"name"`
	Nickname string   `json:"nickname"`
}

func (c *Context) HasRole(role string) bool {
	return c.Perms.HasRole(role)
}

func (c *Context) HasRolePrefix(prefix string) bool {
	for _, v := range c.Perms {
		if strings.HasPrefix(strings.ToLower(v), strings.ToLower(prefix)) {
			return true
		}
	}
	return false
}

func (c *Context) HasRoleSuffix(suffix string) bool {
	for _, v := range c.Perms {
		if strings.HasSuffix(strings.ToLower(v), strings.ToLower(suffix)) {
			return true
		}
	}
	return false
}

// VisibleProjects  parse visible project ids from Context.Perms
func (c *Context) VisibleProjects() []types.ID {
	var projectIds []types.ID
	for _, v := range c.Perms {
		pairs := strings.Split(v, "_")
		if len(pairs) == 2 {
			id, err := types.ParseID(pairs[1])
			if err != nil {
				continue
			}
			projectIds = append(projectIds, id)
		}
	}
	if projectIds == nil {
		return []types.ID{}
	}
	return projectIds
}
