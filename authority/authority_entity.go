package authority

import (
	"flywheel/domain"
	"strings"

	"github.com/fundwit/go-commons/types"
)

type Permissions []string

func (c Permissions) HasRole(role string) bool {
	for _, v := range c {
		if strings.EqualFold(v, role) {
			return true
		}
	}
	return false
}

func (c Permissions) HasGlobalViewRole() bool {
	for _, v := range c {
		if strings.HasPrefix(strings.ToLower(v), "system:") {
			return true
		}
	}
	return false
}

func (c Permissions) HasRolePrefix(prefix string) bool {
	for _, v := range c {
		if strings.HasPrefix(strings.ToLower(v), strings.ToLower(prefix)) {
			return true
		}
	}
	return false
}

func (c Permissions) HasProjectViewPerm(projectId types.ID) bool {
	return c.HasGlobalViewRole() || c.HasRoleSuffix(projectId.String())
}

func (c Permissions) HasRoleSuffix(suffix string) bool {
	for _, v := range c {
		if strings.HasSuffix(strings.ToLower(v), strings.ToLower(suffix)) {
			return true
		}
	}
	return false
}

type ProjectRoles []domain.ProjectRole

func (c ProjectRoles) HasProject(projectId types.ID) bool {
	for _, v := range c {
		if v.ProjectID == projectId {
			return true
		}
	}
	return false
}
