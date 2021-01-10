package security

import (
	"github.com/fundwit/go-commons/types"
	"strings"
)

type Context struct {
	Token    string   `json:"token"`
	Identity Identity `json:"identity"`
	Perms    []string `json:"-"`
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

func (c *Context) VisibleGroups() []types.ID {
	var groupIds []types.ID
	for _, v := range c.Perms {
		pairs := strings.Split(v, "_")
		if len(pairs) == 2 {
			id, err := types.ParseID(pairs[1])
			if err != nil {
				continue
			}
			groupIds = append(groupIds, id)
		}
	}
	if groupIds == nil {
		return []types.ID{}
	}
	return groupIds
}
