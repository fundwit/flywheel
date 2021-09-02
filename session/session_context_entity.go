package session

import (
	"flywheel/authority"
	"strings"
	"time"

	"github.com/fundwit/go-commons/types"
)

type Context struct {
	Token        string                 `json:"token"`
	Identity     Identity               `json:"identity"`
	Perms        authority.Permissions  `json:"perms"`
	ProjectRoles authority.ProjectRoles `json:"projectRoles"`

	SigningTime time.Time `json:"-"`
}

type Identity struct {
	ID       types.ID `json:"id"`
	Name     string   `json:"name"`
	Nickname string   `json:"nickname"`
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
