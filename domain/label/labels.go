package label

import (
	"flywheel/bizerror"
	"flywheel/idgen"
	"flywheel/persistence"
	"flywheel/session"

	"github.com/fundwit/go-commons/types"
	"github.com/sony/sonyflake"
)

type LabelCreation struct {
	Name      string   `json:"name" binding:"required,lte=255"`
	ProjectID types.ID `json:"projectId" binding:"required"`
}

type LabelQuery struct {
	ProjectID types.ID `binding:"required" json:"projectId" form:"projectId"`
}

type Label struct {
	ID types.ID `json:"id"`

	Name      string   `json:"name" binding:"required,lte=255"`
	ProjectID types.ID `json:"projectId" binding:"required"`

	CreatorID  types.ID        `json:"creatorId"`
	CreateTime types.Timestamp `json:"createTime" sql:"type:DATETIME(6) NOT NULL"`
}

var (
	labelIdWorker = sonyflake.NewSonyflake(sonyflake.Settings{})

	CreateLabelFunc = CreateLabel
	QueryLabelsFunc = QueryLabels
)

func CreateLabel(l LabelCreation, ctx *session.Context) (*Label, error) {
	if !ctx.Perms.HasRoleSuffix("_" + l.ProjectID.String()) {
		return nil, bizerror.ErrForbidden
	}

	r := Label{Name: l.Name, ProjectID: l.ProjectID, ID: idgen.NextID(labelIdWorker),
		CreatorID:  ctx.Identity.ID,
		CreateTime: types.CurrentTimestamp()}
	if err := persistence.ActiveDataSourceManager.GormDB().Create(&r).Error; err != nil {
		return nil, err
	}

	return &r, nil
}

func QueryLabels(q LabelQuery, ctx *session.Context) ([]Label, error) {
	if !ctx.Perms.HasRoleSuffix("_" + q.ProjectID.String()) {
		return nil, bizerror.ErrForbidden
	}

	labels := []Label{}
	db := persistence.ActiveDataSourceManager.GormDB()
	if err := db.Order("ID ASC").Where("project_id = ?", q.ProjectID).Find(&labels).Error; err != nil {
		return nil, err
	}
	return labels, nil
}
