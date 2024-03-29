package testinfra

import (
	"flywheel/domain"
	"flywheel/session"
	"strings"

	"github.com/fundwit/go-commons/types"
)

// BuildSecCtx build session context
func BuildSecCtx(uid types.ID, perms ...string) *session.Session {
	visiableProjects := []domain.ProjectRole{}
	for _, perm := range perms {
		idx := strings.Index(perm, "_")
		if idx > 0 {
			role := perm[0:idx]
			projectId, err := types.ParseID(perm[idx+1:])
			if err != nil {
				continue
			}
			visiableProjects = append(visiableProjects, domain.ProjectRole{ProjectID: projectId, Role: role})
		}
	}

	return &session.Session{Identity: session.Identity{ID: uid, Name: "user-" + uid.String(), Nickname: "User " + uid.String()}, Perms: perms, ProjectRoles: visiableProjects}
}
