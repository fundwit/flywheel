package flow

import (
	"flywheel/bizerror"
	"flywheel/domain"
	"flywheel/idgen"
	"flywheel/persistence"
	"flywheel/session"

	"github.com/fundwit/go-commons/types"
	"github.com/sony/sonyflake"
)

var (
	propertyDefinitionIdWorker   = sonyflake.NewSonyflake(sonyflake.Settings{})
	CreatePropertyDefinitionFunc = CreatePropertyDefinition
	QueryPropertyDefinitionsFunc = QueryPropertyDefinitions
)

type WorkflowPropertyDefinition struct {
	ID         types.ID `json:"id"`
	WorkflowID types.ID `json:"workflowId"`

	domain.PropertyDefinition
}

func CreatePropertyDefinition(workflowId types.ID, p domain.PropertyDefinition, s *session.Session) (*WorkflowPropertyDefinition, error) {
	// workflow must be exist
	w := domain.Workflow{ID: workflowId}
	db := persistence.ActiveDataSourceManager.GormDB(s.Context)
	if err := db.Model(&w).First(&w).Error; err != nil {
		return nil, err
	}

	// session user must be manager of workflow's containing project
	if !s.Perms.HasProjectRole(domain.ProjectRoleManager, w.ProjectID) {
		return nil, bizerror.ErrForbidden
	}

	// save record
	r := WorkflowPropertyDefinition{
		ID:                 idgen.NextID(propertyDefinitionIdWorker),
		WorkflowID:         workflowId,
		PropertyDefinition: p,
	}
	if err := db.Create(&r).Error; err != nil {
		return nil, err
	}

	return &r, nil
}

func QueryPropertyDefinitions(workflowId types.ID, s *session.Session) ([]WorkflowPropertyDefinition, error) {
	// workflow must be exist
	w := domain.Workflow{ID: workflowId}
	db := persistence.ActiveDataSourceManager.GormDB(s.Context)
	if err := db.Model(&w).First(&w).Error; err != nil {
		return nil, err
	}

	// session user must be manager of workflow's containing project
	if !s.Perms.HasProjectViewPerm(w.ProjectID) {
		return nil, bizerror.ErrForbidden
	}

	records := []WorkflowPropertyDefinition{}
	if err := db.Where("workflow_id = ?", workflowId).Find(&records).Error; err != nil {
		return nil, err
	}

	return records, nil
}
