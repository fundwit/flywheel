package work

import (
	"flywheel/bizerror"
	"flywheel/domain/label"
	"flywheel/persistence"
	"flywheel/session"

	"github.com/fundwit/go-commons/types"
	"github.com/jinzhu/gorm"
)

type WorkLabelBrief struct {
	WorkID types.ID `json:"workId"`

	LabelID         types.ID `json:"labelId"`
	LabelName       string   `json:"labelName"`
	LabelThemeColor string   `json:"labelThemeColor"`
}

type WorkLabelRelation struct {
	LabelId types.ID `json:"labelId" gorm:"primary_key" sql:"type:BIGINT UNSIGNED NOT NULL" binding:"required"`
	WorkId  types.ID `json:"workId" gorm:"primary_key" sql:"type:BIGINT UNSIGNED NOT NULL" binding:"required"`

	CreateTime types.Timestamp `json:"createTime" sql:"type:DATETIME(6)" binding:"required"`
	CreatorId  types.ID        `json:"creatorId" binding:"required"`
}

type WorkLabelRelationReq struct {
	WorkId  types.ID `json:"workId" form:"workId" binding:"required"`
	LabelId types.ID `json:"labelId" form:"labelId" binding:"required"`
}

var (
	CreateWorkLabelRelationFunc = CreateWorkLabelRelation
	DeleteWorkLabelRelationFunc = DeleteWorkLabelRelation
	ClearWorkLabelRelationsFunc = clearWorkLabelRelations
)

func IsLabelReferencedByWork(l label.Label, tx *gorm.DB) error {
	var r WorkLabelRelation
	if err := tx.Where(&WorkLabelRelation{LabelId: l.ID}).First(&r).Error; err == gorm.ErrRecordNotFound {
		return nil
	} else if err != nil {
		return err
	}
	return bizerror.ErrLabelIsReferenced
}

func QueryLabelBriefsOfWork(workIds []types.ID, s *session.Session) ([]WorkLabelBrief, error) {
	var labelBriefs []WorkLabelBrief
	if len(workIds) == 0 {
		return labelBriefs, nil
	}

	db := persistence.ActiveDataSourceManager.GormDB(s.Context)
	if err := db.LogMode(true).Model(&WorkLabelRelation{}).
		Select("work_label_relations.work_id, work_label_relations.label_id, labels.name as label_name, labels.theme_color as label_theme_color").
		Where("work_label_relations.work_id IN (?)", workIds).
		Joins("INNER JOIN labels ON labels.id = work_label_relations.label_id").
		Scan(&labelBriefs).Error; err != nil {
		return nil, err
	}

	return labelBriefs, nil
}

func CreateWorkLabelRelation(req WorkLabelRelationReq, c *session.Session) (*WorkLabelRelation, error) {
	var r *WorkLabelRelation
	txErr := persistence.ActiveDataSourceManager.GormDB(c.Context).Transaction(func(tx *gorm.DB) error {
		// load work, check perms against to project of work
		w, err := findWorkAndCheckPerms(tx, req.WorkId, c)
		if err != nil {
			return err
		}
		var l label.Label
		if err := tx.Where(&label.Label{ID: req.LabelId, ProjectID: w.ProjectID}).First(&l).Error; err == gorm.ErrRecordNotFound {
			return bizerror.ErrLabelNotFound
		} else if err != nil {
			return err
		}

		r = &WorkLabelRelation{
			WorkId: w.ID, LabelId: l.ID,
			CreateTime: types.CurrentTimestamp(), CreatorId: c.Identity.ID,
		}

		if err := tx.Save(&r).Error; err != nil {
			return err
		}
		return nil
	})

	if txErr != nil {
		return nil, txErr
	}

	return r, nil
}

func DeleteWorkLabelRelation(req WorkLabelRelationReq, c *session.Session) error {
	if (req == WorkLabelRelationReq{}) {
		return bizerror.ErrInvalidArguments
	}

	err1 := persistence.ActiveDataSourceManager.GormDB(c.Context).Transaction(func(tx *gorm.DB) error {
		// load work, check perms against to project of work
		w, err := findWorkAndCheckPerms(tx, req.WorkId, c)
		if err != nil {
			return err
		}

		return tx.Delete(&WorkLabelRelation{}, &WorkLabelRelation{WorkId: w.ID, LabelId: req.LabelId}).Error
	})
	if err1 != nil {
		return err1
	}
	return nil
}

func clearWorkLabelRelations(workID types.ID, tx *gorm.DB) error {
	if workID == types.ID(0) {
		return nil
	}
	return tx.Delete(&WorkLabelRelation{}, "work_id = ?", workID).Error
}
