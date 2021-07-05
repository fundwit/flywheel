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

type VisiableProjects []domain.ProjectRole

func (c VisiableProjects) HasProject(projectId types.ID) bool {
	for _, v := range c {
		if v.ProjectID == projectId {
			return true
		}
	}
	return false
}
