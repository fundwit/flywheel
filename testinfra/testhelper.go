package testinfra

import (
	"flywheel/domain"
	"flywheel/domain/work"
	"flywheel/security"
	"github.com/fundwit/go-commons/types"
	. "github.com/onsi/gomega"
)

func BuildSecCtx(uid types.ID, perms []string) *security.Context {
	return &security.Context{Identity: security.Identity{ID: uid}, Perms: perms}
}

func BuildWorker(m work.WorkManagerTraits, workName string, gid types.ID, secCtx *security.Context) *domain.WorkDetail {
	workCreation := &domain.WorkCreation{
		Name:    workName,
		GroupID: gid,
	}
	detail, err := m.CreateWork(workCreation, secCtx)
	Expect(err).To(BeNil())
	Expect(detail).ToNot(BeNil())
	Expect(detail.StateName).To(Equal("PENDING"))
	return detail
}
