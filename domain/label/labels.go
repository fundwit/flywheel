package label

import (
	"flywheel/bizerror"
	"flywheel/idgen"
	"flywheel/persistence"
	"flywheel/session"

	"github.com/fundwit/go-commons/types"
	"github.com/jinzhu/gorm"
	"github.com/sony/sonyflake"
)

var (
	LabelDeleteCheckFuncs []func(l Label, tx *gorm.DB) error
)

type LabelCreation struct {
	Name       string   `json:"name" binding:"required,lte=255"`
	ThemeColor string   `json:"themeColor" binding:"required,lte=64"`
	ProjectID  types.ID `json:"projectId" binding:"required"`
}

type LabelQuery struct {
	ProjectID types.ID `binding:"required" json:"projectId" form:"projectId"`
}

type Label struct {
	ID types.ID `json:"id"`

	Name       string   `json:"name" binding:"required,lte=255" gorm:"unique_index:uni_name_project"`
	ThemeColor string   `json:"themeColor" binding:"required,lte=64"`
	ProjectID  types.ID `json:"projectId" binding:"required" gorm:"unique_index:uni_name_project"`

	CreatorID  types.ID        `json:"creatorId"`
	CreateTime types.Timestamp `json:"createTime" sql:"type:DATETIME(6) NOT NULL"`
}

type LabelBrief struct {
	ID types.ID `json:"id"`

	Name       string `json:"name"`
	ThemeColor string `json:"themeColor"`
}

var (
	labelIdWorker = sonyflake.NewSonyflake(sonyflake.Settings{})

	CreateLabelFunc = CreateLabel
	QueryLabelsFunc = QueryLabels
	DeleteLabelFunc = DeleteLabel
)

func CreateLabel(l LabelCreation, s *session.Session) (*Label, error) {
	if !s.Perms.HasAnyProjectRole(l.ProjectID) {
		return nil, bizerror.ErrForbidden
	}

	r := Label{Name: l.Name, ThemeColor: l.ThemeColor, ProjectID: l.ProjectID, ID: idgen.NextID(labelIdWorker),
		CreatorID:  s.Identity.ID,
		CreateTime: types.CurrentTimestamp()}
	if err := persistence.ActiveDataSourceManager.GormDB(s.Context).Create(&r).Error; err != nil {
		return nil, err
	}

	return &r, nil
}

func QueryLabels(q LabelQuery, s *session.Session) ([]Label, error) {
	if !s.Perms.HasAnyProjectRole(q.ProjectID) {
		return nil, bizerror.ErrForbidden
	}

	labels := []Label{}
	db := persistence.ActiveDataSourceManager.GormDB(s.Context)
	if err := db.Order("ID ASC").Where("project_id = ?", q.ProjectID).Find(&labels).Error; err != nil {
		return nil, err
	}
	return labels, nil
}

func DeleteLabel(id types.ID, s *session.Session) error {
	err1 := persistence.ActiveDataSourceManager.GormDB(s.Context).Transaction(func(tx *gorm.DB) error {
		l, err := findLabelAndCheckPerms(tx, id, s)
		if err != nil {
			return err
		}

		for _, f := range LabelDeleteCheckFuncs {
			if err := f(*l, tx); err != nil {
				return err
			}
		}

		if err := tx.Delete(Label{}, "id = ?", id).Error; err != nil {
			return err
		}
		return nil
	})
	return err1
}

func findLabelAndCheckPerms(db *gorm.DB, id types.ID, s *session.Session) (*Label, error) {
	var l Label
	if err := db.Where("id = ?", id).First(&l).Error; err != nil {
		return nil, err
	}
	if s == nil || !s.Perms.HasAnyProjectRole(l.ProjectID) {
		return nil, bizerror.ErrForbidden
	}
	return &l, nil
}
