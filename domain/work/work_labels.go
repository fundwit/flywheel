package work

import (
	"flywheel/bizerror"
	"flywheel/domain/label"
	"flywheel/persistence"
	"flywheel/session"

	"github.com/fundwit/go-commons/types"
	"github.com/jinzhu/gorm"
	otgorm "github.com/smacker/opentracing-gorm"
)

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

func QueryLabelBriefsOfWork(workId types.ID, s *session.Session) ([]label.LabelBrief, error) {
	var labelBriefs []label.LabelBrief
	if workId.IsZero() {
		return labelBriefs, nil
	}

	db := otgorm.SetSpanToGorm(s.Context, persistence.ActiveDataSourceManager.GormDB(s.Context))
	if err := db.Model(&WorkLabelRelation{}).
		Select("labels.id, labels.name, labels.theme_color").
		Where(&WorkLabelRelation{WorkId: workId}).
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
