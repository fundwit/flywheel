package testinfra

import (
	"flywheel/domain"
	"flywheel/domain/work"
	"flywheel/security"
	"strings"

	"github.com/fundwit/go-commons/types"
	. "github.com/onsi/gomega"
)

// BuildSecCtx build security context
func BuildSecCtx(uid types.ID, perms ...string) *security.Context {
	visiableGroups := []domain.ProjectRole{}
	for _, perm := range perms {
		idx := strings.Index(perm, "_")
		if idx > 0 {
			role := perm[0:idx]
			projectId, err := types.ParseID(perm[idx+1:])
			if err != nil {
				continue
			}
			visiableGroups = append(visiableGroups, domain.ProjectRole{GroupID: projectId, Role: role})
		}
	}

	return &security.Context{Identity: security.Identity{ID: uid}, Perms: perms, ProjectRoles: visiableGroups}
}

// BuildWorker build work deital
func BuildWorker(m work.WorkManagerTraits, workName string, flowId, gid types.ID, secCtx *security.Context) *domain.WorkDetail {
	workCreation := &domain.WorkCreation{
		Name:             workName,
		GroupID:          gid,
		FlowID:           flowId,
		InitialStateName: domain.StatePending.Name,
	}
	detail, err := m.CreateWork(workCreation, secCtx)
	Expect(err).To(BeNil())
	Expect(detail).ToNot(BeNil())
	Expect(detail.StateName).To(Equal("PENDING"))
	return detail
}
