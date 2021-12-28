package work

import (
	"context"
	"errors"
	"flywheel/bizerror"
	"flywheel/domain"
	"flywheel/domain/flow"
	"flywheel/domain/label"
	"flywheel/domain/namespace"
	"flywheel/domain/state"
	"flywheel/domain/work/checklist"
	"flywheel/event"
	"flywheel/idgen"
	"flywheel/persistence"
	"flywheel/session"
	"strconv"

	"github.com/fundwit/go-commons/types"
	"github.com/jinzhu/gorm"
	"github.com/sony/sonyflake"
)

var (
	workIdWorker = sonyflake.NewSonyflake(sonyflake.Settings{})

	CreateWorkFunc = CreateWork
	UpdateWorkFunc = UpdateWork
	DetailWorkFunc = DetailWork

	LoadWorksFunc              = LoadWorks
	ArchiveWorksFunc           = ArchiveWorks
	DeleteWorkFunc             = DeleteWork
	UpdateStateRangeOrdersFunc = UpdateStateRangeOrders
	QueryLabelBriefsOfWorkFunc = QueryLabelBriefsOfWork

	ExtendWorksFunc = ExtendWorks
)

type WorkDetail struct {
	domain.Work

	State     state.State           `json:"state"`
	Type      *domain.Workflow      `json:"type"`
	Labels    []label.LabelBrief    `json:"labels"`
	CheckList []checklist.CheckItem `json:"checklist"`
}

func CreateWork(c *domain.WorkCreation, s *session.Session) (*WorkDetail, error) {
	if !s.Perms.HasAnyProjectRole(c.ProjectID) {
		return nil, bizerror.ErrForbidden
	}

	var workDetail *WorkDetail
	var ev *event.EventRecord

	err1 := persistence.ActiveDataSourceManager.GormDB(s.Context).Transaction(func(tx *gorm.DB) error {
		workflowDetail, err := flow.DetailWorkflow(c.FlowID, s)
		if err != nil {
			return err
		}
		initialState, found := workflowDetail.StateMachine.FindState(c.InitialStateName)
		if !found {
			return bizerror.ErrUnknownState
		}

		now := types.CurrentTimestamp()
		workDetail = &WorkDetail{
			Work: domain.Work{
				ID:         idgen.NextID(workIdWorker),
				Name:       c.Name,
				ProjectID:  c.ProjectID,
				CreateTime: now,

				FlowID:         workflowDetail.ID,
				OrderInState:   now.Time().UnixNano() / 1e6, // oldest
				StateName:      initialState.Name,
				StateCategory:  initialState.Category,
				StateBeginTime: now,
			},
			State: initialState,
			Type:  &workflowDetail.Workflow,
		}
		if c.PriorityLevel < 0 { // Highest: -1, lowest： 1
			var highestPriorityWork domain.Work
			err := tx.Model(&domain.Work{}).Where(&domain.Work{ProjectID: c.ProjectID, StateName: initialState.Name}).
				Select("order_in_state").
				Order("order_in_state ASC").Limit(1).First(&highestPriorityWork).Error
			if err == nil {
				workDetail.OrderInState = highestPriorityWork.OrderInState - 1
			} else if !errors.Is(err, gorm.ErrRecordNotFound) {
				return err
			}
		}

		identifier, err := namespace.NextWorkIdentifier(c.ProjectID, tx)
		if err != nil {
			return err
		}
		workDetail.Identifier = identifier

		if err := tx.Create(workDetail.Work).Error; err != nil {
			return err
		}

		initProcessStep := domain.WorkProcessStep{WorkID: workDetail.ID, FlowID: workDetail.FlowID,
			CreatorID: s.Identity.ID, CreatorName: s.Identity.Nickname,
			StateName: workDetail.State.Name, StateCategory: workDetail.State.Category, BeginTime: workDetail.CreateTime}
		if err := tx.Create(initProcessStep).Error; err != nil {
			return err
		}

		ev, err = CreateWorkCreatedEvent(&workDetail.Work, &s.Identity, workDetail.CreateTime, tx)
		if err != nil {
			return err
		}

		return nil
	})
	if err1 != nil {
		return nil, err1
	}

	if event.InvokeHandlersFunc != nil {
		event.InvokeHandlersFunc(ev)
	}

	return workDetail, nil
}

func DetailWork(identifier string, s *session.Session) (*WorkDetail, error) {
	id, _ := types.ParseID(identifier)
	w := domain.Work{}
	if err := persistence.ActiveDataSourceManager.GormDB(s.Context).Where("id = ? OR identifier LIKE ?", id, identifier).First(&w).Error; err != nil {
		return nil, err
	}

	if !s.Perms.HasProjectViewPerm(w.ProjectID) {
		return nil, bizerror.ErrForbidden
	}

	ws, err := ExtendWorks([]WorkDetail{{Work: w}}, s)
	if err != nil {
		return nil, err
	}

	wd := &ws[0]
	if err := extendWorkIndexedInfo(wd, s); err != nil {
		return nil, err
	}

	return wd, nil
}

