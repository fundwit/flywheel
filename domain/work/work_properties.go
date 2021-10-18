package work

import (
	"flywheel/bizerror"
	"flywheel/domain"
	"flywheel/domain/flow"
	"flywheel/persistence"
	"flywheel/session"

	"github.com/fundwit/go-commons/types"
	"github.com/jinzhu/gorm"
)

type WorkPropertyValueRecord struct {
	WorkId types.ID `json:"workId" gorm:"primary_key" sql:"type:BIGINT UNSIGNED NOT NULL" binding:"required"`
	Name   string   `json:"name" gorm:"primary_key" binding:"required"`

	Value string `json:"value"`
	Type  string `json:"type" binding:"required,oneof=text number"`

	PropertyDefinitionId types.ID `json:"propertyDefinitionId" sql:"type:BIGINT UNSIGNED NOT NULL" binding:"required"`
}

func (r *WorkPropertyValueRecord) TableName() string {
	return "work_property_values"
}

type WorkPropertyAssign struct {
	WorkId types.ID `json:"workId" binding:"required"`

	Name  string `json:"name" binding:"required"`
	Value string `json:"value"`
}

type WorkPropertyValueDetail struct {
	PropertyDefinitionId types.ID `json:"propertyDefinitionId"`
	Value                string   `json:"value"`

	domain.PropertyDefinition
}

type WorksPropertyValueDetail struct {
	WorkId         types.ID                  `json:"workId"`
	PropertyValues []WorkPropertyValueDetail `json:"propertyValues"`
}

var (
	AssignWorkPropertyValueFunc = AssignWorkPropertyValue
)

func AssignWorkPropertyValue(req WorkPropertyAssign, c *session.Session) (*WorkPropertyValueRecord, error) {
	var r *WorkPropertyValueRecord
	txErr := persistence.ActiveDataSourceManager.GormDB(c.Context).Transaction(func(tx *gorm.DB) error {
		w, err := findWorkAndCheckPerms(tx, req.WorkId, c)
		if err != nil {
			return err
		}
		d := flow.WorkflowPropertyDefinition{}
		if err := tx.Model(&d).Where("workflow_id = ? AND name LIKE ?", w.FlowID, req.Name).First(&d).Error; err == gorm.ErrRecordNotFound {
			return bizerror.ErrPropertyDefinitionNotFound
		} else if err != nil {
			return err
		}

		r = &WorkPropertyValueRecord{
			WorkId: w.ID, Name: req.Name, Value: req.Value,
			PropertyDefinitionId: d.ID, Type: d.Type,
		}

		if err := tx.Save(r).Error; err != nil {
			return err
		}

		if req.Value == "" {
			if err := tx.Model(r).Update("value", "").Error; err != nil {
				return err
			}
		}

		return nil
	})

	if txErr != nil {
		return nil, txErr
	}

	return r, nil
}

func IsPropertyDefinitionReferencedByWork(propDefinitionId types.ID, tx *gorm.DB) error {
	r := WorkPropertyValueRecord{}
	if err := tx.Model(&r).Where("property_definition_id = ?", propDefinitionId).First(&r).Error; err == gorm.ErrRecordNotFound {
		return nil
	} else if err != nil {
		return err
	}
	return bizerror.ErrPropertyDefinitionIsReferenced
}

type workIdWithFlowId struct {
	ID     types.ID
	FlowID types.ID
}

func QueryWorkPropertyValues(reqWorkIds []types.ID, s *session.Session) ([]WorksPropertyValueDetail, error) {
	var worksPropertyValues []WorksPropertyValueDetail
	if len(reqWorkIds) == 0 {
		return worksPropertyValues, nil
	}
	visibleProjects := s.VisibleProjects()
	if len(visibleProjects) == 0 {
		return worksPropertyValues, nil
	}

	workIdFlowIdMap := map[types.ID]types.ID{}
	visibleWorkIds := []types.ID{}

	flowDefinesMap := map[types.ID][]flow.WorkflowPropertyDefinition{}

	values := []WorkPropertyValueRecord{}
	defines := []flow.WorkflowPropertyDefinition{}

	dbErr := persistence.ActiveDataSourceManager.GormDB(s.Context).Transaction(func(tx *gorm.DB) error {
		visibleWorkIdWithFlowId := []workIdWithFlowId{}
		if err := tx.Model(&domain.Work{}).
			Where("id IN (?) AND project_id IN (?)", reqWorkIds, visibleProjects).Scan(&visibleWorkIdWithFlowId).Error; err != nil {
			return err
		}

		if len(visibleWorkIdWithFlowId) == 0 {
			return nil
		}

		flowIdSet := map[types.ID]bool{}
		for _, wf := range visibleWorkIdWithFlowId {
			workIdFlowIdMap[wf.ID] = wf.FlowID
			flowIdSet[wf.FlowID] = true
		}
		flowIds := []types.ID{}
		for flowId := range flowIdSet {
			flowIds = append(flowIds, flowId)
		}
		for workId := range workIdFlowIdMap {
			visibleWorkIds = append(visibleWorkIds, workId)
		}

		if err := tx.Model(&WorkPropertyValueRecord{}).Where("work_id IN (?)", visibleWorkIds).Find(&values).Error; err != nil {
			return nil
		}

		if err := tx.Model(&flow.WorkflowPropertyDefinition{}).Where("workflow_id IN (?)", flowIds).Find(&defines).Error; err != nil {
			return err
		}

		return nil
	})

	if dbErr != nil {
		return nil, dbErr
	}

	for _, define := range defines {
		array := flowDefinesMap[define.WorkflowID]
		if len(array) == 0 {
			array = []flow.WorkflowPropertyDefinition{}
		}
		array = append(array, define)
		flowDefinesMap[define.WorkflowID] = array
	}

	workValueMap := map[string]string{}
	for _, wv := range values {
		workValueMap[wv.WorkId.String()+"_"+wv.PropertyDefinitionId.String()] = wv.Value
	}

	for _, workId := range visibleWorkIds {
		wpv := WorksPropertyValueDetail{WorkId: workId}
		pvList := []WorkPropertyValueDetail{}

		flowId := workIdFlowIdMap[workId]
		flowDefines := flowDefinesMap[flowId]

		for _, define := range flowDefines {
			pv := WorkPropertyValueDetail{PropertyDefinitionId: define.ID, PropertyDefinition: define.PropertyDefinition}
			pv.Value = workValueMap[workId.String()+"_"+define.ID.String()]
			pvList = append(pvList, pv)
		}

		wpv.PropertyValues = pvList
		worksPropertyValues = append(worksPropertyValues, wpv)
	}

	return worksPropertyValues, nil
}
