package security

import (
	"flywheel/domain"
	"github.com/fundwit/go-commons/types"
	"strings"
)

type Context struct {
	Token        string               `json:"token"`
	Identity     Identity             `json:"identity"`
	Perms        []string             `json:"perms"`
	ProjectRoles []domain.ProjectRole `json:"groupRoles"`
}

type Identity struct {
	ID   types.ID `json:"id"`
	Name string   `json:"name"`
}

func (c *Context) HasRole(role string) bool {
	for _, v := range c.Perms {
		if strings.EqualFold(v, role) {
			return true
		}
	}
	return false
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
