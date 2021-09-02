package workcontribution_test

import (
	"errors"
	"flywheel/account"
	"flywheel/authority"
	"flywheel/bizerror"
	"flywheel/domain"
	"flywheel/domain/workcontribution"
	"flywheel/persistence"
	"flywheel/session"
	"flywheel/testinfra"
	"time"

	"github.com/fundwit/go-commons/types"
	"github.com/jinzhu/gorm"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("WorkContributions", func() {
	var (
		testDatabase  *testinfra.TestDatabase
		db            *gorm.DB
		grantedUser   *account.User
		ungrantedUser *account.User
		givenWork     *domain.Work
		sessionUser   *account.User
		testErr       error
	)
	BeforeEach(func() {
		testErr = errors.New("test error")
		testDatabase = testinfra.StartMysqlTestDatabase("flywheel")
		Expect(testDatabase.DS.GormDB().AutoMigrate(
			&workcontribution.WorkContributionRecord{},
			&domain.Work{}, &account.User{}).Error).To(BeNil())
		persistence.ActiveDataSourceManager = testDatabase.DS
		db = testDatabase.DS.GormDB()
		account.LoadPermFunc = func(uid types.ID) (authority.Permissions, authority.ProjectRoles) {
			return authority.Permissions{}, authority.ProjectRoles{}
		}
		// given a work and a user
		grantedUser = &account.User{ID: 10, Name: "testUser", Nickname: "Test User", Secret: "123"}
		Expect(db.Save(grantedUser).Error).To(BeNil())

		ungrantedUser = &account.User{ID: 11, Name: "test user 11", Secret: "123"}
		Expect(db.Save(ungrantedUser).Error).To(BeNil())

		givenWork = &domain.Work{ID: 20, Identifier: "TES-1", Name: "test work", ProjectID: 30, FlowID: 40}
		Expect(db.Save(givenWork).Error).To(BeNil())

		sessionUser = &account.User{ID: 999, Name: "test user 999", Secret: "123"}
		Expect(db.Save(sessionUser).Error).To(BeNil())
	})
	AfterEach(func() {
		testinfra.StopMysqlTestDatabase(testDatabase)
	})

	Describe("CheckContributorWorkPermission", func() {
		It("should return error when contributor is not found or when work is not found", func() {
			sec := &session.Context{Identity: session.Identity{ID: sessionUser.ID}, Perms: []string{domain.ProjectRoleManager + "_" + givenWork.ProjectID.String()}}

			// work not exist
			work, user, err := workcontribution.CheckContributorWorkPermission("TES-404", grantedUser.ID, sec)
			Expect(err).To(Equal(bizerror.ErrNoContent))
			Expect(work).To(BeNil())
			Expect(user).To(BeNil())

			// contributor not exist
			work, user, err = workcontribution.CheckContributorWorkPermission(givenWork.Identifier, 404, sec)
			Expect(err).To(Equal(bizerror.ErrNoContent))
			Expect(work).To(BeNil())
			Expect(user).To(BeNil())
		})

		It("should return corrent result when contributor is not session user", func() {
			// session user: neither admin nor member of work's project    => Forbidden
			// contributor : -
			work, user, err := workcontribution.CheckContributorWorkPermission(givenWork.Identifier, ungrantedUser.ID,
				&session.Context{Identity: session.Identity{ID: sessionUser.ID}, Perms: []string{}})
			Expect(err).To(Equal(bizerror.ErrForbidden))
			Expect(work).To(BeNil())
			Expect(user).To(BeNil())

			// session user: neither admin nor manager of work's project     => Forbidden
			// contributor : -
			work, user, err = workcontribution.CheckContributorWorkPermission(givenWork.Identifier, ungrantedUser.ID,
				&session.Context{Identity: session.Identity{ID: sessionUser.ID}, Perms: []string{"guest_" + givenWork.ProjectID.String()}})
			Expect(err).To(Equal(bizerror.ErrForbidden))
			Expect(work).To(BeNil())
			Expect(user).To(BeNil())

			// session user: admin                          => OK
			// contributor : not member of work's project   => NoContent
			work, user, err = workcontribution.CheckContributorWorkPermission(givenWork.Identifier, ungrantedUser.ID,
				&session.Context{Identity: session.Identity{ID: sessionUser.ID}, Perms: []string{account.SystemAdminPermission.ID}})
			Expect(err).To(Equal(bizerror.ErrNoContent))
			Expect(work).To(BeNil())
			Expect(user).To(BeNil())

			// session user: manager of work's project        => OK
			// contributor : not member of work's project   => NoContent
			work, user, err = workcontribution.CheckContributorWorkPermission(givenWork.Identifier, ungrantedUser.ID,
				&session.Context{Identity: session.Identity{ID: sessionUser.ID}, Perms: []string{domain.ProjectRoleManager + "_" + givenWork.ProjectID.String()}})
			Expect(err).To(Equal(bizerror.ErrNoContent))
			Expect(work).To(BeNil())
			Expect(user).To(BeNil())

			// session user: admin                               => OK
			// contributor : member of work's project            => OK
			account.LoadPermFunc = func(uid types.ID) (authority.Permissions, authority.ProjectRoles) {
				return authority.Permissions{"guest_" + givenWork.ProjectID.String()},
					authority.ProjectRoles{{ProjectID: givenWork.ProjectID, ProjectName: "demo project", ProjectIdentifier: "TES", Role: domain.ProjectRoleManager + ""}}
			}
			work, user, err = workcontribution.CheckContributorWorkPermission(givenWork.Identifier, grantedUser.ID,
				&session.Context{Identity: session.Identity{ID: sessionUser.ID}, Perms: []string{account.SystemAdminPermission.ID}})
			Expect(err).To(BeNil())
			Expect(*work).To(Equal(*givenWork))
			Expect(*user).To(Equal(*grantedUser))

			// session user: manager of work's project             => OK
			// contributor : member of work's project            => OK
			work, user, err = workcontribution.CheckContributorWorkPermission(givenWork.Identifier, grantedUser.ID,
				&session.Context{Identity: session.Identity{ID: sessionUser.ID}, Perms: []string{domain.ProjectRoleManager + "_" + givenWork.ProjectID.String()}})
			Expect(err).To(BeNil())
			Expect(*work).To(Equal(*givenWork))
			Expect(*user).To(Equal(*grantedUser))
		})

		It("should return corrent result when contributor is session user", func() {
			// session user(contributor): neithr admin nor member of work's project
			work, user, err := workcontribution.CheckContributorWorkPermission(givenWork.Identifier, sessionUser.ID,
				&session.Context{Identity: session.Identity{ID: sessionUser.ID}, Perms: []string{}})
			Expect(err).To(Equal(bizerror.ErrForbidden))
			Expect(work).To(BeNil())
			Expect(user).To(BeNil())

			// session user(contributor): admin
			work, user, err = workcontribution.CheckContributorWorkPermission(givenWork.Identifier, sessionUser.ID,
				&session.Context{Identity: session.Identity{ID: sessionUser.ID}, Perms: []string{account.SystemAdminPermission.ID}})
			Expect(err).To(Equal(bizerror.ErrForbidden))
			Expect(work).To(BeNil())
			Expect(user).To(BeNil())

			// session user(contributor): member of project
			work, user, err = workcontribution.CheckContributorWorkPermission(givenWork.Identifier, sessionUser.ID,
				&session.Context{
					Identity:     session.Identity{ID: sessionUser.ID},
					Perms:        []string{"guest_" + givenWork.ProjectID.String()},
					ProjectRoles: []domain.ProjectRole{{ProjectID: givenWork.ProjectID, ProjectName: "demo project", ProjectIdentifier: "TES", Role: "guest"}},
				})
			Expect(err).To(BeNil())
			Expect(*work).To(Equal(*givenWork))
			Expect(*user).To(Equal(*sessionUser))
		})
	})

	Describe("BeginWorkContribution", func() {
		BeforeEach(func() {
			workcontribution.CheckContributorWorkPermissionFunc = func(workKey string, contributorId types.ID, sec *session.Context) (*domain.Work, *account.User, error) {
				return givenWork, grantedUser, nil
			}
		})
		It("should return error when permission check failed", func() {
			workcontribution.CheckContributorWorkPermissionFunc = func(workKey string, contributorId types.ID, sec *session.Context) (*domain.Work, *account.User, error) {
				return nil, nil, testErr
			}
			id, err := workcontribution.BeginWorkContribution(
				&workcontribution.WorkContribution{WorkKey: givenWork.Identifier, ContributorId: grantedUser.ID},
				&session.Context{Identity: session.Identity{ID: sessionUser.ID}})
			Expect(id).To(BeZero())
			Expect(err).To(Equal(testErr))
		})

		It("should be able to begin new contribution", func() {
			sec := &session.Context{Identity: session.Identity{ID: sessionUser.ID}, Perms: []string{account.SystemAdminPermission.ID}}
			begin := time.Now()
			id, err := workcontribution.BeginWorkContribution(
				&workcontribution.WorkContribution{WorkKey: givenWork.Identifier, ContributorId: grantedUser.ID}, sec)
			Expect(err).To(BeNil())
			Expect(id).ToNot(BeZero())
			record := workcontribution.WorkContributionRecord{ID: id}
			Expect(db.Where(&record).First(&record).Error).To(BeNil())
			Expect(record.ID).To(Equal(id))
			Expect(record.WorkKey).To(Equal(givenWork.Identifier))
			Expect(record.ContributorId).To(Equal(grantedUser.ID))
			Expect(record.ContributorName).To(Equal(grantedUser.Nickname))
			Expect(record.WorkProjectId).To(Equal(givenWork.ProjectID))
			Expect(record.BeginTime.Time().Unix() >= begin.Round(time.Microsecond).Unix() && record.BeginTime.Time().Unix() <= time.Now().Round(time.Microsecond).Unix()).To(BeTrue())
			Expect(record.EndTime).To(BeZero())
			Expect(record.Effective).To(BeTrue())
		})

		It("should be able to contribution again when last contribution is undergoing", func() {
			sec := &session.Context{Identity: session.Identity{ID: sessionUser.ID}, Perms: []string{account.SystemAdminPermission.ID}}

			givenReocrd := workcontribution.WorkContributionRecord{
				WorkContribution: workcontribution.WorkContribution{WorkKey: "TEST-100", ContributorId: 200},
				ID:               1000, ContributorName: "user 200", BeginTime: types.TimestampOfDate(2021, 1, 1, 12, 0, 0, 0, time.Local), Effective: true,
			}
			Expect(db.Save(&givenReocrd).Error).To(BeNil())

			time.Sleep(10 * time.Millisecond)
			// begin work contribution again
			id1, err := workcontribution.BeginWorkContribution(
				&workcontribution.WorkContribution{WorkKey: givenReocrd.WorkKey, ContributorId: givenReocrd.ContributorId}, sec)
			Expect(err).To(BeNil())
			Expect(id1).To(Equal(givenReocrd.ID))

			record := workcontribution.WorkContributionRecord{ID: id1}
			Expect(db.Where(&record).First(&record).Error).To(BeNil())
			Expect(record.ID).To(Equal(givenReocrd.ID))
			Expect(record.WorkKey).To(Equal(givenReocrd.WorkKey))
			Expect(record.ContributorId).To(Equal(givenReocrd.ContributorId))
			Expect(record.ContributorName).To(Equal(grantedUser.Nickname)) // updated
			Expect(record.WorkProjectId).To(Equal(givenWork.ProjectID))    // updated
			Expect(record.BeginTime).To(Equal(givenReocrd.BeginTime))      // not change any more
			Expect(record.EndTime).To(BeZero())
			Expect(record.Effective).To(BeTrue())
		})

		It("should be able to contribution again when last contribution is finished", func() {
			sec := &session.Context{Identity: session.Identity{ID: sessionUser.ID}, Perms: []string{account.SystemAdminPermission.ID}}

			givenReocrd := workcontribution.WorkContributionRecord{
				WorkContribution: workcontribution.WorkContribution{WorkKey: "TEST-100", ContributorId: 200},
				ID:               1000, ContributorName: "user 200", BeginTime: types.TimestampOfDate(2021, 1, 1, 12, 0, 0, 0, time.Local),
				EndTime: types.TimestampOfDate(2021, 1, 2, 12, 0, 0, 0, time.Local), Effective: true,
			}
			Expect(db.Save(&givenReocrd).Error).To(BeNil())

			time.Sleep(10 * time.Millisecond)
			// begin work contribution again
			id1, err := workcontribution.BeginWorkContribution(
				&workcontribution.WorkContribution{WorkKey: givenReocrd.WorkKey, ContributorId: givenReocrd.ContributorId}, sec)
			Expect(err).To(BeNil())
			Expect(id1).To(Equal(givenReocrd.ID))

			record := workcontribution.WorkContributionRecord{ID: id1}
			Expect(db.Where(&record).First(&record).Error).To(BeNil())
			Expect(record.ID).To(Equal(givenReocrd.ID))
			Expect(record.WorkKey).To(Equal(givenReocrd.WorkKey))
			Expect(record.ContributorId).To(Equal(givenReocrd.ContributorId))
			Expect(record.ContributorName).To(Equal(grantedUser.Nickname)) // updated
			Expect(record.WorkProjectId).To(Equal(givenWork.ProjectID))    // updated
			Expect(record.BeginTime).To(Equal(givenReocrd.BeginTime))      // not change any more
			Expect(record.EndTime).To(BeZero())                            // end time is reseted
			Expect(record.Effective).To(BeTrue())
		})

		It("should be able to contribution again when last contribution is discarded", func() {
			sec := &session.Context{Identity: session.Identity{ID: sessionUser.ID}, Perms: []string{account.SystemAdminPermission.ID}}

			givenReocrd := workcontribution.WorkContributionRecord{
				WorkContribution: workcontribution.WorkContribution{WorkKey: "TEST-100", ContributorId: 200},
				ID:               1000, ContributorName: "user 200", BeginTime: types.TimestampOfDate(2021, 1, 1, 12, 0, 0, 0, time.Local),
				EndTime: types.TimestampOfDate(2021, 1, 2, 12, 0, 0, 0, time.Local), Effective: false,
			}
			Expect(db.Save(&givenReocrd).Error).To(BeNil())

			time.Sleep(10 * time.Millisecond)
			// begin work contribution again
			id1, err := workcontribution.BeginWorkContribution(
				&workcontribution.WorkContribution{WorkKey: givenReocrd.WorkKey, ContributorId: givenReocrd.ContributorId}, sec)
			Expect(err).To(BeNil())
			Expect(id1).To(Equal(givenReocrd.ID))

			record := workcontribution.WorkContributionRecord{ID: id1}
			Expect(db.Where(&record).First(&record).Error).To(BeNil())
			Expect(record.ID).To(Equal(givenReocrd.ID))
			Expect(record.WorkKey).To(Equal(givenReocrd.WorkKey))
			Expect(record.ContributorId).To(Equal(givenReocrd.ContributorId))
			Expect(record.ContributorName).To(Equal(grantedUser.Nickname)) // updated
			Expect(record.WorkProjectId).To(Equal(givenWork.ProjectID))    // updated
			Expect(record.BeginTime).To(Equal(givenReocrd.BeginTime))      // begin time not change any more
			Expect(record.EndTime).To(BeZero())                            // end time update to zero
			Expect(record.Effective).To(BeTrue())                          // effective update to true
		})
	})

	Describe("FinishWorkContribution effective", func() {
		BeforeEach(func() {
			workcontribution.CheckContributorWorkPermissionFunc = func(workKey string, contributorId types.ID, sec *session.Context) (*domain.Work, *account.User, error) {
				return givenWork, grantedUser, nil
			}
		})
		It("should return error when permission check failed", func() {
			workcontribution.CheckContributorWorkPermissionFunc = func(workKey string, contributorId types.ID, sec *session.Context) (*domain.Work, *account.User, error) {
				return nil, nil, testErr
			}
			err := workcontribution.FinishWorkContribution(&workcontribution.WorkContribuitonFinishBody{Effective: true,
				WorkContribution: workcontribution.WorkContribution{WorkKey: givenWork.Identifier, ContributorId: grantedUser.ID}},
				&session.Context{Identity: session.Identity{ID: sessionUser.ID}})
			Expect(err).To(Equal(testErr))
		})
		It("should faild to finish work contribution when work contribution is not exist", func() {
			err := workcontribution.FinishWorkContribution(&workcontribution.WorkContribuitonFinishBody{Effective: true,
				WorkContribution: workcontribution.WorkContribution{WorkKey: givenWork.Identifier, ContributorId: grantedUser.ID}},
				&session.Context{Identity: session.Identity{ID: sessionUser.ID}})
			Expect(err).To(Equal(gorm.ErrRecordNotFound))
		})

		It("should be able to finish work contribution when work contribution is undergoing", func() {
			givenReocrd := workcontribution.WorkContributionRecord{
				WorkContribution: workcontribution.WorkContribution{WorkKey: givenWork.Identifier, ContributorId: grantedUser.ID},
				ID:               1000, ContributorName: "user 200", BeginTime: types.TimestampOfDate(2021, 1, 1, 12, 0, 0, 0, time.Local),
				Effective: true,
			}
			Expect(db.Save(&givenReocrd).Error).To(BeNil())
			begin := time.Now()
			err := workcontribution.FinishWorkContribution(&workcontribution.WorkContribuitonFinishBody{Effective: true,
				WorkContribution: workcontribution.WorkContribution{WorkKey: givenWork.Identifier, ContributorId: grantedUser.ID}},
				&session.Context{Identity: session.Identity{ID: sessionUser.ID}})
			Expect(err).To(BeNil())

			record := workcontribution.WorkContributionRecord{ID: givenReocrd.ID}
			Expect(db.Where(&record).First(&record).Error).To(BeNil())
			Expect(record.ID).To(Equal(givenReocrd.ID))
			Expect(record.WorkKey).To(Equal(givenReocrd.WorkKey))
			Expect(record.ContributorId).To(Equal(givenReocrd.ContributorId))
			Expect(record.ContributorName).To(Equal(grantedUser.Nickname)) // updated
			Expect(record.WorkProjectId).To(Equal(givenWork.ProjectID))    // updated
			Expect(record.BeginTime).To(Equal(givenReocrd.BeginTime))      // begin time not change any more
			Expect(record.EndTime.Time().Unix() >= begin.Round(time.Microsecond).Unix() && record.EndTime.Time().Unix() <= time.Now().Round(time.Microsecond).Unix()).To(BeTrue())
			Expect(record.Effective).To(BeTrue()) // effective update to true
		})

		It("should be able to finish work contribution when work contribution already finished", func() {
			givenReocrd := workcontribution.WorkContributionRecord{
				WorkContribution: workcontribution.WorkContribution{WorkKey: givenWork.Identifier, ContributorId: grantedUser.ID},
				ID:               1000, ContributorName: "user 200", BeginTime: types.TimestampOfDate(2021, 1, 1, 12, 0, 0, 0, time.Local),
				EndTime: types.TimestampOfDate(2021, 1, 2, 12, 0, 0, 0, time.Local), Effective: true,
			}
			Expect(db.Save(&givenReocrd).Error).To(BeNil())
			err := workcontribution.FinishWorkContribution(&workcontribution.WorkContribuitonFinishBody{Effective: true,
				WorkContribution: workcontribution.WorkContribution{WorkKey: givenWork.Identifier, ContributorId: grantedUser.ID}},
				&session.Context{Identity: session.Identity{ID: sessionUser.ID}})
			Expect(err).To(BeNil())

			record := workcontribution.WorkContributionRecord{ID: givenReocrd.ID}
			Expect(db.Where(&record).First(&record).Error).To(BeNil())
			Expect(record.ID).To(Equal(givenReocrd.ID))
			Expect(record.WorkKey).To(Equal(givenReocrd.WorkKey))
			Expect(record.ContributorId).To(Equal(givenReocrd.ContributorId))
			Expect(record.ContributorName).To(Equal(grantedUser.Nickname)) // updated
			Expect(record.WorkProjectId).To(Equal(givenWork.ProjectID))    // updated
			Expect(record.BeginTime).To(Equal(givenReocrd.BeginTime))      // begin time not change any more
			Expect(record.EndTime).To(Equal(givenReocrd.EndTime))          // end time not change any more
			Expect(record.Effective).To(BeTrue())                          // effective not change any more
		})

		It("should be able to finish work contribution when work contribution is discarded", func() {
			givenReocrd := workcontribution.WorkContributionRecord{
				WorkContribution: workcontribution.WorkContribution{WorkKey: givenWork.Identifier, ContributorId: grantedUser.ID},
				ID:               1000, ContributorName: "user 200", BeginTime: types.TimestampOfDate(2021, 1, 1, 12, 0, 0, 0, time.Local),
				EndTime: types.TimestampOfDate(2021, 1, 2, 12, 0, 0, 0, time.Local), Effective: false,
			}
			Expect(db.Save(&givenReocrd).Error).To(BeNil())
			err := workcontribution.FinishWorkContribution(&workcontribution.WorkContribuitonFinishBody{Effective: true,
				WorkContribution: workcontribution.WorkContribution{WorkKey: givenWork.Identifier, ContributorId: grantedUser.ID}},
				&session.Context{Identity: session.Identity{ID: sessionUser.ID}})
			Expect(err).To(BeNil())

			record := workcontribution.WorkContributionRecord{ID: givenReocrd.ID}
			Expect(db.Where(&record).First(&record).Error).To(BeNil())
			Expect(record.ID).To(Equal(givenReocrd.ID))
			Expect(record.WorkKey).To(Equal(givenReocrd.WorkKey))
			Expect(record.ContributorId).To(Equal(givenReocrd.ContributorId))
			Expect(record.ContributorName).To(Equal(grantedUser.Nickname)) // updated
			Expect(record.WorkProjectId).To(Equal(givenWork.ProjectID))    // updated
			Expect(record.BeginTime).To(Equal(givenReocrd.BeginTime))      // begin time not change any more
			Expect(record.EndTime).To(Equal(givenReocrd.EndTime))          // end time not change any more
			Expect(record.Effective).To(BeTrue())                          // effective update to true
		})
	})

	Describe("FinishWorkContribution effective = false", func() {
		BeforeEach(func() {
			workcontribution.CheckContributorWorkPermissionFunc = func(workKey string, contributorId types.ID, sec *session.Context) (*domain.Work, *account.User, error) {
				return givenWork, grantedUser, nil
			}
		})
		It("should return error when permission check failed", func() {
			workcontribution.CheckContributorWorkPermissionFunc = func(workKey string, contributorId types.ID, sec *session.Context) (*domain.Work, *account.User, error) {
				return nil, nil, testErr
			}
			err := workcontribution.FinishWorkContribution(&workcontribution.WorkContribuitonFinishBody{
				WorkContribution: workcontribution.WorkContribution{WorkKey: givenWork.Identifier, ContributorId: grantedUser.ID}},
				&session.Context{Identity: session.Identity{ID: sessionUser.ID}})
			Expect(err).To(Equal(testErr))
		})
		It("should faild to discard work contribution when work contribution is not exist", func() {
			err := workcontribution.FinishWorkContribution(&workcontribution.WorkContribuitonFinishBody{
				WorkContribution: workcontribution.WorkContribution{WorkKey: givenWork.Identifier, ContributorId: grantedUser.ID}},
				&session.Context{Identity: session.Identity{ID: sessionUser.ID}})
			Expect(err).To(Equal(gorm.ErrRecordNotFound))
		})

		It("should be able to discard work contribution when work contribution is undergoing", func() {
			givenReocrd := workcontribution.WorkContributionRecord{
				WorkContribution: workcontribution.WorkContribution{WorkKey: givenWork.Identifier, ContributorId: grantedUser.ID},
				ID:               1000, ContributorName: "user 200", BeginTime: types.TimestampOfDate(2021, 1, 1, 12, 0, 0, 0, time.Local),
				Effective: true,
			}
			Expect(db.Save(&givenReocrd).Error).To(BeNil())
			begin := time.Now()
			err := workcontribution.FinishWorkContribution(&workcontribution.WorkContribuitonFinishBody{
				WorkContribution: workcontribution.WorkContribution{WorkKey: givenWork.Identifier, ContributorId: grantedUser.ID}},
				&session.Context{Identity: session.Identity{ID: sessionUser.ID}})
			Expect(err).To(BeNil())

			record := workcontribution.WorkContributionRecord{ID: givenReocrd.ID}
			Expect(db.Where(&record).First(&record).Error).To(BeNil())
			Expect(record.ID).To(Equal(givenReocrd.ID))
			Expect(record.WorkKey).To(Equal(givenReocrd.WorkKey))
			Expect(record.ContributorId).To(Equal(givenReocrd.ContributorId))
			Expect(record.ContributorName).To(Equal(grantedUser.Nickname)) // updated
			Expect(record.WorkProjectId).To(Equal(givenWork.ProjectID))    // updated
			Expect(record.BeginTime).To(Equal(givenReocrd.BeginTime))      // begin time not change any more
			Expect(record.EndTime.Time().Unix() >= begin.Round(time.Microsecond).Unix() && record.EndTime.Time().Unix() <= time.Now().Round(time.Microsecond).Unix()).To(BeTrue())
			Expect(record.Effective).To(BeFalse()) // effective update to false
		})

		It("should be able to discard work contribution when work contribution already finished", func() {
			givenReocrd := workcontribution.WorkContributionRecord{
				WorkContribution: workcontribution.WorkContribution{WorkKey: givenWork.Identifier, ContributorId: grantedUser.ID},
				ID:               1000, ContributorName: "user 200", BeginTime: types.TimestampOfDate(2021, 1, 1, 12, 0, 0, 0, time.Local),
				EndTime: types.TimestampOfDate(2021, 1, 2, 12, 0, 0, 0, time.Local), Effective: true,
			}
			Expect(db.Save(&givenReocrd).Error).To(BeNil())
			err := workcontribution.FinishWorkContribution(&workcontribution.WorkContribuitonFinishBody{
				WorkContribution: workcontribution.WorkContribution{WorkKey: givenWork.Identifier, ContributorId: grantedUser.ID}},
				&session.Context{Identity: session.Identity{ID: sessionUser.ID}})
			Expect(err).To(BeNil())

			record := workcontribution.WorkContributionRecord{ID: givenReocrd.ID}
			Expect(db.Where(&record).First(&record).Error).To(BeNil())
			Expect(record.ID).To(Equal(givenReocrd.ID))
			Expect(record.WorkKey).To(Equal(givenReocrd.WorkKey))
			Expect(record.ContributorId).To(Equal(givenReocrd.ContributorId))
			Expect(record.ContributorName).To(Equal(grantedUser.Nickname)) // updated
			Expect(record.WorkProjectId).To(Equal(givenWork.ProjectID))    // updated
			Expect(record.BeginTime).To(Equal(givenReocrd.BeginTime))      // begin time not change any more
			Expect(record.EndTime).To(Equal(givenReocrd.EndTime))          // end time not change any more
			Expect(record.Effective).To(BeFalse())                         // effective udpate to false
		})

		It("should be able to discard work contribution when work contribution is discarded", func() {
			givenReocrd := workcontribution.WorkContributionRecord{
				WorkContribution: workcontribution.WorkContribution{WorkKey: givenWork.Identifier, ContributorId: grantedUser.ID},
				ID:               1000, ContributorName: "user 200", BeginTime: types.TimestampOfDate(2021, 1, 1, 12, 0, 0, 0, time.Local),
				EndTime: types.TimestampOfDate(2021, 1, 2, 12, 0, 0, 0, time.Local), Effective: false,
			}
			Expect(db.Save(&givenReocrd).Error).To(BeNil())
			err := workcontribution.FinishWorkContribution(&workcontribution.WorkContribuitonFinishBody{
				WorkContribution: workcontribution.WorkContribution{WorkKey: givenWork.Identifier, ContributorId: grantedUser.ID}},
				&session.Context{Identity: session.Identity{ID: sessionUser.ID}})
			Expect(err).To(BeNil())

			record := workcontribution.WorkContributionRecord{ID: givenReocrd.ID}
			Expect(db.Where(&record).First(&record).Error).To(BeNil())
			Expect(record.ID).To(Equal(givenReocrd.ID))
			Expect(record.WorkKey).To(Equal(givenReocrd.WorkKey))
			Expect(record.ContributorId).To(Equal(givenReocrd.ContributorId))
			Expect(record.ContributorName).To(Equal(grantedUser.Nickname)) // updated
			Expect(record.WorkProjectId).To(Equal(givenWork.ProjectID))    // updated
			Expect(record.BeginTime).To(Equal(givenReocrd.BeginTime))      // begin time not change any more
			Expect(record.EndTime).To(Equal(givenReocrd.EndTime))          // end time not change any more
			Expect(record.Effective).To(BeFalse())                         // effective not change any more
		})
	})

	Describe("QueryWorkContributions", func() {
		It("should get empty result when no work keys", func() {
			result, err := workcontribution.QueryWorkContributions(
				workcontribution.WorkContributionsQuery{}, &session.Context{Identity: session.Identity{ID: sessionUser.ID}})
			Expect(err).To(BeNil())
			Expect(result).ToNot(BeNil())
			Expect(len(*result)).To(BeZero())

			result, err = workcontribution.QueryWorkContributions(
				workcontribution.WorkContributionsQuery{WorkKeys: []string{}}, &session.Context{Identity: session.Identity{ID: sessionUser.ID}})
			Expect(err).To(BeNil())
			Expect(result).ToNot(BeNil())
			Expect(len(*result)).To(BeZero())
		})

		It("should get empty result when work keys not exists", func() {
			result, err := workcontribution.QueryWorkContributions(
				workcontribution.WorkContributionsQuery{WorkKeys: []string{"TEST_404"}}, &session.Context{Identity: session.Identity{ID: sessionUser.ID}})
			Expect(err).To(BeNil())
			Expect(result).ToNot(BeNil())
			Expect(len(*result)).To(BeZero())
		})

		It("should be able to get correct result", func() {
			givenReocrd1 := workcontribution.WorkContributionRecord{
				WorkContribution: workcontribution.WorkContribution{WorkKey: givenWork.Identifier, ContributorId: grantedUser.ID},
				ID:               1000, ContributorName: "user 200", BeginTime: types.TimestampOfDate(2021, 1, 1, 12, 0, 0, 0, time.Local),
				EndTime: types.TimestampOfDate(2021, 1, 2, 12, 0, 0, 0, time.Local), Effective: true,
				WorkProjectId: 300,
			}
			Expect(db.Save(&givenReocrd1).Error).To(BeNil())
			givenReocrd2 := workcontribution.WorkContributionRecord{
				WorkContribution: workcontribution.WorkContribution{WorkKey: "TEST_404", ContributorId: grantedUser.ID},
				ID:               1001, ContributorName: "user 200", BeginTime: types.TimestampOfDate(2021, 1, 1, 13, 0, 0, 0, time.Local),
				EndTime: types.TimestampOfDate(2021, 1, 2, 13, 0, 0, 0, time.Local), Effective: true,
				WorkProjectId: 404,
			}
			Expect(db.Save(&givenReocrd2).Error).To(BeNil())

			// should be able to get result of all project for system admin
			result, err := workcontribution.QueryWorkContributions(
				workcontribution.WorkContributionsQuery{WorkKeys: []string{givenReocrd1.WorkKey, givenReocrd2.WorkKey}},
				&session.Context{Identity: session.Identity{ID: sessionUser.ID}, Perms: []string{account.SystemAdminPermission.ID}})
			Expect(err).To(BeNil())
			Expect(len(*result)).To(Equal(2))
			Expect((*result)[0]).To(Equal(givenReocrd1))
			Expect((*result)[1]).To(Equal(givenReocrd2))

			result, err = workcontribution.QueryWorkContributions(
				workcontribution.WorkContributionsQuery{WorkKeys: []string{givenReocrd2.WorkKey}},
				&session.Context{Identity: session.Identity{ID: sessionUser.ID}, Perms: []string{account.SystemAdminPermission.ID}})
			Expect(err).To(BeNil())
			Expect(len(*result)).To(Equal(1))
			Expect((*result)[0]).To(Equal(givenReocrd2))

			// should be able to get result from visable projects for non system admin
			// session.Context.VisibleProjects()
			result, err = workcontribution.QueryWorkContributions(
				workcontribution.WorkContributionsQuery{WorkKeys: []string{givenReocrd1.WorkKey, givenReocrd2.WorkKey}},
				&session.Context{Identity: session.Identity{ID: sessionUser.ID}, Perms: []string{"guest_404"}})
			Expect(err).To(BeNil())
			Expect(len(*result)).To(Equal(1))
			Expect((*result)[0]).To(Equal(givenReocrd2))
		})
	})
})