func extendWorkIndexedInfo(w *WorkDetail, c *session.Session) error {
	// append checklist
	cl, err := checklist.ListCheckItemsFunc(w.ID, c)
	if err != nil {
		return err
	}
	w.CheckList = cl
	return nil
}

// ExtendWorks append Work.state type and labels
func ExtendWorks(workDetails []WorkDetail, s *session.Session) ([]WorkDetail, error) {
	var err error
	c := len(workDetails)

	workflowMap := map[types.ID]*domain.WorkflowDetail{}
	var workIds []types.ID
	for i := 0; i < c; i++ {
		w := workDetails[i] // w is a copy, not a reference
		workflowMap[w.FlowID] = nil
		workIds = append(workIds, w.ID)
	}

	// load workflow
	workflowCache := map[types.ID]*domain.WorkflowDetail{}
	for flowId, workflow := range workflowMap {
		if workflow == nil {
			workflow, err = flow.DetailWorkflowFunc(flowId, s)
			if err != nil {
				return nil, err
			}
			workflowCache[flowId] = workflow
			workflowMap[flowId] = workflow
		}
	}

	// load labels
	wls, err := QueryLabelBriefsOfWorkFunc(workIds, s)
	if err != nil {
		return nil, err
	}

	for i := 0; i < c; i++ {
		w := workDetails[i] // w is a copy, not a reference

		// using w.FlowID to append workflow, state, stateCategory
		workflow := workflowCache[w.FlowID]
		if workflow != nil {
			w.Type = &workflow.Workflow
			stateFound, found := workflow.FindState(w.StateName)
			if !found {
				return nil, bizerror.ErrStateInvalid
			}
			w.State = stateFound
			w.StateCategory = stateFound.Category
		}

		// append labels
		var ls []label.LabelBrief
		for _, l := range wls {
			if l.WorkID == w.ID {
				ls = append(ls, label.LabelBrief{ID: l.LabelID, Name: l.LabelName, ThemeColor: l.LabelThemeColor})
			}
		}
		w.Labels = ls

		// at last, put the copy w into slice
		workDetails[i] = w
	}
	return workDetails, nil
}

func ArchiveWorks(ids []types.ID, s *session.Session) error {
	var events []*event.EventRecord
	now := types.CurrentTimestamp()
	err1 := persistence.ActiveDataSourceManager.GormDB(s.Context).Transaction(func(tx *gorm.DB) error {
		for _, id := range ids {
			work, err := findWorkAndCheckPerms(tx, id, s)
			if err != nil {
				return err
			}
			if work.StateCategory != state.Done && work.StateCategory != state.Rejected {
				return bizerror.ErrStateCategoryInvalid
			}
			if !work.ArchiveTime.IsZero() {
				return nil
			}

			ev, err := CreateWorkPropertyUpdatedEvent(work,
				[]event.UpdatedProperty{{
					PropertyName: "ArchiveTime", PropertyDesc: "ArchiveTime",
					OldValue: work.ArchiveTime.String(), OldValueDesc: work.ArchiveTime.String(),
					NewValue: now.String(), NewValueDesc: now.String(),
				}},
				&s.Identity, now, tx)
			if err != nil {
				return err
			}
			events = append(events, ev)

			db := tx.Model(&domain.Work{ID: id}).Updates(&domain.Work{ArchiveTime: now})
			if err := db.Error; err != nil {
				return err
			}
		}
		return nil
	})
	if err1 != nil {
		return err1
	}

	if event.InvokeHandlersFunc != nil {
		for _, ev := range events {
			event.InvokeHandlersFunc(ev)
		}
	}

	return nil
}

func findWorkAndCheckPerms(db *gorm.DB, id types.ID, s *session.Session) (*domain.Work, error) {
	var work domain.Work
	if err := db.Where("id = ?", id).First(&work).Error; err != nil {
		return nil, err
	}
	if s == nil || !s.Perms.HasAnyProjectRole(work.ProjectID) {
		return nil, bizerror.ErrForbidden
	}
	return &work, nil
}

