package indices

import (
	"errors"
	"flywheel/account"
	"flywheel/authority"
	"flywheel/bizerror"
	"flywheel/domain/work"
	"flywheel/es"
	"flywheel/event"
	"flywheel/indices/indexlog"
	"flywheel/session"
	"fmt"
	"sync"

	"github.com/jinzhu/gorm"
	"github.com/sirupsen/logrus"
)

var (
	WorkIndexEventHandlerName = "workIndexr"
	indexRobot                = &session.Session{
		Identity: session.Identity{ID: 10, Name: "index-robot"},
		Perms:    authority.Permissions{account.SystemViewPermission.ID},
	}
	anonymousRecoveryInvoker = &session.Session{
		Identity: session.Identity{ID: 11, Name: "anonymous-invoker"},
		Perms:    authority.Permissions{account.SystemRecoveryPermission.ID},
	}

	lock    sync.Mutex
	running bool

	IndicesFullSyncFunc         = IndicesFullSync
	ScheduleNewSyncRunFunc      = ScheduleNewSyncRun
	IndexlogRecoveryRoutineFunc = IndexlogRecoveryRoutine
)

func ScheduleNewSyncRun(sec *session.Session) (bool, error) {
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

func IndexlogRecoveryRoutine(sec *session.Session) (err error) {
	if !sec.Perms.HasRole(account.SystemRecoveryPermission.ID) && !sec.Perms.HasRole(account.SystemAdminPermission.ID) {
		return bizerror.ErrForbidden
	}

	defer func() {
		if ret := recover(); ret != nil {
			e, ok := ret.(error)
			if ok {
				err = e
			} else {
				err = fmt.Errorf("error on index log recovery routine: %v", ret)
			}
		}
	}()

	page := 1
	for {
		indexLogs, err := indexlog.LoadPendingIndexLogFunc(page, SyncBatchSize)
		if err != nil {
			logrus.Warnf("pending index log sync: error on retrive index logs(page = %d, pageSize = %d): %v", page, SyncBatchSize, err)
			page++
			continue
		}

		if len(indexLogs) == 0 {
			logrus.Infof("pending index log sync: there are no more index log to index")
			return nil // loop exit
		}

		workDetails := make([]work.WorkDetail, 0, len(indexLogs))
		for _, w := range indexLogs {
			d, err := work.DetailWorkFunc(w.SourceId.String(), indexRobot)
			if errors.Is(err, gorm.ErrRecordNotFound) {
				if err := indexlog.ObsoleteIndexLogFunc(w.ID); err != nil {
					logrus.Warnf("pending index log sync: failed to obsolete index log %s, %v", w.ID, err)
				}
				continue
			} else if err != nil {
				logrus.Warnf("pending index log sync: failed to detail work %s, %v", w.ID, err)
				continue
			}
			workDetails = append(workDetails, *d)
		}

		// IndexFunc will be invoked
		if err := IndexWorks(workDetails); err != nil {
			logrus.Warnf("indices fully sync: error on index works(page = %d, pageSize = %d): %v", page, SyncBatchSize, err)
		}
		page++
	}
}

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

	if err := indexlog.FinishIndexLogFunc(e.ID); err != nil {
		return &event.EventHandleResult{
			Message:           fmt.Sprintf("error on finish index log %d of work %s, %v", e.ID, e.Event.SourceDesc, err),
			HandlerIdentifier: WorkIndexEventHandlerName,
		}
	}
	return &event.EventHandleResult{Success: true, HandlerIdentifier: WorkIndexEventHandlerName}
}
