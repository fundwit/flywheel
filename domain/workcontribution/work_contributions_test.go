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
	"testing"
	"time"

	"github.com/fundwit/go-commons/types"
	"github.com/jinzhu/gorm"
	. "github.com/onsi/gomega"
)

func workContributionTestSetup(t *testing.T, testDatabase **testinfra.TestDatabase) (
	*account.User, *account.User, *domain.Work, *account.User) {

	db := testinfra.StartMysqlTestDatabase("flywheel")
	*testDatabase = db
	// migration
	Expect(db.DS.GormDB().AutoMigrate(
		&workcontribution.WorkContributionRecord{},
		&domain.Work{}, &account.User{}).Error).To(BeNil())

	persistence.ActiveDataSourceManager = db.DS
	account.LoadPermFunc = func(uid types.ID) (authority.Permissions, authority.ProjectRoles) {
		return authority.Permissions{}, authority.ProjectRoles{}
	}

	// given a work and a user
	grantedUser := &account.User{ID: 10, Name: "testUser", Nickname: "Test User", Secret: "123"}
	Expect(db.DS.GormDB().Save(grantedUser).Error).To(BeNil())

	ungrantedUser := &account.User{ID: 11, Name: "test user 11", Secret: "123"}
	Expect(db.DS.GormDB().Save(ungrantedUser).Error).To(BeNil())

	givenWork := &domain.Work{ID: 20, Identifier: "TES-1", Name: "test work", ProjectID: 30, FlowID: 40}
	Expect(db.DS.GormDB().Save(givenWork).Error).To(BeNil())

	sessionUser := &account.User{ID: 999, Name: "test user 999", Secret: "123"}
	Expect(db.DS.GormDB().Save(sessionUser).Error).To(BeNil())

	return grantedUser, ungrantedUser, givenWork, sessionUser
}

func workContributionTestTeardown(t *testing.T, testDatabase *testinfra.TestDatabase) {
	if testDatabase != nil {
		testinfra.StopMysqlTestDatabase(testDatabase)
	}
}

