package checklist

import (
	"errors"
	"flywheel/bizerror"
	"flywheel/domain"
	"flywheel/event"
	"flywheel/idgen"
	"flywheel/persistence"
	"flywheel/session"

	"github.com/fundwit/go-commons/types"
	"github.com/jinzhu/gorm"
	"github.com/sony/sonyflake"
)

var (
	checkitemIdWorker = sonyflake.NewSonyflake(sonyflake.Settings{})

	CreateCheckItemFunc     = CreateCheckItem
	ListWorkCheckItemsFunc  = ListWorkCheckItems
	UpdateCheckItemFunc     = UpdateCheckItem
	DeleteCheckItemFunc     = DeleteCheckItem
	CleanWorkCheckItemsFunc = CleanWorkCheckItems

	CleanWorkCheckItemsDirectlyFunc = CleanWorkCheckItemsDirectly
	InnerListWorksCheckItemsFunc    = InnerListWorksCheckItems
)

type CheckItemState string

type CheckItem struct {
	ID     types.ID `json:"id" gorm:"primary_key"`
	Name   string   `json:"name"`
	WorkId types.ID `json:"workId"`

	Done bool `json:"done"`

	CreateTime types.Timestamp `json:"createTime" sql:"type:DATETIME(6) NOT NULL"`
}

type CheckItemCreation struct {
	Name   string   `json:"name" binding:"required"`
	WorkId types.ID `json:"workId" binding:"required"`
}

type CheckItemUpdate struct {
	Name string `json:"name"`
	Done *bool  `json:"done"`
}

func CreateCheckItem(req CheckItemCreation, c *session.Session) (*CheckItem, error) {
	var r *CheckItem
	var ev *event.EventRecord
	txErr := persistence.ActiveDataSourceManager.GormDB(c.Context).Transaction(func(tx *gorm.DB) error {
		// check permission against to work
		w, err := findWorkAndCheckPerms(tx, req.WorkId, c)
		if err != nil {
			return err
		}
		i := CheckItem{
			ID:         idgen.NextID(checkitemIdWorker),
			Name:       req.Name,
			WorkId:     w.ID,
			CreateTime: types.CurrentTimestamp(),
			Done:       false,
		}
		if err := tx.Save(&i).Error; err != nil {
			return err
		}
		r = &i

		ev, err = event.CreateEvent("WORK", w.ID, w.Identifier, event.EventCategoryExtensionUpdated,
			[]event.UpdatedProperty{{
				PropertyName: "Checklist", PropertyDesc: "Checklist",
				NewValue: req.Name, NewValueDesc: req.Name,
			}}, nil, &c.Identity, i.CreateTime, tx)
		if err != nil {
			return err
		}

		return nil
	})

	if txErr != nil {
		return nil, txErr
	}

	if event.InvokeHandlersFunc != nil {
		event.InvokeHandlersFunc(ev)
	}

	return r, nil
}

func ListWorkCheckItems(workId types.ID, c *session.Session) ([]CheckItem, error) {
	var r []CheckItem
	txErr := persistence.ActiveDataSourceManager.GormDB(c.Context).Transaction(func(tx *gorm.DB) error {
		w, err := findWorkAndCheckPerms(tx, workId, c)
		if err != nil {
			return err
		}
		if err := tx.Where("work_id = ?", w.ID).Find(&r).Error; err != nil {
			return err
		}
		return nil
	})
	if txErr != nil {
		return nil, txErr
	}
	return r, nil
}

func InnerListWorksCheckItems(workIds []types.ID, tx *gorm.DB) ([]CheckItem, error) {
	var r []CheckItem

	if len(workIds) == 0 {
		return r, nil
	}
	if err := tx.Where("work_id IN (?)", workIds).Find(&r).Error; err != nil {
		return nil, err
	}
	return r, nil
}