func UpdateWork(id types.ID, u *domain.WorkUpdating, s *session.Session) (*domain.Work, error) {
	var updatedWork domain.Work
	var ev *event.EventRecord
	err1 := persistence.ActiveDataSourceManager.GormDB(s.Context).Transaction(func(tx *gorm.DB) error {
		originWork, err := findWorkAndCheckPerms(tx, id, s)
		if err != nil {
			return err
		}

		if !originWork.ArchiveTime.IsZero() {
			return bizerror.ErrArchiveStatusInvalid
		}

		db := tx.Model(&domain.Work{}).Where(&domain.Work{ID: id}).Update(u)
		if err := db.Error; err != nil {
			return err
		}
		if db.RowsAffected != 1 {
			return errors.New("expected affected row is 1, but actual is " + strconv.FormatInt(db.RowsAffected, 10))
		}

		ev, err = CreateWorkPropertyUpdatedEvent(originWork,
			[]event.UpdatedProperty{{
				PropertyName: "Name", PropertyDesc: "Name",
				OldValue: originWork.Name, OldValueDesc: originWork.Name,
				NewValue: u.Name, NewValueDesc: u.Name,
			}},
			&s.Identity, types.CurrentTimestamp(), tx)
		if err != nil {
			return err
		}

		if err := tx.Where(&domain.Work{ID: id}).First(&updatedWork).Error; err != nil {
			return err
		}
		return nil
	})
	if err1 != nil {
		return nil, err1
	}

	if event.InvokeHandlersFunc != nil {
		event.InvokeHandlersFunc(ev)
	}

	return &updatedWork, nil
}

func DeleteWork(id types.ID, s *session.Session) error {
	var ev *event.EventRecord
	err1 := persistence.ActiveDataSourceManager.GormDB(s.Context).Transaction(func(tx *gorm.DB) error {
		_, err := findWorkAndCheckPerms(tx, id, s)
		if err != nil {
			return err
		}
		work := domain.Work{ID: id}
		err = tx.Model(&work).First(&work).Error
		if err == nil {
			ev, err = CreateWorkDeletedEvent(&work, &s.Identity, types.CurrentTimestamp(), tx)
			if err != nil {
				return err
			}
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}

		if err := tx.Delete(domain.Work{}, "id = ?", id).Error; err != nil {
			return err
		}
		if err := tx.Delete(domain.WorkProcessStep{}, "work_id = ?", id).Error; err != nil {
			return err
		}
		if err := checklist.CleanWorkCheckItemsDirectlyFunc(id, tx); err != nil {
			return err
		}
		if err := ClearWorkLabelRelationsFunc(work.ID, tx); err != nil {
			return err
		}
		return nil
	})
	if err1 != nil {
		return err1
	}
	if event.InvokeHandlersFunc != nil {
		event.InvokeHandlersFunc(ev)
	}
	return err1
}

func UpdateStateRangeOrders(wantedOrders *[]domain.WorkOrderRangeUpdating, s *session.Session) error {
	if wantedOrders == nil || len(*wantedOrders) == 0 {
		return nil
	}

	var events []*event.EventRecord
	err1 := persistence.ActiveDataSourceManager.GormDB(s.Context).Transaction(func(tx *gorm.DB) error {
		for _, orderUpdating := range *wantedOrders {
			// TODO transition issues
			originWork, err := findWorkAndCheckPerms(tx, orderUpdating.ID, s)
			if err != nil {
				return err
			}
			db := tx.Model(&domain.Work{}).Where(&domain.Work{ID: orderUpdating.ID, OrderInState: orderUpdating.OldOlder}).
				Update(&domain.Work{OrderInState: orderUpdating.NewOlder})
			if err := db.Error; err != nil {
				return err
			}
			if db.RowsAffected != 1 {
				return errors.New("expected affected row is 1, but actual is " + strconv.FormatInt(db.RowsAffected, 10))
			}
			ev, err := CreateWorkPropertyUpdatedEvent(originWork,
				[]event.UpdatedProperty{{
					PropertyName: "OrderInState", PropertyDesc: "OrderInState",
					OldValue: strconv.FormatInt(originWork.OrderInState, 10), OldValueDesc: strconv.FormatInt(originWork.OrderInState, 10),
					NewValue: strconv.FormatInt(orderUpdating.NewOlder, 10), NewValueDesc: strconv.FormatInt(orderUpdating.NewOlder, 10),
				}},
				&s.Identity, types.CurrentTimestamp(), tx)
			if err != nil {
				return err
			}
			events = append(events, ev)
		}
		return nil
	})
	if err1 != nil {
		return err1
	}
	if event.InvokeHandlersFunc != nil {
		for _, ev := range events {
			event.InvokeHandlersFunc(ev)
		}
	}
	return nil
}

func LoadWorks(page, size int) ([]domain.Work, error) {
	works := []domain.Work{}
	db := persistence.ActiveDataSourceManager.GormDB(context.Background())
	offset := (page - 1) * size
	if offset < 0 {
		offset = 0
	}
	if err := db.Order("ID ASC").Offset(offset).Limit(size).Find(&works).Error; err != nil {
		return nil, err
	}
	return works, nil
}
