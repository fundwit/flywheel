package namespace

import (
	"flywheel/common"
	"flywheel/domain"
	"flywheel/persistence"
	"flywheel/security"
	"github.com/fundwit/go-commons/types"
	"github.com/jinzhu/gorm"
	"github.com/sony/sonyflake"
	"time"
)

type GroupManagerTraits interface {
	CreateGroup(name string, sec *security.Context) (*domain.Group, error)
	QueryGroupRole(groupId types.ID, sec *security.Context) (string, error)
}

type GroupManager struct {
	dataSource *persistence.DataSourceManager
	idWorker   *sonyflake.Sonyflake
}

func NewGroupManager(ds *persistence.DataSourceManager) GroupManagerTraits {
	return &GroupManager{
		dataSource: ds,
		idWorker:   sonyflake.NewSonyflake(sonyflake.Settings{}),
	}
}

func (m *GroupManager) CreateGroup(name string, sec *security.Context) (*domain.Group, error) {
	now := time.Now()
	g := domain.Group{ID: common.NextId(m.idWorker), Name: name, CreateTime: now, Creator: sec.Identity.ID}
	r := domain.GroupMember{GroupID: g.ID, MemberId: sec.Identity.ID, Role: domain.RoleOwner, CreateTime: now}
	err := m.dataSource.GormDB().Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(g).Error; err != nil {
			return err
		}
		if err := tx.Create(r).Error; err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &g, nil
}

func (m *GroupManager) QueryGroupRole(groupId types.ID, sec *security.Context) (string, error) {
	gm := domain.GroupMember{GroupID: groupId, MemberId: sec.Identity.ID}
	db := m.dataSource.GormDB()
	var founds []domain.GroupMember
	if err := db.Model(domain.GroupMember{}).Where(&gm).Find(&founds).Error; err != nil || founds == nil || len(founds) == 0 {
		return "", err
	}
	return founds[0].Role, nil
}