func UpdateCheckItem(id types.ID, req CheckItemUpdate, c *session.Session) error {
	var ev *event.EventRecord
	txErr := persistence.ActiveDataSourceManager.GormDB(c.Context).Transaction(func(tx *gorm.DB) error {
		// find checkitem
		ci := CheckItem{}
		if err := tx.Where("id = ?", id).First(&ci).Error; err != nil {
			return err
		}
		// check permission against to work
		w, err := findWorkAndCheckPerms(tx, ci.WorkId, c)
		if err != nil {
			return err
		}

		changes := map[string]interface{}{}
		if req.Name != "" && req.Name != ci.Name {
			changes["name"] = req.Name
		}
		if req.Done != nil && *req.Done != ci.Done {
			changes["done"] = *req.Done
		}
		if len(changes) == 0 {
			return nil
		}
		if err := tx.Model(&CheckItem{}).Where("id = ?", ci.ID).Updates(changes).Error; err != nil {
			return err
		}

		ev, err = event.CreateEvent("WORK", w.ID, w.Identifier, event.EventCategoryExtensionUpdated,
			[]event.UpdatedProperty{{
				PropertyName: "Checklist", PropertyDesc: "Checklist",
			}}, nil, &c.Identity, types.CurrentTimestamp(), tx)
		if err != nil {
			return err
		}

		return nil
	})
	if txErr != nil {
		return txErr
	}

	if event.InvokeHandlersFunc != nil && ev != nil {
		event.InvokeHandlersFunc(ev)
	}

	return nil
}

func DeleteCheckItem(id types.ID, c *session.Session) error {
	var ev *event.EventRecord
	txErr := persistence.ActiveDataSourceManager.GormDB(c.Context).Transaction(func(tx *gorm.DB) error {
		// find checkitem
		ci := CheckItem{}
		if err := tx.Where("id = ?", id).First(&ci).Error; errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		} else if err != nil {
			return err
		}
		// check permission against to work
		w, err := findWorkAndCheckPerms(tx, ci.WorkId, c)
		if err != nil {
			return err
		}

		if err := tx.Delete(&CheckItem{}, "id = ?", id).Error; err != nil {
			return err
		}

		ev, err = event.CreateEvent("WORK", w.ID, w.Identifier, event.EventCategoryExtensionUpdated,
			[]event.UpdatedProperty{{
				PropertyName: "Checklist", PropertyDesc: "Checklist",
				OldValue: ci.Name, OldValueDesc: ci.Name,
			}}, nil, &c.Identity, types.CurrentTimestamp(), tx)
		if err != nil {
			return err
		}

		return nil
	})
	if txErr != nil {
		return txErr
	}

	if event.InvokeHandlersFunc != nil && ev != nil {
		event.InvokeHandlersFunc(ev)
	}

	return nil
}

func CleanWorkCheckItems(workId types.ID, c *session.Session) error {
	var ev *event.EventRecord
	txErr := persistence.ActiveDataSourceManager.GormDB(c.Context).Transaction(func(tx *gorm.DB) error {
		// check permission against to work
		w, err := findWorkAndCheckPerms(tx, workId, c)
		if err != nil {
			return err
		}
		if err := CleanWorkCheckItemsDirectly(workId, tx); err != nil {
			return err
		}

		ev, err = event.CreateEvent("WORK", w.ID, w.Identifier, event.EventCategoryExtensionUpdated,
			[]event.UpdatedProperty{{
				PropertyName: "Checklist", PropertyDesc: "Checklist",
			}}, nil, &c.Identity, types.CurrentTimestamp(), tx)
		if err != nil {
			return err
		}

		return nil
	})
	if txErr != nil {
		return txErr
	}
	if event.InvokeHandlersFunc != nil {
		event.InvokeHandlersFunc(ev)
	}
	return nil
}

func CleanWorkCheckItemsDirectly(workId types.ID, tx *gorm.DB) error {
	if err := tx.Delete(&CheckItem{}, "work_id = ?", workId).Error; err != nil {
		return err
	}
	return nil
}

func findWorkAndCheckPerms(db *gorm.DB, id types.ID, s *session.Session) (*domain.Work, error) {
	var work domain.Work
	if err := db.Where("id = ?", id).First(&work).Error; err != nil {
		return nil, err
	}

	if s == nil || !s.Perms.HasProjectViewPerm(work.ProjectID) {
		return nil, bizerror.ErrForbidden
	}
	return &work, nil
}