func TestCheckContributorWorkPermission(t *testing.T) {
	RegisterTestingT(t)
	var testDatabase *testinfra.TestDatabase

	t.Run("should return error when contributor is not found or when work is not found", func(t *testing.T) {
		defer workContributionTestTeardown(t, testDatabase)
		grantedUser, _, givenWork, sessionUser := workContributionTestSetup(t, &testDatabase)

		sec := &session.Session{Identity: session.Identity{ID: sessionUser.ID}, Perms: []string{domain.ProjectRoleManager + "_" + givenWork.ProjectID.String()}}

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

	t.Run("should return corrent result when contributor is not session user", func(t *testing.T) {
		defer workContributionTestTeardown(t, testDatabase)
		grantedUser, ungrantedUser, givenWork, sessionUser := workContributionTestSetup(t, &testDatabase)

		// session user: neither admin nor member of work's project    => Forbidden
		// contributor : -
		work, user, err := workcontribution.CheckContributorWorkPermission(givenWork.Identifier, ungrantedUser.ID,
			&session.Session{Identity: session.Identity{ID: sessionUser.ID}, Perms: []string{}})
		Expect(err).To(Equal(bizerror.ErrForbidden))
		Expect(work).To(BeNil())
		Expect(user).To(BeNil())

		// session user: neither admin nor manager of work's project     => Forbidden
		// contributor : -
		work, user, err = workcontribution.CheckContributorWorkPermission(givenWork.Identifier, ungrantedUser.ID,
			&session.Session{Identity: session.Identity{ID: sessionUser.ID}, Perms: []string{"guest_" + givenWork.ProjectID.String()}})
		Expect(err).To(Equal(bizerror.ErrForbidden))
		Expect(work).To(BeNil())
		Expect(user).To(BeNil())

		// session user: admin                          => OK
		// contributor : not member of work's project   => NoContent
		work, user, err = workcontribution.CheckContributorWorkPermission(givenWork.Identifier, ungrantedUser.ID,
			&session.Session{Identity: session.Identity{ID: sessionUser.ID}, Perms: []string{account.SystemAdminPermission.ID}})
		Expect(err).To(Equal(bizerror.ErrNoContent))
		Expect(work).To(BeNil())
		Expect(user).To(BeNil())

		// session user: manager of work's project        => OK
		// contributor : not member of work's project   => NoContent
		work, user, err = workcontribution.CheckContributorWorkPermission(givenWork.Identifier, ungrantedUser.ID,
			&session.Session{Identity: session.Identity{ID: sessionUser.ID}, Perms: []string{domain.ProjectRoleManager + "_" + givenWork.ProjectID.String()}})
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
			&session.Session{Identity: session.Identity{ID: sessionUser.ID}, Perms: []string{account.SystemAdminPermission.ID}})
		Expect(err).To(BeNil())
		Expect(*work).To(Equal(*givenWork))
		Expect(*user).To(Equal(*grantedUser))

		// session user: manager of work's project             => OK
		// contributor : member of work's project            => OK
		work, user, err = workcontribution.CheckContributorWorkPermission(givenWork.Identifier, grantedUser.ID,
			&session.Session{Identity: session.Identity{ID: sessionUser.ID}, Perms: []string{domain.ProjectRoleManager + "_" + givenWork.ProjectID.String()}})
		Expect(err).To(BeNil())
		Expect(*work).To(Equal(*givenWork))
		Expect(*user).To(Equal(*grantedUser))
	})

	t.Run("should return corrent result when contributor is session user", func(t *testing.T) {
		defer workContributionTestTeardown(t, testDatabase)
		_, _, givenWork, sessionUser := workContributionTestSetup(t, &testDatabase)

		// session user(contributor): neithr admin nor member of work's project
		work, user, err := workcontribution.CheckContributorWorkPermission(givenWork.Identifier, sessionUser.ID,
			&session.Session{Identity: session.Identity{ID: sessionUser.ID}, Perms: []string{}})
		Expect(err).To(Equal(bizerror.ErrForbidden))
		Expect(work).To(BeNil())
		Expect(user).To(BeNil())

		// session user(contributor): admin
		work, user, err = workcontribution.CheckContributorWorkPermission(givenWork.Identifier, sessionUser.ID,
			&session.Session{Identity: session.Identity{ID: sessionUser.ID}, Perms: []string{account.SystemAdminPermission.ID}})
		Expect(err).To(Equal(bizerror.ErrForbidden))
		Expect(work).To(BeNil())
		Expect(user).To(BeNil())

		// session user(contributor): member of project
		work, user, err = workcontribution.CheckContributorWorkPermission(givenWork.Identifier, sessionUser.ID,
			&session.Session{
				Identity:     session.Identity{ID: sessionUser.ID},
				Perms:        []string{"guest_" + givenWork.ProjectID.String()},
				ProjectRoles: []domain.ProjectRole{{ProjectID: givenWork.ProjectID, ProjectName: "demo project", ProjectIdentifier: "TES", Role: "guest"}},
			})
		Expect(err).To(BeNil())
		Expect(*work).To(Equal(*givenWork))
		Expect(*user).To(Equal(*sessionUser))
	})
}

func TestBeginWorkContribution(t *testing.T) {
	RegisterTestingT(t)
	var testDatabase *testinfra.TestDatabase

	t.Run("should return error when permission check failed", func(t *testing.T) {
		defer workContributionTestTeardown(t, testDatabase)
		grantedUser, _, givenWork, sessionUser := workContributionTestSetup(t, &testDatabase)
		testErr := errors.New("test error")

		workcontribution.CheckContributorWorkPermissionFunc = func(workKey string, contributorId types.ID, sec *session.Session) (*domain.Work, *account.User, error) {
			return nil, nil, testErr
		}
		id, err := workcontribution.BeginWorkContribution(
			&workcontribution.WorkContribution{WorkKey: givenWork.Identifier, ContributorId: grantedUser.ID},
			&session.Session{Identity: session.Identity{ID: sessionUser.ID}})
		Expect(id).To(BeZero())
		Expect(err).To(Equal(testErr))
	})

	t.Run("should create new contribution for every check item, but not create new contribution for which check item is existed", func(t *testing.T) {
		defer workContributionTestTeardown(t, testDatabase)
		grantedUser, _, givenWork, sessionUser := workContributionTestSetup(t, &testDatabase)

		workcontribution.CheckContributorWorkPermissionFunc = func(workKey string, contributorId types.ID, sec *session.Session) (*domain.Work, *account.User, error) {
			return givenWork, grantedUser, nil
		}

		sec := &session.Session{Identity: session.Identity{ID: sessionUser.ID}, Perms: []string{account.SystemAdminPermission.ID}}

		// prepare contribution of work
		givenRecord1 := workcontribution.WorkContributionRecord{
			WorkContribution: workcontribution.WorkContribution{WorkKey: "TEST-100", ContributorId: 200},
			ID:               1000, ContributorName: "user 200", BeginTime: types.TimestampOfDate(2021, 1, 1, 12, 0, 0, 0, time.Local),
			EndTime: types.TimestampOfDate(2021, 1, 2, 12, 0, 0, 0, time.Local), Effective: false,
		}
		Expect(testDatabase.DS.GormDB().Save(&givenRecord1).Error).To(BeNil())
		time.Sleep(10 * time.Millisecond)

		// begin new contribution for check item 1
		id1, err := workcontribution.BeginWorkContribution(
			&workcontribution.WorkContribution{WorkKey: givenRecord1.WorkKey, CheckitemId: 1, ContributorId: givenRecord1.ContributorId}, sec)
		Expect(err).To(BeNil())
		Expect(id1).ToNot(Equal(givenRecord1.ID))

		// begin new contribution for check item 2
		id2, err := workcontribution.BeginWorkContribution(
			&workcontribution.WorkContribution{WorkKey: givenRecord1.WorkKey, CheckitemId: 2, ContributorId: givenRecord1.ContributorId}, sec)
		Expect(err).To(BeNil())
		Expect(id2).ToNot(Equal(givenRecord1.ID))
		Expect(id2).ToNot(Equal(id1))

		// begin contribution for exist check item 2 again
		id2a, err := workcontribution.BeginWorkContribution(
			&workcontribution.WorkContribution{WorkKey: givenRecord1.WorkKey, CheckitemId: 2, ContributorId: givenRecord1.ContributorId}, sec)
		Expect(err).To(BeNil())
		Expect(id2a).To(Equal(id2))

		// others begin new contribution for work
		begin := time.Now()
		id, err := workcontribution.BeginWorkContribution(
			&workcontribution.WorkContribution{WorkKey: givenWork.Identifier, ContributorId: 300}, sec)
		Expect(err).To(BeNil())
		Expect(id).ToNot(BeZero())
		Expect(id).ToNot(Equal(givenRecord1))
		record := workcontribution.WorkContributionRecord{ID: id}
		Expect(testDatabase.DS.GormDB().Where(&record).First(&record).Error).To(BeNil())
		Expect(record.ID).To(Equal(id))
		Expect(record.WorkKey).To(Equal(givenWork.Identifier))
		Expect(record.ContributorId).To(Equal(types.ID(300)))
		Expect(record.ContributorName).To(Equal(grantedUser.Nickname))
		Expect(record.WorkProjectId).To(Equal(givenWork.ProjectID))
		Expect(record.BeginTime.Time().Unix() >= begin.Round(time.Microsecond).Unix() && record.BeginTime.Time().Unix() <= time.Now().Round(time.Microsecond).Unix()).To(BeTrue())
		Expect(record.EndTime).To(BeZero())
		Expect(record.Effective).To(BeTrue())

		// others begin contribution for check item 2
		id2b, err := workcontribution.BeginWorkContribution(
			&workcontribution.WorkContribution{WorkKey: givenRecord1.WorkKey, CheckitemId: 2, ContributorId: 300}, sec)
		Expect(err).To(BeNil())
		Expect(id2b).ToNot(Equal(id2a))
		Expect(id2b).ToNot(Equal(givenRecord1.ID))
	})

	t.Run("should be able to contribution again when last contribution is undergoing", func(t *testing.T) {
		defer workContributionTestTeardown(t, testDatabase)
		grantedUser, _, givenWork, sessionUser := workContributionTestSetup(t, &testDatabase)

		workcontribution.CheckContributorWorkPermissionFunc = func(workKey string, contributorId types.ID, sec *session.Session) (*domain.Work, *account.User, error) {
			return givenWork, grantedUser, nil
		}

		sec := &session.Session{Identity: session.Identity{ID: sessionUser.ID}, Perms: []string{account.SystemAdminPermission.ID}}

		givenRecord2 := workcontribution.WorkContributionRecord{
			WorkContribution: workcontribution.WorkContribution{WorkKey: "TEST-100", CheckitemId: 2, ContributorId: 200},
			ID:               1002, ContributorName: "user 200", BeginTime: types.TimestampOfDate(2021, 1, 1, 12, 0, 0, 0, time.Local), Effective: true,
		}
		db := testDatabase.DS.GormDB()
		Expect(db.Save(&givenRecord2).Error).To(BeNil())
		givenRecord1 := workcontribution.WorkContributionRecord{
			WorkContribution: workcontribution.WorkContribution{WorkKey: "TEST-100", ContributorId: 200},
			ID:               1000, ContributorName: "user 200", BeginTime: types.TimestampOfDate(2021, 1, 1, 12, 0, 0, 0, time.Local), Effective: true,
		}
		Expect(db.Save(&givenRecord1).Error).To(BeNil())

		time.Sleep(10 * time.Millisecond)
		// begin work contribution again
		id1, err := workcontribution.BeginWorkContribution(
			&workcontribution.WorkContribution{WorkKey: givenRecord1.WorkKey, ContributorId: givenRecord1.ContributorId}, sec)
		Expect(err).To(BeNil())
		Expect(id1).To(Equal(givenRecord1.ID))

		record := workcontribution.WorkContributionRecord{ID: id1}
		Expect(db.Where(&record).First(&record).Error).To(BeNil())
		Expect(record.ID).To(Equal(givenRecord1.ID))
		Expect(record.WorkKey).To(Equal(givenRecord1.WorkKey))
		Expect(record.ContributorId).To(Equal(givenRecord1.ContributorId))
		Expect(record.ContributorName).To(Equal(grantedUser.Nickname)) // updated
		Expect(record.WorkProjectId).To(Equal(givenWork.ProjectID))    // updated
		Expect(record.BeginTime).To(Equal(givenRecord1.BeginTime))     // not change any more
		Expect(record.EndTime).To(BeZero())                            // end time not change
		Expect(record.Effective).To(BeTrue())

		record = workcontribution.WorkContributionRecord{ID: givenRecord2.ID}
		Expect(db.Where(&record).First(&record).Error).To(BeNil())
		Expect(record.EndTime).To(Equal(givenRecord2.EndTime)) // end time not change
		Expect(record.EndTime).To(BeZero())
	})

	t.Run("should be able to contribution again when last contribution is finished", func(t *testing.T) {
		defer workContributionTestTeardown(t, testDatabase)
		grantedUser, _, givenWork, sessionUser := workContributionTestSetup(t, &testDatabase)

		workcontribution.CheckContributorWorkPermissionFunc = func(workKey string, contributorId types.ID, sec *session.Session) (*domain.Work, *account.User, error) {
			return givenWork, grantedUser, nil
		}

		db := testDatabase.DS.GormDB()
		sec := &session.Session{Identity: session.Identity{ID: sessionUser.ID}, Perms: []string{account.SystemAdminPermission.ID}}

		givenRecord2 := workcontribution.WorkContributionRecord{
			WorkContribution: workcontribution.WorkContribution{WorkKey: "TEST-100", CheckitemId: 2, ContributorId: 200},
			ID:               1002, ContributorName: "user 200", BeginTime: types.TimestampOfDate(2021, 1, 1, 12, 0, 0, 0, time.Local),
			EndTime: types.TimestampOfDate(2021, 1, 2, 12, 0, 0, 0, time.Local), Effective: false,
		}
		Expect(db.Save(&givenRecord2).Error).To(BeNil())
		givenRecord1 := workcontribution.WorkContributionRecord{
			WorkContribution: workcontribution.WorkContribution{WorkKey: "TEST-100", ContributorId: 200},
			ID:               1000, ContributorName: "user 200", BeginTime: types.TimestampOfDate(2021, 1, 1, 12, 0, 0, 0, time.Local),
			EndTime: types.TimestampOfDate(2021, 1, 2, 12, 0, 0, 0, time.Local), Effective: true,
		}
		Expect(db.Save(&givenRecord1).Error).To(BeNil())

		time.Sleep(10 * time.Millisecond)
		// begin work contribution again
		id2, err := workcontribution.BeginWorkContribution(
			&workcontribution.WorkContribution{WorkKey: givenRecord2.WorkKey, CheckitemId: 2, ContributorId: givenRecord2.ContributorId}, sec)
		Expect(err).To(BeNil())
		Expect(id2).To(Equal(givenRecord2.ID))

		record := workcontribution.WorkContributionRecord{ID: id2}
		Expect(db.Where(&record).First(&record).Error).To(BeNil())
		Expect(record.ID).To(Equal(givenRecord2.ID))
		Expect(record.WorkKey).To(Equal(givenRecord2.WorkKey))
		Expect(record.ContributorId).To(Equal(givenRecord2.ContributorId))
		Expect(record.ContributorName).To(Equal(grantedUser.Nickname)) // updated
		Expect(record.WorkProjectId).To(Equal(givenWork.ProjectID))    // updated
		Expect(record.BeginTime).To(Equal(givenRecord2.BeginTime))     // not change any more
		Expect(record.EndTime).To(BeZero())                            // end time is reset
		Expect(record.Effective).To(BeTrue())

		record = workcontribution.WorkContributionRecord{ID: givenRecord1.ID}
		Expect(db.Where(&record).First(&record).Error).To(BeNil())
		Expect(record.EndTime).To(Equal(givenRecord1.EndTime)) // end time is not reset
		Expect(record.EndTime).ToNot(BeNil())
	})

	t.Run("should be able to contribution again when last contribution is discarded", func(t *testing.T) {
		defer workContributionTestTeardown(t, testDatabase)
		grantedUser, _, givenWork, sessionUser := workContributionTestSetup(t, &testDatabase)

		workcontribution.CheckContributorWorkPermissionFunc = func(workKey string, contributorId types.ID, sec *session.Session) (*domain.Work, *account.User, error) {
			return givenWork, grantedUser, nil
		}

		db := testDatabase.DS.GormDB()
		sec := &session.Session{Identity: session.Identity{ID: sessionUser.ID}, Perms: []string{account.SystemAdminPermission.ID}}

		givenRecord2 := workcontribution.WorkContributionRecord{
			WorkContribution: workcontribution.WorkContribution{WorkKey: "TEST-100", CheckitemId: 2, ContributorId: 200},
			ID:               1002, ContributorName: "user 200", BeginTime: types.TimestampOfDate(2021, 1, 1, 12, 0, 0, 0, time.Local),
			EndTime: types.TimestampOfDate(2021, 1, 2, 12, 0, 0, 0, time.Local), Effective: false,
		}
		Expect(db.Save(&givenRecord2).Error).To(BeNil())
		givenRecord1 := workcontribution.WorkContributionRecord{
			WorkContribution: workcontribution.WorkContribution{WorkKey: "TEST-100", ContributorId: 200},
			ID:               1000, ContributorName: "user 200", BeginTime: types.TimestampOfDate(2021, 1, 1, 12, 0, 0, 0, time.Local),
			EndTime: types.TimestampOfDate(2021, 1, 2, 12, 0, 0, 0, time.Local), Effective: false,
		}
		Expect(db.Save(&givenRecord1).Error).To(BeNil())

		time.Sleep(10 * time.Millisecond)

		// begin work contribution again
		id1, err := workcontribution.BeginWorkContribution(
			&workcontribution.WorkContribution{WorkKey: givenRecord1.WorkKey, ContributorId: givenRecord1.ContributorId}, sec)
		Expect(err).To(BeNil())
		Expect(id1).To(Equal(givenRecord1.ID))

		record := workcontribution.WorkContributionRecord{ID: id1}
		Expect(db.Where(&record).First(&record).Error).To(BeNil())
		Expect(record.ID).To(Equal(givenRecord1.ID))
		Expect(record.WorkKey).To(Equal(givenRecord1.WorkKey))
		Expect(record.ContributorId).To(Equal(givenRecord1.ContributorId))
		Expect(record.ContributorName).To(Equal(grantedUser.Nickname)) // updated
		Expect(record.WorkProjectId).To(Equal(givenWork.ProjectID))    // updated
		Expect(record.BeginTime).To(Equal(givenRecord1.BeginTime))     // begin time not change any more
		Expect(record.EndTime).To(BeZero())                            // end time update to zero
		Expect(record.Effective).To(BeTrue())                          // effective update to true

		record = workcontribution.WorkContributionRecord{ID: givenRecord2.ID}
		Expect(db.Where(&record).First(&record).Error).To(BeNil())
		Expect(record.EndTime).ToNot(BeZero()) // end time not change
		Expect(record.EndTime).To(Equal(givenRecord2.EndTime))
		Expect(record.Effective).To(BeFalse()) // effective not changed
	})
}

func TestFinishWorkContributionEffective(t *testing.T) {
	RegisterTestingT(t)
	var testDatabase *testinfra.TestDatabase

	t.Run("should return error when permission check failed", func(t *testing.T) {
		defer workContributionTestTeardown(t, testDatabase)
		grantedUser, _, givenWork, sessionUser := workContributionTestSetup(t, &testDatabase)
		testErr := errors.New("test error")

		workcontribution.CheckContributorWorkPermissionFunc = func(workKey string, contributorId types.ID, sec *session.Session) (*domain.Work, *account.User, error) {
			return nil, nil, testErr
		}
		err := workcontribution.FinishWorkContribution(&workcontribution.WorkContributionFinishBody{Effective: true,
			WorkContribution: workcontribution.WorkContribution{WorkKey: givenWork.Identifier, ContributorId: grantedUser.ID}},
			&session.Session{Identity: session.Identity{ID: sessionUser.ID}})
		Expect(err).To(Equal(testErr))
	})

	t.Run("should failed to finish work contribution when work contribution is not exist", func(t *testing.T) {
		defer workContributionTestTeardown(t, testDatabase)
		grantedUser, _, givenWork, sessionUser := workContributionTestSetup(t, &testDatabase)

		workcontribution.CheckContributorWorkPermissionFunc = func(workKey string, contributorId types.ID, sec *session.Session) (*domain.Work, *account.User, error) {
			return givenWork, grantedUser, nil
		}
		db := testDatabase.DS.GormDB()

		// case1
		givenRecord1 := workcontribution.WorkContributionRecord{
			WorkContribution: workcontribution.WorkContribution{WorkKey: givenWork.Identifier, CheckitemId: 2, ContributorId: grantedUser.ID},
			ID:               1000, ContributorName: "user 200", BeginTime: types.TimestampOfDate(2021, 1, 1, 12, 0, 0, 0, time.Local),
			Effective: true,
		}
		Expect(db.Save(&givenRecord1).Error).To(BeNil())

		err := workcontribution.FinishWorkContribution(&workcontribution.WorkContributionFinishBody{Effective: true,
			WorkContribution: workcontribution.WorkContribution{WorkKey: givenWork.Identifier, ContributorId: grantedUser.ID}},
			&session.Session{Identity: session.Identity{ID: sessionUser.ID}})
		Expect(err).To(Equal(gorm.ErrRecordNotFound))

		// case2
		givenRecord2 := workcontribution.WorkContributionRecord{
			WorkContribution: workcontribution.WorkContribution{WorkKey: givenWork.Identifier, ContributorId: grantedUser.ID},
			ID:               1002, ContributorName: "user 200", BeginTime: types.TimestampOfDate(2021, 1, 1, 12, 0, 0, 0, time.Local),
			Effective: true,
		}
		Expect(db.Save(&givenRecord2).Error).To(BeNil())

		err = workcontribution.FinishWorkContribution(&workcontribution.WorkContributionFinishBody{Effective: true,
			WorkContribution: workcontribution.WorkContribution{WorkKey: givenWork.Identifier, CheckitemId: 3, ContributorId: grantedUser.ID}},
			&session.Session{Identity: session.Identity{ID: sessionUser.ID}})
		Expect(err).To(Equal(gorm.ErrRecordNotFound))
	})

	t.Run("should be able to finish work contribution when work contribution is undergoing", func(t *testing.T) {
		defer workContributionTestTeardown(t, testDatabase)
		grantedUser, _, givenWork, sessionUser := workContributionTestSetup(t, &testDatabase)

		workcontribution.CheckContributorWorkPermissionFunc = func(workKey string, contributorId types.ID, sec *session.Session) (*domain.Work, *account.User, error) {
			return givenWork, grantedUser, nil
		}
		db := testDatabase.DS.GormDB()

		givenRecord2 := workcontribution.WorkContributionRecord{
			WorkContribution: workcontribution.WorkContribution{WorkKey: givenWork.Identifier, CheckitemId: 2, ContributorId: grantedUser.ID},
			ID:               1002, ContributorName: "user 200", BeginTime: types.TimestampOfDate(2021, 1, 1, 12, 0, 0, 0, time.Local),
			Effective: true,
		}
		Expect(db.Save(&givenRecord2).Error).To(BeNil())
		givenRecord1 := workcontribution.WorkContributionRecord{
			WorkContribution: workcontribution.WorkContribution{WorkKey: givenWork.Identifier, ContributorId: grantedUser.ID},
			ID:               1000, ContributorName: "user 200", BeginTime: types.TimestampOfDate(2021, 1, 1, 12, 0, 0, 0, time.Local),
			Effective: true,
		}
		Expect(db.Save(&givenRecord1).Error).To(BeNil())
		begin := time.Now()
		err := workcontribution.FinishWorkContribution(&workcontribution.WorkContributionFinishBody{Effective: true,
			WorkContribution: workcontribution.WorkContribution{WorkKey: givenWork.Identifier, ContributorId: grantedUser.ID}},
			&session.Session{Identity: session.Identity{ID: sessionUser.ID}})
		Expect(err).To(BeNil())

		record := workcontribution.WorkContributionRecord{ID: givenRecord1.ID}
		Expect(db.Where(&record).First(&record).Error).To(BeNil())
		Expect(record.ID).To(Equal(givenRecord1.ID))
		Expect(record.WorkKey).To(Equal(givenRecord1.WorkKey))
		Expect(record.ContributorId).To(Equal(givenRecord1.ContributorId))
		Expect(record.ContributorName).To(Equal(grantedUser.Nickname)) // updated
		Expect(record.WorkProjectId).To(Equal(givenWork.ProjectID))    // updated
		Expect(record.BeginTime).To(Equal(givenRecord1.BeginTime))     // begin time not change any more
		Expect(record.EndTime.Time().Unix() >= begin.Round(time.Microsecond).Unix() && record.EndTime.Time().Unix() <= time.Now().Round(time.Microsecond).Unix()).To(BeTrue())
		Expect(record.Effective).To(BeTrue()) // effective update to true

		// assert contribution with checkitem not changed
		record = workcontribution.WorkContributionRecord{ID: givenRecord2.ID}
		Expect(db.Where(&record).First(&record).Error).To(BeNil())
		Expect(record.EndTime).To(BeZero())
	})

	t.Run("should be able to finish work contribution when work contribution already finished", func(t *testing.T) {
		defer workContributionTestTeardown(t, testDatabase)
		grantedUser, _, givenWork, sessionUser := workContributionTestSetup(t, &testDatabase)

		workcontribution.CheckContributorWorkPermissionFunc = func(workKey string, contributorId types.ID, sec *session.Session) (*domain.Work, *account.User, error) {
			return givenWork, grantedUser, nil
		}
		db := testDatabase.DS.GormDB()

		givenRecord2 := workcontribution.WorkContributionRecord{
			WorkContribution: workcontribution.WorkContribution{WorkKey: givenWork.Identifier, CheckitemId: 2, ContributorId: grantedUser.ID},
			ID:               1002, ContributorName: "user 200", BeginTime: types.TimestampOfDate(2021, 1, 1, 12, 0, 0, 0, time.Local),
			EndTime: types.TimestampOfDate(2021, 1, 2, 12, 0, 0, 0, time.Local), Effective: true,
		}
		Expect(db.Save(&givenRecord2).Error).To(BeNil())
		err := workcontribution.FinishWorkContribution(&workcontribution.WorkContributionFinishBody{Effective: true,
			WorkContribution: workcontribution.WorkContribution{WorkKey: givenWork.Identifier, CheckitemId: 2, ContributorId: grantedUser.ID}},
			&session.Session{Identity: session.Identity{ID: sessionUser.ID}})
		Expect(err).To(BeNil())

		record := workcontribution.WorkContributionRecord{ID: givenRecord2.ID}
		Expect(db.Where(&record).First(&record).Error).To(BeNil())
		Expect(record.ID).To(Equal(givenRecord2.ID))
		Expect(record.WorkKey).To(Equal(givenRecord2.WorkKey))
		Expect(record.ContributorId).To(Equal(givenRecord2.ContributorId))
		Expect(record.ContributorName).To(Equal(grantedUser.Nickname)) // updated
		Expect(record.WorkProjectId).To(Equal(givenWork.ProjectID))    // updated
		Expect(record.BeginTime).To(Equal(givenRecord2.BeginTime))     // begin time not change any more
		Expect(record.EndTime).To(Equal(givenRecord2.EndTime))         // end time not change any more
		Expect(record.Effective).To(BeTrue())                          // effective not change any more
	})

	t.Run("should be able to finish work contribution when work contribution is discarded", func(t *testing.T) {
		defer workContributionTestTeardown(t, testDatabase)
		grantedUser, _, givenWork, sessionUser := workContributionTestSetup(t, &testDatabase)

		workcontribution.CheckContributorWorkPermissionFunc = func(workKey string, contributorId types.ID, sec *session.Session) (*domain.Work, *account.User, error) {
			return givenWork, grantedUser, nil
		}
		db := testDatabase.DS.GormDB()

		givenRecord2 := workcontribution.WorkContributionRecord{
			WorkContribution: workcontribution.WorkContribution{WorkKey: givenWork.Identifier, CheckitemId: 2, ContributorId: grantedUser.ID},
			ID:               1002, ContributorName: "user 200", BeginTime: types.TimestampOfDate(2021, 1, 1, 12, 0, 0, 0, time.Local),
			EndTime: types.TimestampOfDate(2021, 1, 2, 12, 0, 0, 0, time.Local), Effective: false,
		}
		Expect(db.Save(&givenRecord2).Error).To(BeNil())
		givenRecord1 := workcontribution.WorkContributionRecord{
			WorkContribution: workcontribution.WorkContribution{WorkKey: givenWork.Identifier, ContributorId: grantedUser.ID},
			ID:               1000, ContributorName: "user 200", BeginTime: types.TimestampOfDate(2021, 1, 1, 12, 0, 0, 0, time.Local),
			EndTime: types.TimestampOfDate(2021, 1, 2, 12, 0, 0, 0, time.Local), Effective: false,
		}
		Expect(db.Save(&givenRecord1).Error).To(BeNil())
		err := workcontribution.FinishWorkContribution(&workcontribution.WorkContributionFinishBody{Effective: true,
			WorkContribution: workcontribution.WorkContribution{WorkKey: givenWork.Identifier, ContributorId: grantedUser.ID}},
			&session.Session{Identity: session.Identity{ID: sessionUser.ID}})
		Expect(err).To(BeNil())

		record := workcontribution.WorkContributionRecord{ID: givenRecord1.ID}
		Expect(db.Where(&record).First(&record).Error).To(BeNil())
		Expect(record.ID).To(Equal(givenRecord1.ID))
		Expect(record.WorkKey).To(Equal(givenRecord1.WorkKey))
		Expect(record.ContributorId).To(Equal(givenRecord1.ContributorId))
		Expect(record.ContributorName).To(Equal(grantedUser.Nickname)) // updated
		Expect(record.WorkProjectId).To(Equal(givenWork.ProjectID))    // updated
		Expect(record.BeginTime).To(Equal(givenRecord1.BeginTime))     // begin time not change any more
		Expect(record.EndTime).To(Equal(givenRecord1.EndTime))         // end time not change any more
		Expect(record.Effective).To(BeTrue())                          // effective update to true

		// assert contribution with checkitem not changed
		record = workcontribution.WorkContributionRecord{ID: givenRecord2.ID}
		Expect(db.Where(&record).First(&record).Error).To(BeNil())
		Expect(record.Effective).To(BeFalse())
	})
}

func TestFinishWorkContributionDiscard(t *testing.T) {
	RegisterTestingT(t)
	var testDatabase *testinfra.TestDatabase

	t.Run("should return error when permission check failed", func(t *testing.T) {
		defer workContributionTestTeardown(t, testDatabase)
		grantedUser, _, givenWork, sessionUser := workContributionTestSetup(t, &testDatabase)
		testErr := errors.New("test error")

		workcontribution.CheckContributorWorkPermissionFunc = func(workKey string, contributorId types.ID, sec *session.Session) (*domain.Work, *account.User, error) {
			return nil, nil, testErr
		}
		err := workcontribution.FinishWorkContribution(&workcontribution.WorkContributionFinishBody{
			WorkContribution: workcontribution.WorkContribution{WorkKey: givenWork.Identifier, ContributorId: grantedUser.ID}},
			&session.Session{Identity: session.Identity{ID: sessionUser.ID}})
		Expect(err).To(Equal(testErr))
	})

	t.Run("should failed to discard work contribution when work contribution is not exist", func(t *testing.T) {
		defer workContributionTestTeardown(t, testDatabase)
		grantedUser, _, givenWork, sessionUser := workContributionTestSetup(t, &testDatabase)

		workcontribution.CheckContributorWorkPermissionFunc = func(workKey string, contributorId types.ID, sec *session.Session) (*domain.Work, *account.User, error) {
			return givenWork, grantedUser, nil
		}
		db := testDatabase.DS.GormDB()

		// case1
		givenRecord1 := workcontribution.WorkContributionRecord{
			WorkContribution: workcontribution.WorkContribution{WorkKey: givenWork.Identifier, CheckitemId: 2, ContributorId: grantedUser.ID},
			ID:               1000, ContributorName: "user 200", BeginTime: types.TimestampOfDate(2021, 1, 1, 12, 0, 0, 0, time.Local),
			Effective: true,
		}
		Expect(db.Save(&givenRecord1).Error).To(BeNil())

		err := workcontribution.FinishWorkContribution(&workcontribution.WorkContributionFinishBody{
			WorkContribution: workcontribution.WorkContribution{WorkKey: givenWork.Identifier, ContributorId: grantedUser.ID}},
			&session.Session{Identity: session.Identity{ID: sessionUser.ID}})
		Expect(err).To(Equal(gorm.ErrRecordNotFound))

		// case2
		givenRecord2 := workcontribution.WorkContributionRecord{
			WorkContribution: workcontribution.WorkContribution{WorkKey: givenWork.Identifier, ContributorId: grantedUser.ID},
			ID:               1002, ContributorName: "user 200", BeginTime: types.TimestampOfDate(2021, 1, 1, 12, 0, 0, 0, time.Local),
			Effective: true,
		}
		Expect(db.Save(&givenRecord2).Error).To(BeNil())
		err = workcontribution.FinishWorkContribution(&workcontribution.WorkContributionFinishBody{
			WorkContribution: workcontribution.WorkContribution{WorkKey: givenWork.Identifier, CheckitemId: 3, ContributorId: grantedUser.ID}},
			&session.Session{Identity: session.Identity{ID: sessionUser.ID}})
		Expect(err).To(Equal(gorm.ErrRecordNotFound))
	})

	t.Run("should be able to discard work contribution when work contribution is undergoing", func(t *testing.T) {
		defer workContributionTestTeardown(t, testDatabase)
		grantedUser, _, givenWork, sessionUser := workContributionTestSetup(t, &testDatabase)

		workcontribution.CheckContributorWorkPermissionFunc = func(workKey string, contributorId types.ID, sec *session.Session) (*domain.Work, *account.User, error) {
			return givenWork, grantedUser, nil
		}
		db := testDatabase.DS.GormDB()

		givenRecord2 := workcontribution.WorkContributionRecord{
			WorkContribution: workcontribution.WorkContribution{WorkKey: givenWork.Identifier, CheckitemId: 2, ContributorId: grantedUser.ID},
			ID:               1002, ContributorName: "user 200", BeginTime: types.TimestampOfDate(2021, 1, 1, 12, 0, 0, 0, time.Local),
			Effective: true,
		}
		Expect(db.Save(&givenRecord2).Error).To(BeNil())
		givenRecord1 := workcontribution.WorkContributionRecord{
			WorkContribution: workcontribution.WorkContribution{WorkKey: givenWork.Identifier, ContributorId: grantedUser.ID},
			ID:               1000, ContributorName: "user 200", BeginTime: types.TimestampOfDate(2021, 1, 1, 12, 0, 0, 0, time.Local),
			Effective: true,
		}
		Expect(db.Save(&givenRecord1).Error).To(BeNil())
		begin := time.Now()
		err := workcontribution.FinishWorkContribution(&workcontribution.WorkContributionFinishBody{
			WorkContribution: workcontribution.WorkContribution{WorkKey: givenWork.Identifier, ContributorId: grantedUser.ID}},
			&session.Session{Identity: session.Identity{ID: sessionUser.ID}})
		Expect(err).To(BeNil())

		record := workcontribution.WorkContributionRecord{ID: givenRecord1.ID}
		Expect(db.Where(&record).First(&record).Error).To(BeNil())
		Expect(record.ID).To(Equal(givenRecord1.ID))
		Expect(record.WorkKey).To(Equal(givenRecord1.WorkKey))
		Expect(record.ContributorId).To(Equal(givenRecord1.ContributorId))
		Expect(record.ContributorName).To(Equal(grantedUser.Nickname)) // updated
		Expect(record.WorkProjectId).To(Equal(givenWork.ProjectID))    // updated
		Expect(record.BeginTime).To(Equal(givenRecord1.BeginTime))     // begin time not change any more
		Expect(record.EndTime.Time().Unix() >= begin.Round(time.Microsecond).Unix() && record.EndTime.Time().Unix() <= time.Now().Round(time.Microsecond).Unix()).To(BeTrue())
		Expect(record.Effective).To(BeFalse()) // effective update to false

		// assert contribution with checkitem not changed
		record = workcontribution.WorkContributionRecord{ID: givenRecord2.ID}
		Expect(db.Where(&record).First(&record).Error).To(BeNil())
		Expect(record.EndTime).To(BeZero())
	})

	t.Run("should be able to discard work contribution when work contribution already finished", func(t *testing.T) {
		defer workContributionTestTeardown(t, testDatabase)
		grantedUser, _, givenWork, sessionUser := workContributionTestSetup(t, &testDatabase)

		workcontribution.CheckContributorWorkPermissionFunc = func(workKey string, contributorId types.ID, sec *session.Session) (*domain.Work, *account.User, error) {
			return givenWork, grantedUser, nil
		}
		db := testDatabase.DS.GormDB()

		givenRecord2 := workcontribution.WorkContributionRecord{
			WorkContribution: workcontribution.WorkContribution{WorkKey: givenWork.Identifier, CheckitemId: 2, ContributorId: grantedUser.ID},
			ID:               1002, ContributorName: "user 200", BeginTime: types.TimestampOfDate(2021, 1, 1, 12, 0, 0, 0, time.Local),
			EndTime: types.TimestampOfDate(2021, 1, 2, 12, 0, 0, 0, time.Local), Effective: true,
		}
		Expect(db.Save(&givenRecord2).Error).To(BeNil())
		givenRecord1 := workcontribution.WorkContributionRecord{
			WorkContribution: workcontribution.WorkContribution{WorkKey: givenWork.Identifier, ContributorId: grantedUser.ID},
			ID:               1000, ContributorName: "user 200", BeginTime: types.TimestampOfDate(2021, 1, 1, 12, 0, 0, 0, time.Local),
			EndTime: types.TimestampOfDate(2021, 1, 2, 12, 0, 0, 0, time.Local), Effective: true,
		}
		Expect(db.Save(&givenRecord1).Error).To(BeNil())
		err := workcontribution.FinishWorkContribution(&workcontribution.WorkContributionFinishBody{
			WorkContribution: workcontribution.WorkContribution{WorkKey: givenWork.Identifier, CheckitemId: 2, ContributorId: grantedUser.ID}},
			&session.Session{Identity: session.Identity{ID: sessionUser.ID}})
		Expect(err).To(BeNil())

		record := workcontribution.WorkContributionRecord{ID: givenRecord2.ID}
		Expect(db.Where(&record).First(&record).Error).To(BeNil())
		Expect(record.ID).To(Equal(givenRecord2.ID))
		Expect(record.WorkKey).To(Equal(givenRecord2.WorkKey))
		Expect(record.ContributorId).To(Equal(givenRecord2.ContributorId))
		Expect(record.ContributorName).To(Equal(grantedUser.Nickname)) // updated
		Expect(record.WorkProjectId).To(Equal(givenWork.ProjectID))    // updated
		Expect(record.BeginTime).To(Equal(givenRecord2.BeginTime))     // begin time not change any more
		Expect(record.EndTime).To(Equal(givenRecord2.EndTime))         // end time not change any more
		Expect(record.Effective).To(BeFalse())                         // effective update to false

		// assert contribution without checkitem not changed
		record = workcontribution.WorkContributionRecord{ID: givenRecord1.ID}
		Expect(db.Where(&record).First(&record).Error).To(BeNil())
		Expect(record.Effective).To(BeTrue())
	})

	t.Run("should be able to discard work contribution when work contribution is discarded", func(t *testing.T) {
		defer workContributionTestTeardown(t, testDatabase)
		grantedUser, _, givenWork, sessionUser := workContributionTestSetup(t, &testDatabase)

		workcontribution.CheckContributorWorkPermissionFunc = func(workKey string, contributorId types.ID, sec *session.Session) (*domain.Work, *account.User, error) {
			return givenWork, grantedUser, nil
		}
		db := testDatabase.DS.GormDB()
		givenRecord1 := workcontribution.WorkContributionRecord{
			WorkContribution: workcontribution.WorkContribution{WorkKey: givenWork.Identifier, ContributorId: grantedUser.ID},
			ID:               1002, ContributorName: "user 200", BeginTime: types.TimestampOfDate(2021, 1, 1, 12, 0, 0, 0, time.Local),
			EndTime: types.TimestampOfDate(2021, 1, 2, 12, 0, 0, 0, time.Local), Effective: false,
		}
		Expect(db.Save(&givenRecord1).Error).To(BeNil())
		err := workcontribution.FinishWorkContribution(&workcontribution.WorkContributionFinishBody{
			WorkContribution: workcontribution.WorkContribution{WorkKey: givenWork.Identifier, ContributorId: grantedUser.ID}},
			&session.Session{Identity: session.Identity{ID: sessionUser.ID}})
		Expect(err).To(BeNil())

		record := workcontribution.WorkContributionRecord{ID: givenRecord1.ID}
		Expect(db.Where(&record).First(&record).Error).To(BeNil())
		Expect(record.ID).To(Equal(givenRecord1.ID))
		Expect(record.WorkKey).To(Equal(givenRecord1.WorkKey))
		Expect(record.ContributorId).To(Equal(givenRecord1.ContributorId))
		Expect(record.ContributorName).To(Equal(grantedUser.Nickname)) // updated
		Expect(record.WorkProjectId).To(Equal(givenWork.ProjectID))    // updated
		Expect(record.BeginTime).To(Equal(givenRecord1.BeginTime))     // begin time not change any more
		Expect(record.EndTime).To(Equal(givenRecord1.EndTime))         // end time not change any more
		Expect(record.Effective).To(BeFalse())                         // effective not change any more
	})
}

func TestQueryWorkContributions(t *testing.T) {
	RegisterTestingT(t)
	var testDatabase *testinfra.TestDatabase

	t.Run("should get empty result when no work keys", func(t *testing.T) {
		defer workContributionTestTeardown(t, testDatabase)
		_, _, _, sessionUser := workContributionTestSetup(t, &testDatabase)

		result, err := workcontribution.QueryWorkContributions(
			workcontribution.WorkContributionsQuery{}, &session.Session{Identity: session.Identity{ID: sessionUser.ID}})
		Expect(err).To(BeNil())
		Expect(result).ToNot(BeNil())
		Expect(len(*result)).To(BeZero())

		result, err = workcontribution.QueryWorkContributions(
			workcontribution.WorkContributionsQuery{WorkKeys: []string{}}, &session.Session{Identity: session.Identity{ID: sessionUser.ID}})
		Expect(err).To(BeNil())
		Expect(result).ToNot(BeNil())
		Expect(len(*result)).To(BeZero())
	})

	t.Run("should get empty result when work keys not exists", func(t *testing.T) {
		defer workContributionTestTeardown(t, testDatabase)
		_, _, _, sessionUser := workContributionTestSetup(t, &testDatabase)

		result, err := workcontribution.QueryWorkContributions(
			workcontribution.WorkContributionsQuery{WorkKeys: []string{"TEST_404"}}, &session.Session{Identity: session.Identity{ID: sessionUser.ID}})
		Expect(err).To(BeNil())
		Expect(result).ToNot(BeNil())
		Expect(len(*result)).To(BeZero())
	})

	t.Run("should be able to get correct result", func(t *testing.T) {
		grantedUser, _, givenWork, sessionUser := workContributionTestSetup(t, &testDatabase)

		workcontribution.CheckContributorWorkPermissionFunc = func(workKey string, contributorId types.ID, sec *session.Session) (*domain.Work, *account.User, error) {
			return givenWork, grantedUser, nil
		}
		db := testDatabase.DS.GormDB()

		givenRecord1 := workcontribution.WorkContributionRecord{
			WorkContribution: workcontribution.WorkContribution{WorkKey: givenWork.Identifier, ContributorId: grantedUser.ID},
			ID:               1000, ContributorName: "user 200", BeginTime: types.TimestampOfDate(2021, 1, 1, 12, 0, 0, 0, time.Local),
			EndTime: types.TimestampOfDate(2021, 1, 2, 12, 0, 0, 0, time.Local), Effective: true,
			WorkProjectId: 300,
		}
		Expect(db.Save(&givenRecord1).Error).To(BeNil())
		givenRecord2 := workcontribution.WorkContributionRecord{
			WorkContribution: workcontribution.WorkContribution{WorkKey: "TEST_404", ContributorId: grantedUser.ID},
			ID:               1002, ContributorName: "user 200", BeginTime: types.TimestampOfDate(2021, 1, 1, 13, 0, 0, 0, time.Local),
			EndTime: types.TimestampOfDate(2021, 1, 2, 13, 0, 0, 0, time.Local), Effective: true,
			WorkProjectId: 404,
		}
		Expect(db.Save(&givenRecord2).Error).To(BeNil())

		// should be able to get result of all project for system admin
		result, err := workcontribution.QueryWorkContributions(
			workcontribution.WorkContributionsQuery{WorkKeys: []string{givenRecord1.WorkKey, givenRecord2.WorkKey}},
			&session.Session{Identity: session.Identity{ID: sessionUser.ID}, Perms: []string{account.SystemAdminPermission.ID}})
		Expect(err).To(BeNil())
		Expect(len(*result)).To(Equal(2))
		Expect((*result)[0]).To(Equal(givenRecord1))
		Expect((*result)[1]).To(Equal(givenRecord2))

		result, err = workcontribution.QueryWorkContributions(
			workcontribution.WorkContributionsQuery{WorkKeys: []string{givenRecord2.WorkKey}},
			&session.Session{Identity: session.Identity{ID: sessionUser.ID}, Perms: []string{account.SystemAdminPermission.ID}})
		Expect(err).To(BeNil())
		Expect(len(*result)).To(Equal(1))
		Expect((*result)[0]).To(Equal(givenRecord2))

		// should be able to get result from visible projects for non system admin
		// session.Context.VisibleProjects()
		result, err = workcontribution.QueryWorkContributions(
			workcontribution.WorkContributionsQuery{WorkKeys: []string{givenRecord1.WorkKey, givenRecord2.WorkKey}},
			&session.Session{Identity: session.Identity{ID: sessionUser.ID}, Perms: []string{"guest_404"}})
		Expect(err).To(BeNil())
		Expect(len(*result)).To(Equal(1))
		Expect((*result)[0]).To(Equal(givenRecord2))
	})
}
