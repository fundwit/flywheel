package namespace_test

import (
	"flywheel/account"
	"flywheel/bizerror"
	"flywheel/domain"
	"flywheel/domain/namespace"
	"flywheel/persistence"
	"flywheel/testinfra"
	"time"

	"github.com/fundwit/go-commons/types"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("ProjectMembers", func() {
	var (
		testDatabase *testinfra.TestDatabase
	)
	BeforeEach(func() {
		testDatabase = testinfra.StartMysqlTestDatabase("flywheel")
		Expect(testDatabase.DS.GormDB().AutoMigrate(&domain.Project{}, &domain.ProjectMember{}, &account.User{}).Error).To(BeNil())
		persistence.ActiveDataSourceManager = testDatabase.DS

		namespace.QueryProjectNamesFunc = namespace.QueryProjectNames
		namespace.QueryAccountNamesFunc = account.QueryAccountNames
		namespace.DetailProjectMembersFunc = namespace.DetailProjectMembers
	})
	AfterEach(func() {
		testinfra.StopMysqlTestDatabase(testDatabase)
	})

	Describe("CreateProjectMember", func() {
		It("should be able to create project member", func() {
			Expect(testDatabase.DS.GormDB().Save(&account.User{ID: 111, Name: "user111", Secret: "xxxx"}).Error).To(BeNil())
			sec := testinfra.BuildSecCtx(types.ID(111), account.SystemAdminPermission.ID)
			g, err := namespace.CreateProject(&domain.ProjectCreating{Name: "demo", Identifier: "DEM"}, sec)
			Expect(err).To(BeNil())
			Expect(testDatabase.DS.GormDB().Save(&account.User{ID: 222, Name: "test", Secret: "xxxx"}).Error).To(BeNil())

			// system administrators can create project member: 222-guest
			Expect(namespace.CreateProjectMember(
				&domain.ProjectMemberCreation{ProjectID: g.ID, MemberId: 222, Role: "guest"}, sec)).To(BeNil())

			var q []domain.ProjectMember
			Expect(testDatabase.DS.GormDB().Find(&q).Error).To(BeNil())
			Expect(q).ToNot(BeNil())
			Expect(len(q)).To(Equal(2))

			Expect(q[0].MemberId).To(Equal(sec.Identity.ID))
			Expect(q[0].ProjectId).To(Equal(g.ID))
			Expect(q[0].Role).To(Equal(domain.ProjectRoleManager))

			Expect(q[1].MemberId).To(Equal(types.ID(222)))
			Expect(q[1].ProjectId).To(Equal(g.ID))
			Expect(q[1].Role).To(Equal("guest"))
			Expect(q[1].CreateTime.Before(time.Now()) && q[1].CreateTime.After(g.CreateTime)).To(BeTrue())

			// repeat creating will update role
			Expect(namespace.CreateProjectMember(
				&domain.ProjectMemberCreation{ProjectID: g.ID, MemberId: 222, Role: "watcher"}, sec)).To(BeNil())
			q = []domain.ProjectMember{}
			Expect(testDatabase.DS.GormDB().Find(&q).Error).To(BeNil())
			Expect(q).ToNot(BeNil())
			Expect(len(q)).To(Equal(2))

			Expect(q[0].MemberId).To(Equal(sec.Identity.ID))
			Expect(q[0].ProjectId).To(Equal(g.ID))
			Expect(q[0].Role).To(Equal(domain.ProjectRoleManager))

			Expect(q[1].MemberId).To(Equal(types.ID(222)))
			Expect(q[1].ProjectId).To(Equal(g.ID))
			Expect(q[1].Role).To(Equal("watcher"))
			Expect(q[1].CreateTime.Before(time.Now()) && q[1].CreateTime.After(g.CreateTime)).To(BeTrue())

			// project manager can create project member: 333-guest
			Expect(testDatabase.DS.GormDB().Save(&account.User{ID: 333, Name: "test333", Secret: "xxxx"}).Error).To(BeNil())
			Expect(namespace.CreateProjectMember(
				&domain.ProjectMemberCreation{ProjectID: g.ID, MemberId: 333, Role: domain.ProjectRoleManager},
				testinfra.BuildSecCtx(444, domain.ProjectRoleManager+"_"+g.ID.String()))).To(BeNil())

			// system administrators can grant themselfs
			Expect(namespace.CreateProjectMember(
				&domain.ProjectMemberCreation{ProjectID: g.ID, MemberId: sec.Identity.ID, Role: "guest"}, sec)).To(BeNil())
			q = []domain.ProjectMember{}
			Expect(testDatabase.DS.GormDB().Where(domain.ProjectMember{MemberId: sec.Identity.ID}).Find(&q).Error).To(BeNil())
			Expect(q).ToNot(BeNil())
			Expect(len(q)).To(Equal(1))
			Expect(q[0].Role).To(Equal("guest"))

			// non system administrators can not grant themselfs
			Expect(namespace.CreateProjectMember(
				&domain.ProjectMemberCreation{ProjectID: g.ID, MemberId: 444, Role: "guest"},
				testinfra.BuildSecCtx(444, domain.ProjectRoleManager+"_"+g.ID.String()))).To(Equal(bizerror.ErrProjectMemberSelfGrant))
		})

		It("non non-administrator nor non-project manager can create project member", func() {
			sec := testinfra.BuildSecCtx(types.ID(1), account.SystemAdminPermission.ID)
			g, err := namespace.CreateProject(&domain.ProjectCreating{Name: "demo", Identifier: "DEM"}, sec)
			Expect(err).To(BeNil())
			Expect(testDatabase.DS.GormDB().Save(&account.User{ID: 2, Name: "test", Secret: "xxxx"}).Error).To(BeNil())

			Expect(namespace.CreateProjectMember(
				&domain.ProjectMemberCreation{ProjectID: g.ID, MemberId: 2, Role: "guest"},
				testinfra.BuildSecCtx(333, "guest_"+g.ID.String()))).To(Equal(bizerror.ErrForbidden))
		})
	})

	Describe("DetailProjectMembers", func() {
		It("should return correct project member details", func() {
			details, err := namespace.DetailProjectMembers(nil)
			Expect(err).To(BeNil())
			Expect(len(*details)).To(BeZero())

			details, err = namespace.DetailProjectMembers(&[]domain.ProjectMember{})
			Expect(err).To(BeNil())
			Expect(len(*details)).To(BeZero())

			namespace.QueryAccountNamesFunc = func(ids []types.ID) (map[types.ID]string, error) {
				result := map[types.ID]string{}
				for _, id := range ids {
					result[id] = "u" + id.String()
				}
				return result, nil
			}
			namespace.QueryProjectNamesFunc = func(ids []types.ID) (map[types.ID]string, error) {
				result := map[types.ID]string{}
				for _, id := range ids {
					result[id] = "p" + id.String()
				}
				return result, nil
			}

			details, err = namespace.DetailProjectMembers(&[]domain.ProjectMember{
				{ProjectId: 1, MemberId: 10},
				{ProjectId: 2, MemberId: 20},
			})
			Expect(err).To(BeNil())
			Expect(*details).To(Equal([]domain.ProjectMemberDetail{
				{ProjectMember: domain.ProjectMember{ProjectId: 1, MemberId: 10}, ProjectName: "p1", MemberName: "u10"},
				{ProjectMember: domain.ProjectMember{ProjectId: 2, MemberId: 20}, ProjectName: "p2", MemberName: "u20"},
			}))

			namespace.QueryAccountNamesFunc = func(ids []types.ID) (map[types.ID]string, error) {
				return map[types.ID]string{}, nil
			}
			namespace.QueryProjectNamesFunc = func(ids []types.ID) (map[types.ID]string, error) {
				return map[types.ID]string{}, nil
			}
			details, err = namespace.DetailProjectMembers(&[]domain.ProjectMember{
				{ProjectId: 1, MemberId: 10},
				{ProjectId: 2, MemberId: 20},
			})
			Expect(err).To(BeNil())
			Expect(*details).To(Equal([]domain.ProjectMemberDetail{
				{ProjectMember: domain.ProjectMember{ProjectId: 1, MemberId: 10}, ProjectName: "Unknown", MemberName: "Unknown"},
				{ProjectMember: domain.ProjectMember{ProjectId: 2, MemberId: 20}, ProjectName: "Unknown", MemberName: "Unknown"},
			}))
		})
	})

	Describe("QueryProjectMemberDetails", func() {
		It("system admin can query members in all projects", func() {
			namespace.DetailProjectMembersFunc = func(pms *[]domain.ProjectMember) (*[]domain.ProjectMemberDetail, error) {
				var result []domain.ProjectMemberDetail
				for _, pm := range *pms {
					result = append(result, domain.ProjectMemberDetail{ProjectMember: pm,
						ProjectName: "p" + pm.ProjectId.String(), MemberName: "u" + pm.MemberId.String()})
				}
				return &result, nil
			}

			db := testDatabase.DS.GormDB()
			t := time.Date(2021, 5, 20, 0, 0, 0, 0, time.Local)
			Expect(db.Save(&domain.ProjectMember{ProjectId: 1, MemberId: 10, Role: "admin", CreateTime: t}).Error).To(BeNil())
			Expect(db.Save(&domain.ProjectMember{ProjectId: 1, MemberId: 20, Role: "guest", CreateTime: t}).Error).To(BeNil())
			Expect(db.Save(&domain.ProjectMember{ProjectId: 2, MemberId: 10, Role: "admin", CreateTime: t}).Error).To(BeNil())
			Expect(db.Save(&domain.ProjectMember{ProjectId: 2, MemberId: 30, Role: "admin", CreateTime: t}).Error).To(BeNil())

			result, err := namespace.QueryProjectMemberDetails(&domain.ProjectMemberQuery{},
				testinfra.BuildSecCtx(100, account.SystemAdminPermission.ID))
			Expect(err).To(BeNil())
			Expect(len(*result)).To(Equal(4))

			Expect(*result).To(Equal([]domain.ProjectMemberDetail{
				{ProjectMember: domain.ProjectMember{ProjectId: 1, MemberId: 10, Role: "admin", CreateTime: t},
					ProjectName: "p1", MemberName: "u10"},
				{ProjectMember: domain.ProjectMember{ProjectId: 1, MemberId: 20, Role: "guest", CreateTime: t},
					ProjectName: "p1", MemberName: "u20"},
				{ProjectMember: domain.ProjectMember{ProjectId: 2, MemberId: 10, Role: "admin", CreateTime: t},
					ProjectName: "p2", MemberName: "u10"},
				{ProjectMember: domain.ProjectMember{ProjectId: 2, MemberId: 30, Role: "admin", CreateTime: t},
					ProjectName: "p2", MemberName: "u30"},
			}))

			projectId := types.ID(1)
			result, err = namespace.QueryProjectMemberDetails(&domain.ProjectMemberQuery{ProjectID: &projectId},
				testinfra.BuildSecCtx(100, account.SystemAdminPermission.ID))
			Expect(err).To(BeNil())
			Expect(len(*result)).To(Equal(2))

			Expect(*result).To(Equal([]domain.ProjectMemberDetail{
				{ProjectMember: domain.ProjectMember{ProjectId: 1, MemberId: 10, Role: "admin", CreateTime: t},
					ProjectName: "p1", MemberName: "u10"},
				{ProjectMember: domain.ProjectMember{ProjectId: 1, MemberId: 20, Role: "guest", CreateTime: t},
					ProjectName: "p1", MemberName: "u20"},
			}))

			memberId := types.ID(10)
			result, err = namespace.QueryProjectMemberDetails(&domain.ProjectMemberQuery{MemberID: &memberId},
				testinfra.BuildSecCtx(100, account.SystemAdminPermission.ID))
			Expect(err).To(BeNil())
			Expect(len(*result)).To(Equal(2))

			Expect(*result).To(Equal([]domain.ProjectMemberDetail{
				{ProjectMember: domain.ProjectMember{ProjectId: 1, MemberId: 10, Role: "admin", CreateTime: t},
					ProjectName: "p1", MemberName: "u10"},
				{ProjectMember: domain.ProjectMember{ProjectId: 2, MemberId: 10, Role: "admin", CreateTime: t},
					ProjectName: "p2", MemberName: "u10"},
			}))

			projectId = types.ID(2)
			memberId = types.ID(10)
			result, err = namespace.QueryProjectMemberDetails(&domain.ProjectMemberQuery{ProjectID: &projectId, MemberID: &memberId},
				testinfra.BuildSecCtx(100, account.SystemAdminPermission.ID))
			Expect(err).To(BeNil())
			Expect(len(*result)).To(Equal(1))

			Expect(*result).To(Equal([]domain.ProjectMemberDetail{
				{ProjectMember: domain.ProjectMember{ProjectId: 2, MemberId: 10, Role: "admin", CreateTime: t},
					ProjectName: "p2", MemberName: "u10"},
			}))
		})

		It("non-admin user can only query members in their joined projects", func() {
			namespace.DetailProjectMembersFunc = func(pms *[]domain.ProjectMember) (*[]domain.ProjectMemberDetail, error) {
				var result []domain.ProjectMemberDetail
				for _, pm := range *pms {
					result = append(result, domain.ProjectMemberDetail{ProjectMember: pm,
						ProjectName: "p" + pm.ProjectId.String(), MemberName: "u" + pm.MemberId.String()})
				}
				return &result, nil
			}

			db := testDatabase.DS.GormDB()
			t := time.Date(2021, 5, 20, 0, 0, 0, 0, time.Local)
			Expect(db.Save(&domain.ProjectMember{ProjectId: 1, MemberId: 10, Role: "admin", CreateTime: t}).Error).To(BeNil())
			Expect(db.Save(&domain.ProjectMember{ProjectId: 1, MemberId: 20, Role: "guest", CreateTime: t}).Error).To(BeNil())
			Expect(db.Save(&domain.ProjectMember{ProjectId: 2, MemberId: 10, Role: "admin", CreateTime: t}).Error).To(BeNil())
			Expect(db.Save(&domain.ProjectMember{ProjectId: 2, MemberId: 30, Role: "admin", CreateTime: t}).Error).To(BeNil())
			Expect(db.Save(&domain.ProjectMember{ProjectId: 3, MemberId: 40, Role: "admin", CreateTime: t}).Error).To(BeNil())

			result, err := namespace.QueryProjectMemberDetails(&domain.ProjectMemberQuery{},
				testinfra.BuildSecCtx(100, domain.ProjectRoleManager+"_1", "guest_3"))
			Expect(err).To(BeNil())
			Expect(len(*result)).To(Equal(3))

			Expect(*result).To(Equal([]domain.ProjectMemberDetail{
				{ProjectMember: domain.ProjectMember{ProjectId: 1, MemberId: 10, Role: "admin", CreateTime: t},
					ProjectName: "p1", MemberName: "u10"},
				{ProjectMember: domain.ProjectMember{ProjectId: 1, MemberId: 20, Role: "guest", CreateTime: t},
					ProjectName: "p1", MemberName: "u20"},
				{ProjectMember: domain.ProjectMember{ProjectId: 3, MemberId: 40, Role: "admin", CreateTime: t},
					ProjectName: "p3", MemberName: "u40"},
			}))

			projectId := types.ID(1)
			result, err = namespace.QueryProjectMemberDetails(&domain.ProjectMemberQuery{ProjectID: &projectId},
				testinfra.BuildSecCtx(100, domain.ProjectRoleManager+"_1", "guest_3"))
			Expect(err).To(BeNil())
			Expect(len(*result)).To(Equal(2))

			Expect(*result).To(Equal([]domain.ProjectMemberDetail{
				{ProjectMember: domain.ProjectMember{ProjectId: 1, MemberId: 10, Role: "admin", CreateTime: t},
					ProjectName: "p1", MemberName: "u10"},
				{ProjectMember: domain.ProjectMember{ProjectId: 1, MemberId: 20, Role: "guest", CreateTime: t},
					ProjectName: "p1", MemberName: "u20"},
			}))

			memberId := types.ID(10)
			result, err = namespace.QueryProjectMemberDetails(&domain.ProjectMemberQuery{MemberID: &memberId},
				testinfra.BuildSecCtx(100, domain.ProjectRoleManager+"_1", "guest_3"))
			Expect(err).To(BeNil())
			Expect(len(*result)).To(Equal(1))

			Expect(*result).To(Equal([]domain.ProjectMemberDetail{
				{ProjectMember: domain.ProjectMember{ProjectId: 1, MemberId: 10, Role: "admin", CreateTime: t},
					ProjectName: "p1", MemberName: "u10"},
			}))

			projectId = types.ID(2)
			memberId = types.ID(10)
			result, err = namespace.QueryProjectMemberDetails(&domain.ProjectMemberQuery{ProjectID: &projectId, MemberID: &memberId},
				testinfra.BuildSecCtx(100, domain.ProjectRoleManager+"_1", "guest_2"))
			Expect(err).To(BeNil())
			Expect(len(*result)).To(Equal(1))

			Expect(*result).To(Equal([]domain.ProjectMemberDetail{
				{ProjectMember: domain.ProjectMember{ProjectId: 2, MemberId: 10, Role: "admin", CreateTime: t},
					ProjectName: "p2", MemberName: "u10"},
			}))

			result, err = namespace.QueryProjectMemberDetails(&domain.ProjectMemberQuery{ProjectID: &projectId, MemberID: &memberId},
				testinfra.BuildSecCtx(100, domain.ProjectRoleManager+"_1", "guest_3"))
			Expect(err).To(BeNil())
			Expect(len(*result)).To(Equal(0))
		})
	})

	Describe("DeleteProjectMember", func() {
		It("should be able to delete project member by admin or project manager", func() {
			sec := testinfra.BuildSecCtx(types.ID(111), account.SystemAdminPermission.ID)

			db := testDatabase.DS.GormDB()
			t := time.Date(2021, 5, 20, 0, 0, 0, 0, time.Local)
			Expect(db.Save(&domain.ProjectMember{ProjectId: 1, MemberId: 10, Role: "guest", CreateTime: t}).Error).To(BeNil())
			Expect(db.Save(&domain.ProjectMember{ProjectId: 1, MemberId: 20, Role: "watcher", CreateTime: t}).Error).To(BeNil())

			q := []domain.ProjectMember{}
			Expect(testDatabase.DS.GormDB().Find(&q).Error).To(BeNil())
			Expect(q).ToNot(BeNil())
			Expect(len(q)).To(Equal(2))

			Expect(namespace.DeleteProjectMember(&domain.ProjectMemberDeletion{ProjectID: 1, MemberID: 10}, sec)).To(BeNil())
			q = []domain.ProjectMember{}
			Expect(testDatabase.DS.GormDB().Find(&q).Error).To(BeNil())
			Expect(q).ToNot(BeNil())
			Expect(len(q)).To(Equal(1))

			Expect(q[0].ProjectId).To(Equal(types.ID(1)))
			Expect(q[0].MemberId).To(Equal(types.ID(20)))

			sec = testinfra.BuildSecCtx(types.ID(111), domain.ProjectRoleManager+"_1")
			Expect(namespace.DeleteProjectMember(&domain.ProjectMemberDeletion{ProjectID: 1, MemberID: 20}, sec)).To(BeNil())
			q = []domain.ProjectMember{}
			Expect(testDatabase.DS.GormDB().Find(&q).Error).To(BeNil())
			Expect(q).ToNot(BeNil())
			Expect(len(q)).To(Equal(0))

			Expect(namespace.DeleteProjectMember(&domain.ProjectMemberDeletion{ProjectID: 1, MemberID: 20}, sec)).To(BeNil())
		})

		It("last project manager should not be able to delete", func() {
			db := testDatabase.DS.GormDB()
			t := time.Date(2021, 5, 20, 0, 0, 0, 0, time.Local)
			Expect(db.Save(&domain.ProjectMember{ProjectId: 1, MemberId: 10, Role: domain.ProjectRoleManager, CreateTime: t}).Error).To(BeNil())

			Expect(db.Save(&domain.ProjectMember{ProjectId: 1, MemberId: 11, Role: domain.ProjectRoleManager, CreateTime: t}).Error).To(BeNil())

			sec := testinfra.BuildSecCtx(types.ID(111), account.SystemAdminPermission.ID)
			Expect(namespace.DeleteProjectMember(&domain.ProjectMemberDeletion{ProjectID: 1, MemberID: 10}, sec)).To(BeNil())
			Expect(namespace.DeleteProjectMember(&domain.ProjectMemberDeletion{ProjectID: 1, MemberID: 11}, sec)).To(Equal(bizerror.ErrLastProjectManagerDelete))
		})

		It("non non-administrator nor non-project manager can create project member", func() {
			db := testDatabase.DS.GormDB()
			t := time.Date(2021, 5, 20, 0, 0, 0, 0, time.Local)
			Expect(db.Save(&domain.ProjectMember{ProjectId: 1, MemberId: 10, Role: "guest", CreateTime: t}).Error).To(BeNil())
			Expect(db.Save(&domain.ProjectMember{ProjectId: 1, MemberId: 20, Role: "watcher", CreateTime: t}).Error).To(BeNil())

			sec := testinfra.BuildSecCtx(types.ID(111), domain.ProjectRoleManager+"_2")
			Expect(namespace.DeleteProjectMember(&domain.ProjectMemberDeletion{ProjectID: 1, MemberID: 20}, sec)).To(Equal(bizerror.ErrForbidden))

			sec = testinfra.BuildSecCtx(types.ID(111), "guest_1")
			Expect(namespace.DeleteProjectMember(&domain.ProjectMemberDeletion{ProjectID: 1, MemberID: 20}, sec)).To(Equal(bizerror.ErrForbidden))
		})
	})
})
