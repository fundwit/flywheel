package indices

import (
	"flywheel/account"
	"flywheel/authority"
	"flywheel/bizerror"
	"flywheel/domain/work"
	"flywheel/es"
	"flywheel/event"
	"flywheel/session"
	"fmt"
	"sync"

	"github.com/sirupsen/logrus"
)

var (
	WorkIndexEventHandlerName = "workIndexr"
	indexRobot                = &session.Context{
		Identity: session.Identity{ID: 10, Name: "index-robot"},
		Perms:    authority.Permissions{account.SystemViewPermission.ID},
	}

	lock    sync.Mutex
	running bool

	IndicesFullSyncFunc    = IndicesFullSync
	ScheduleNewSyncRunFunc = ScheduleNewSyncRun
)

func ScheduleNewSyncRun(sec *session.Context) (bool, error) {
	if !sec.Perms.HasRole(account.SystemAdminPermission.ID) {
		return false, bizerror.ErrForbidden
	}

	lock.Lock()
	if running {
		lock.Unlock()
		return false, nil
	}
	running = true
	lock.Unlock()

	waitRunning := sync.WaitGroup{}
	waitRunning.Add(1)
	go func() {
		waitRunning.Done()
		defer func() {
			lock.Lock()
			running = false
			lock.Unlock()
		}()
		IndicesFullSyncFunc()
	}()
	waitRunning.Wait()
	return true, nil
}

var (
	SyncBatchSize = 500
)

func IndicesFullSync() (err error) {
	defer func() {
		if ret := recover(); ret != nil {
			e, ok := ret.(error)
			if ok {
				err = e
			} else {
				err = fmt.Errorf("error on indices full sync: %v", ret)
			}
		}
	}()

	page := 1
	for {
		works, err := work.LoadWorksFunc(page, SyncBatchSize)
		if err != nil {
			logrus.Warnf("indices fully sync: error on retrive works(page = %d, pageSize = %d): %v", page, SyncBatchSize, err)
			page++
			continue
		}

		if len(works) == 0 {
			logrus.Infof("indices fully sync: there are no more work to index")
			return nil // loop exit
		}

		workDetails := make([]work.WorkDetail, 0, len(works))
		for _, w := range works {
			workDetails = append(workDetails, work.WorkDetail{Work: w})
		}

		details, err := work.ExtendWorksFunc(workDetails, indexRobot)
		if err != nil {
			logrus.Warnf("indices fully sync: error on detail works(page = %d, pageSize = %d): %v", page, SyncBatchSize, err)
			page++
			continue
		}

		// IndexFunc will be invoked
		if err := IndexWorks(details); err != nil {
			logrus.Warnf("indices fully sync: error on index works(page = %d, pageSize = %d): %v", page, SyncBatchSize, err)
		}
		page++
	}
}

func IndexWorkEventHandle(e *event.EventRecord) *event.EventHandleResult {
	if e.SourceType != "WORK" {
		return nil
	}

	if e.EventCategory == event.EventCategoryDeleted {
		err := es.DeleteDocumentByIdFunc(WorkIndexName, e.Event.SourceId)
		if err != nil {
			return &event.EventHandleResult{
				Message:           fmt.Sprintf("delete work index %d, %v", e.Event.SourceId, err),
				HandlerIdentifier: WorkIndexEventHandlerName,
			}
		}
		return &event.EventHandleResult{Success: true, HandlerIdentifier: WorkIndexEventHandlerName}
	} else {
		w, err := work.DetailWorkFunc(e.Event.SourceId.String(), indexRobot)
		if err != nil {
			return &event.EventHandleResult{
				Message:           fmt.Sprintf("detail work %d which will be indexed, %v", e.Event.SourceId, err),
				HandlerIdentifier: WorkIndexEventHandlerName,
			}
		}
		// IndexWorks will invoke es.IndexFunc
		if err := IndexWorks([]work.WorkDetail{*w}); err != nil {
			return &event.EventHandleResult{
				Message:           fmt.Sprintf("index work %d, %v", e.Event.SourceId, err),
				HandlerIdentifier: WorkIndexEventHandlerName,
			}
		}
	}
	return &event.EventHandleResult{Success: true, HandlerIdentifier: WorkIndexEventHandlerName}
}
