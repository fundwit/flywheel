package authority

import (
	"flywheel/domain"
	"fmt"
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

func (c Permissions) HasAnyProjectRole(projectId types.ID) bool {
	suffix := strings.ToLower("_" + projectId.String())

	for _, v := range c {
		if strings.HasSuffix(strings.ToLower(v), suffix) {
			return true
		}
	}
	return false
}

func (c Permissions) HasProjectRole(role string, projectId types.ID) bool {
	return c.HasRole(fmt.Sprintf("%s_%d", role, projectId))
}

func (c Permissions) HasGlobalViewRole() bool {
	for _, v := range c {
		if strings.HasPrefix(strings.ToLower(v), "system:") {
			return true
		}
	}
	return false
}

func (c Permissions) HasProjectViewPerm(projectId types.ID) bool {
	return c.HasGlobalViewRole() || c.HasAnyProjectRole(projectId)
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
