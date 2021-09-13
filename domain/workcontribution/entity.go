package workcontribution

import (
	"github.com/fundwit/go-commons/types"
)

type WorkContributionRecord struct {
	ID types.ID `json:"id" gorm:"primary_key"`

	WorkContribution

	WorkProjectId   types.ID `json:"workProjectId"`
	ContributorName string   `json:"contributorName"`

	BeginTime types.Timestamp `json:"beginTime" sql:"type:DATETIME(6) NOT NULL"`
	EndTime   types.Timestamp `json:"endTime" sql:"type:DATETIME(6) NOT NULL"`

	Effective bool `json:"effective"`
}

func (r *WorkContributionRecord) TableName() string {
	return "work_contributions"
}

type WorkContribution struct {
	WorkKey       string   `json:"workKey" binding:"required" gorm:"unique_index:work_contributor_checkitem_unique"`
	ContributorId types.ID `json:"contributorId" binding:"required" gorm:"unique_index:work_contributor_checkitem_unique"`
	CheckitemId   types.ID `json:"checkitemId" gorm:"unique_index:work_contributor_checkitem_unique"`
}

type WorkContributionFinishBody struct {
	WorkContribution
	Effective bool `json:"effective"`
}
