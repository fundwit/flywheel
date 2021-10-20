package flow

import (
	"errors"
	"flywheel/bizerror"
	"flywheel/domain"
	"flywheel/idgen"
	"flywheel/persistence"
	"flywheel/session"

	"github.com/fundwit/go-commons/types"
	"github.com/jinzhu/gorm"
	"github.com/sony/sonyflake"
)

var (
	propertyDefinitionIdWorker   = sonyflake.NewSonyflake(sonyflake.Settings{})
	CreatePropertyDefinitionFunc = CreatePropertyDefinition
	QueryPropertyDefinitionsFunc = QueryPropertyDefinitions
	DeletePropertyDefinitionFunc = DeletePropertyDefinition

	PropertyDefinitionDeleteCheckFuncs = []func(d WorkflowPropertyDefinition, db *gorm.DB) error{}
)

type WorkflowPropertyDefinition struct {
	ID         types.ID `json:"id"`
	WorkflowID types.ID `json:"workflowId" gorm:"unique_index:uni_workflow_prop"`

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

	if !s.Perms.HasProjectViewPerm(w.ProjectID) {
		return nil, bizerror.ErrForbidden
	}

	records := []WorkflowPropertyDefinition{}
	if err := db.Where("workflow_id = ?", workflowId).Find(&records).Error; err != nil {
		return nil, err
	}

	return records, nil
}

func DeletePropertyDefinition(id types.ID, s *session.Session) error {
	db := persistence.ActiveDataSourceManager.GormDB(s.Context)

	p := WorkflowPropertyDefinition{}
	if dbErr := db.Where("id = ?", id).First(&p).Error; errors.Is(dbErr, gorm.ErrRecordNotFound) {
		return nil
	} else if dbErr != nil {
		return dbErr
	}

	// query workflow, use workflow's containing project to determine permission
	w := domain.Workflow{}
	if dbErr := db.Model(&w).Where("id = ?", p.WorkflowID).First(&w).Error; errors.Is(dbErr, gorm.ErrRecordNotFound) {
		// workflow not found, delete directly
		return db.Where("id = ?", id).Delete(&WorkflowPropertyDefinition{ID: id}).Error
	} else if dbErr != nil {
		return dbErr
	}

	if !s.Perms.HasProjectRole(domain.ProjectRoleManager, w.ProjectID) {
		return bizerror.ErrForbidden
	}

	dbErr := db.Transaction(func(tx *gorm.DB) error {
		for _, checkFunc := range PropertyDefinitionDeleteCheckFuncs {
			err := checkFunc(p, tx)
			if err != nil {
				return err
			}
		}

		return db.Where("id = ?", id).Delete(&WorkflowPropertyDefinition{ID: id}).Error
	})

	return dbErr
}
