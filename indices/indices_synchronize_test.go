package indices_test

import (
	"errors"
	"flywheel/account"
	"flywheel/authority"
	"flywheel/bizerror"
	"flywheel/client/es"
	"flywheel/domain"
	"flywheel/domain/state"
	"flywheel/domain/work"
	"flywheel/domain/work/checklist"
	"flywheel/event"
	"flywheel/indices"
	"flywheel/indices/indexlog"
	"flywheel/session"
	"testing"
	"time"

	"github.com/fundwit/go-commons/types"
	"github.com/jinzhu/gorm"
	. "github.com/onsi/gomega"
)

func TestScheduleNewSyncRun(t *testing.T) {
	RegisterTestingT(t)

	t.Run("only system admin can schedule sync run", func(t *testing.T) {
		sec := session.Session{Perms: authority.Permissions{account.SystemViewPermission.ID}}
		success, err := indices.ScheduleNewSyncRun(&sec)
		Expect(err).To(Equal(bizerror.ErrForbidden))
		Expect(success).To(BeFalse())
	})

	t.Run("schedule sync run channel should works", func(t *testing.T) {
		indices.IndicesFullSyncFunc = func() error {
			time.Sleep(100 * time.Millisecond)
			return nil
		}

		sec := session.Session{Perms: authority.Permissions{account.SystemAdminPermission.ID}}
		success, err := indices.ScheduleNewSyncRun(&sec)
		Expect(err).To(BeNil())
		Expect(success).To(BeTrue())

		success, err = indices.ScheduleNewSyncRun(&sec)
		Expect(err).To(BeNil())
		Expect(success).To(BeFalse())

		time.Sleep(200 * time.Millisecond)

		success, err = indices.ScheduleNewSyncRun(&sec)
		Expect(err).To(BeNil())
		Expect(success).To(BeTrue())
	})
}

func TestIndexWorkEventHandle(t *testing.T) {
	RegisterTestingT(t)

	t.Run("only accept event of Work", func(t *testing.T) {
		Expect(indices.IndexWorkEventHandle(&event.EventRecord{Event: event.Event{SourceType: "NOT_WORK"}})).To(BeNil())
	})

	t.Run("work delete event handle success", func(t *testing.T) {
		es.DeleteDocumentByIdFunc = func(index string, id types.ID, s *session.Session) error {
			return nil
		}
		indexlog.FinishIndexLogFunc = func(id types.ID) error {
			return nil
		}
		ev := event.EventRecord{Event: event.Event{SourceType: "WORK", SourceId: 100, EventCategory: event.EventCategoryDeleted}}

		expectedResult := event.EventHandleResult{Success: true, HandlerIdentifier: indices.WorkIndexEventHandlerName}
		Expect(*indices.IndexWorkEventHandle(&ev)).To(Equal(expectedResult))
	})
	t.Run("work delete event handle failed", func(t *testing.T) {
		es.DeleteDocumentByIdFunc = func(index string, id types.ID, s *session.Session) error {
			return errors.New("error on delete document")
		}
		ev := event.EventRecord{Event: event.Event{SourceType: "WORK", SourceId: 100, EventCategory: event.EventCategoryDeleted}}

		expectedResult := event.EventHandleResult{
			Success:           false,
			HandlerIdentifier: indices.WorkIndexEventHandlerName,
			Message:           "delete work index 100, error on delete document",
		}
		Expect(*indices.IndexWorkEventHandle(&ev)).To(Equal(expectedResult))
	})

	t.Run("work create or update event handle success", func(t *testing.T) {
		es.IndexFunc = func(index string, id types.ID, doc interface{}, s *session.Session) error {
			return nil
		}
		work.DetailWorkFunc = func(identifier string, s *session.Session) (*work.WorkDetail, error) {
			return &work.WorkDetail{}, nil
		}
		var finishedIndexLogId types.ID
		indexlog.FinishIndexLogFunc = func(id types.ID) error {
			finishedIndexLogId = id
			return nil
		}
		ev := event.EventRecord{ID: 123, Event: event.Event{SourceType: "WORK", SourceId: 100, EventCategory: event.EventCategoryCreated}}

		expectedResult := event.EventHandleResult{Success: true, HandlerIdentifier: indices.WorkIndexEventHandlerName}
		Expect(*indices.IndexWorkEventHandle(&ev)).To(Equal(expectedResult))
		Expect(finishedIndexLogId).To(Equal(types.ID(123)))
	})

	t.Run("failed in detail work progress for work creation event or work updating event", func(t *testing.T) {
		es.IndexFunc = func(index string, id types.ID, doc interface{}, s *session.Session) error {
			return nil
		}
		work.DetailWorkFunc = func(identifier string, s *session.Session) (*work.WorkDetail, error) {
			return nil, errors.New("error on detail work")
		}
		ev := event.EventRecord{Event: event.Event{SourceType: "WORK", SourceId: 100, EventCategory: event.EventCategoryCreated}}

		expectedResult := event.EventHandleResult{
			Success:           false,
			HandlerIdentifier: indices.WorkIndexEventHandlerName,
			Message:           "detail work 100 which will be indexed, error on detail work",
		}
		Expect(*indices.IndexWorkEventHandle(&ev)).To(Equal(expectedResult))
	})

	t.Run("failed in index progress for work creation event or work updating event", func(t *testing.T) {
		es.IndexFunc = func(index string, id types.ID, doc interface{}, s *session.Session) error {
			return errors.New("error on index document")
		}
		work.DetailWorkFunc = func(identifier string, s *session.Session) (*work.WorkDetail, error) {
			id, err := types.ParseID(identifier)
			if err != nil {
				return nil, err
			}
			return &work.WorkDetail{Work: domain.Work{ID: id}}, nil
		}
		ev := event.EventRecord{Event: event.Event{SourceType: "WORK", SourceId: 100, EventCategory: event.EventCategoryCreated}}

		expectedResult := event.EventHandleResult{
			Success:           false,
			HandlerIdentifier: indices.WorkIndexEventHandlerName,
			Message:           "index work 100, map[100:error on index document]",
		}
		Expect(*indices.IndexWorkEventHandle(&ev)).To(Equal(expectedResult))
	})

	t.Run("failed in finish index log for work creation event or work updating event", func(t *testing.T) {
		es.IndexFunc = func(index string, id types.ID, doc interface{}, s *session.Session) error {
			return nil
		}
		work.DetailWorkFunc = func(identifier string, s *session.Session) (*work.WorkDetail, error) {
			id, err := types.ParseID(identifier)
			if err != nil {
				return nil, err
			}
			return &work.WorkDetail{Work: domain.Work{ID: id}}, nil
		}
		indexlog.FinishIndexLogFunc = func(id types.ID) error {
			return errors.New("error on finish index log")
		}
		ev := event.EventRecord{ID: 100, Event: event.Event{
			SourceType: "WORK", SourceDesc: "work1000", SourceId: 1000, EventCategory: event.EventCategoryCreated}}

		expectedResult := event.EventHandleResult{
			Success:           false,
			HandlerIdentifier: indices.WorkIndexEventHandlerName,
			Message:           "error on finish index log " + ev.ID.String() + " of work work1000, error on finish index log",
		}
		Expect(*indices.IndexWorkEventHandle(&ev)).To(Equal(expectedResult))
	})
}

func TestIndicesFullSync(t *testing.T) {
	RegisterTestingT(t)

	type indexResult struct {
		index string
		id    types.ID
		doc   interface{}
	}

	t.Run("should recover panic to error", func(t *testing.T) {
		raisedErr := errors.New("error on load works")
		work.InnerLoadWorksFunc = func(page, size int) ([]domain.Work, error) {
			panic(raisedErr)
		}
		err := indices.IndicesFullSync()
		Expect(err).To(Equal(raisedErr))

		work.InnerLoadWorksFunc = func(page, size int) ([]domain.Work, error) {
			panic("error on load works")
		}
		err = indices.IndicesFullSync()
		Expect(err).To(Equal(errors.New("error on indices full sync: error on load works")))
	})

	t.Run("should be able to index all works", func(t *testing.T) {
		docs := []indexResult{}

		es.IndexFunc = func(index string, id types.ID, doc interface{}, s *session.Session) error {
			docs = append(docs, indexResult{index, id, doc})
			return nil
		}
		total := 5
		work.InnerLoadWorksFunc = func(page, size int) ([]domain.Work, error) {
			works := []domain.Work{}
			cur := size * (page - 1)
			n := 0
			for cur < total && n < size {
				works = append(works, domain.Work{ID: types.ID(cur + 1)})
				cur++
				n++
			}
			return works, nil
		}
		work.ExtendWorksFunc = func(details []work.WorkDetail, s *session.Session) ([]work.WorkDetail, error) {
			c := len(details)
			for i := 0; i < c; i++ {
				details[i].State = state.State{Name: "test"}
			}
			return details, nil
		}
		work.InnerAppendChecklistsFunc = func(details []work.WorkDetail, s *session.Session) error {
			c := len(details)
			for i := 0; i < c; i++ {
				details[i].CheckList = []checklist.CheckItem{{Name: "checkitem"}}
			}
			return nil
		}

		indices.SyncBatchSize = 2
		Expect(indices.IndicesFullSync()).To(BeNil())

		wantedDocs := []indexResult{}
		for i := 0; i < total; i++ {
			d := work.WorkDetail{Work: domain.Work{ID: types.ID(i + 1)}, State: state.State{Name: "test"},
				CheckList: []checklist.CheckItem{{Name: "checkitem"}}}
			wantedDocs = append(wantedDocs, indexResult{indices.WorkIndexName, types.ID(i + 1),
				indices.WorkDocument{d},
			})
		}
		Expect(len(docs)).To(Equal(5))
		Expect(docs).To(Equal(wantedDocs))
	})

	t.Run("should continue to next batch when failed in load works", func(t *testing.T) {
		docs := []indexResult{}

		es.IndexFunc = func(index string, id types.ID, doc interface{}, s *session.Session) error {
			docs = append(docs, indexResult{index, id, doc})
			return nil
		}
		total := 5
		work.InnerLoadWorksFunc = func(page, size int) ([]domain.Work, error) {
			if page == 2 {
				return nil, errors.New("error on load works")
			}
			works := []domain.Work{}
			cur := size * (page - 1)
			n := 0
			for cur < total && n < size {
				works = append(works, domain.Work{ID: types.ID(cur + 1)})
				cur++
				n++
			}
			return works, nil
		}
		work.ExtendWorksFunc = func(details []work.WorkDetail, s *session.Session) ([]work.WorkDetail, error) {
			c := len(details)
			for i := 0; i < c; i++ {
				details[i].State = state.State{Name: "test"}
			}
			return details, nil
		}
		work.InnerAppendChecklistsFunc = func(details []work.WorkDetail, s *session.Session) error {
			c := len(details)
			for i := 0; i < c; i++ {
				details[i].CheckList = []checklist.CheckItem{{Name: "checkitem"}}
			}
			return nil
		}

		indices.SyncBatchSize = 2
		Expect(indices.IndicesFullSync()).To(BeNil())

		wantedDocs := []indexResult{}
		for i := 0; i < total; i++ {
			if i/indices.SyncBatchSize == 1 {
				continue
			}
			d := work.WorkDetail{Work: domain.Work{ID: types.ID(i + 1)}, State: state.State{Name: "test"},
				CheckList: []checklist.CheckItem{{Name: "checkitem"}}}
			wantedDocs = append(wantedDocs, indexResult{indices.WorkIndexName, types.ID(i + 1),
				indices.WorkDocument{d},
			})
		}
		Expect(len(docs)).To(Equal(3))
		Expect(docs).To(Equal(wantedDocs))
	})

	t.Run("should continue to next batch when failed in append checklists", func(t *testing.T) {
		docs := []indexResult{}

		es.IndexFunc = func(index string, id types.ID, doc interface{}, s *session.Session) error {
			docs = append(docs, indexResult{index, id, doc})
			return nil
		}
		total := 5
		work.InnerLoadWorksFunc = func(page, size int) ([]domain.Work, error) {
			works := []domain.Work{}
			cur := size * (page - 1)
			n := 0
			for cur < total && n < size {
				works = append(works, domain.Work{ID: types.ID(cur + 1)})
				cur++
				n++
			}
			return works, nil
		}
		work.InnerAppendChecklistsFunc = func(details []work.WorkDetail, s *session.Session) error {
			c := len(details)
			for i := 0; i < c; i++ {
				if int(details[i].ID-1)/indices.SyncBatchSize == 1 {
					return errors.New("error on append check lists")
				}
				details[i].CheckList = []checklist.CheckItem{{Name: "checkitem"}}
			}
			return nil
		}
		work.ExtendWorksFunc = func(details []work.WorkDetail, s *session.Session) ([]work.WorkDetail, error) {
			c := len(details)
			for i := 0; i < c; i++ {
				details[i].State = state.State{Name: "test"}
			}
			return details, nil
		}

		indices.SyncBatchSize = 2
		Expect(indices.IndicesFullSync()).To(BeNil())

		wantedDocs := []indexResult{}
		for i := 0; i < total; i++ {
			if i/indices.SyncBatchSize == 1 {
				continue
			}
			d := work.WorkDetail{Work: domain.Work{ID: types.ID(i + 1)}, State: state.State{Name: "test"},
				CheckList: []checklist.CheckItem{{Name: "checkitem"}}}
			wantedDocs = append(wantedDocs, indexResult{indices.WorkIndexName, types.ID(i + 1),
				indices.WorkDocument{d},
			})
		}
		Expect(len(docs)).To(Equal(3))
		Expect(docs).To(Equal(wantedDocs))
	})

	t.Run("should continue to next batch when failed in extend work details", func(t *testing.T) {
		docs := []indexResult{}

		es.IndexFunc = func(index string, id types.ID, doc interface{}, s *session.Session) error {
			docs = append(docs, indexResult{index, id, doc})
			return nil
		}
		total := 5
		work.InnerLoadWorksFunc = func(page, size int) ([]domain.Work, error) {
			works := []domain.Work{}
			cur := size * (page - 1)
			n := 0
			for cur < total && n < size {
				works = append(works, domain.Work{ID: types.ID(cur + 1)})
				cur++
				n++
			}
			return works, nil
		}
		work.InnerAppendChecklistsFunc = func(details []work.WorkDetail, s *session.Session) error {
			c := len(details)
			for i := 0; i < c; i++ {
				details[i].CheckList = []checklist.CheckItem{{Name: "checkitem"}}
			}
			return nil
		}
		work.ExtendWorksFunc = func(details []work.WorkDetail, s *session.Session) ([]work.WorkDetail, error) {
			c := len(details)
			for i := 0; i < c; i++ {
				if int(details[i].ID-1)/indices.SyncBatchSize == 1 {
					return nil, errors.New("error on extend work details")
				}
				details[i].State = state.State{Name: "test"}
			}
			return details, nil
		}

		indices.SyncBatchSize = 2
		Expect(indices.IndicesFullSync()).To(BeNil())

		wantedDocs := []indexResult{}
		for i := 0; i < total; i++ {
			if i/indices.SyncBatchSize == 1 {
				continue
			}
			d := work.WorkDetail{Work: domain.Work{ID: types.ID(i + 1)}, State: state.State{Name: "test"},
				CheckList: []checklist.CheckItem{{Name: "checkitem"}}}
			wantedDocs = append(wantedDocs, indexResult{indices.WorkIndexName, types.ID(i + 1),
				indices.WorkDocument{d},
			})
		}
		Expect(len(docs)).To(Equal(3))
		Expect(docs).To(Equal(wantedDocs))
	})

	t.Run("should continue to next batch when failed in index works", func(t *testing.T) {
		docs := []indexResult{}

		es.IndexFunc = func(index string, id types.ID, doc interface{}, s *session.Session) error {
			if int(id-1)/indices.SyncBatchSize == 1 {
				return errors.New("error on load works")
			}
			docs = append(docs, indexResult{index, id, doc})
			return nil
		}
		total := 5
		work.InnerLoadWorksFunc = func(page, size int) ([]domain.Work, error) {
			works := []domain.Work{}
			cur := size * (page - 1)
			n := 0
			for cur < total && n < size {
				works = append(works, domain.Work{ID: types.ID(cur + 1)})
				cur++
				n++
			}
			return works, nil
		}
		work.ExtendWorksFunc = func(details []work.WorkDetail, s *session.Session) ([]work.WorkDetail, error) {
			c := len(details)
			for i := 0; i < c; i++ {
				details[i].State = state.State{Name: "test"}
			}
			return details, nil
		}
		work.InnerAppendChecklistsFunc = func(details []work.WorkDetail, s *session.Session) error {
			c := len(details)
			for i := 0; i < c; i++ {
				details[i].CheckList = []checklist.CheckItem{{Name: "checkitem"}}
			}
			return nil
		}

		indices.SyncBatchSize = 2
		Expect(indices.IndicesFullSync()).To(BeNil())

		wantedDocs := []indexResult{}
		for i := 0; i < total; i++ {
			if i/indices.SyncBatchSize == 1 {
				continue
			}
			d := work.WorkDetail{Work: domain.Work{ID: types.ID(i + 1)}, State: state.State{Name: "test"},
				CheckList: []checklist.CheckItem{{Name: "checkitem"}}}
			wantedDocs = append(wantedDocs, indexResult{indices.WorkIndexName, types.ID(i + 1),
				indices.WorkDocument{d},
			})
		}
		Expect(len(docs)).To(Equal(3))
		Expect(docs).To(Equal(wantedDocs))
	})
}

func TestIndexlogRecoverRoutine(t *testing.T) {
	RegisterTestingT(t)
	c := &session.Session{Perms: authority.Permissions{account.SystemRecoveryPermission.ID}}

	type indexResult struct {
		index string
		id    types.ID
		doc   interface{}
	}

	t.Run("only system recovery permission or can invoke IndexlogRecoveryRoutine", func(t *testing.T) {
		sec := session.Session{Perms: authority.Permissions{account.SystemViewPermission.ID}}
		err := indices.IndexlogRecoveryRoutine(&sec)
		Expect(err).To(Equal(bizerror.ErrForbidden))
	})

	t.Run("should recover panic to error", func(t *testing.T) {
		raisedErr := errors.New("error on load pending index logs")
		indexlog.LoadPendingIndexLogFunc = func(page, size int) ([]indexlog.IndexLogRecord, error) {
			panic(raisedErr)
		}
		err := indices.IndexlogRecoveryRoutine(
			&session.Session{Perms: authority.Permissions{account.SystemAdminPermission.ID}})
		Expect(err).To(Equal(raisedErr))

		indexlog.LoadPendingIndexLogFunc = func(page, size int) ([]indexlog.IndexLogRecord, error) {
			panic("error on load pending index logs")
		}
		err = indices.IndexlogRecoveryRoutine(
			&session.Session{Perms: authority.Permissions{account.SystemAdminPermission.ID}})
		Expect(err).To(Equal(errors.New("error on index log recovery routine: error on load pending index logs")))
	})

	t.Run("should be able to index all pending index log", func(t *testing.T) {
		docs := []indexResult{}

		es.IndexFunc = func(index string, id types.ID, doc interface{}, s *session.Session) error {
			docs = append(docs, indexResult{index, id, doc})
			return nil
		}
		total := 5
		indexlog.LoadPendingIndexLogFunc = func(page, size int) ([]indexlog.IndexLogRecord, error) {
			logs := []indexlog.IndexLogRecord{}
			cur := size * (page - 1)
			n := 0
			for cur < total && n < size {
				logs = append(logs, indexlog.IndexLogRecord{ID: types.ID(cur + 1),
					IndexLog: indexlog.IndexLog{SourceId: types.ID(cur + 1)}})
				cur++
				n++
			}
			return logs, nil
		}
		work.DetailWorkFunc = func(identifier string, s *session.Session) (*work.WorkDetail, error) {
			if identifier == "3" {
				return nil, gorm.ErrRecordNotFound
			}
			id, err := types.ParseID(identifier)
			if err != nil {
				return nil, err
			}
			return &work.WorkDetail{Work: domain.Work{ID: id}, State: state.State{Name: "test"}}, nil
		}
		var obsoleteIndexLogs []types.ID
		indexlog.ObsoleteIndexLogFunc = func(id types.ID) error {
			obsoleteIndexLogs = append(obsoleteIndexLogs, id)
			return nil
		}

		indices.SyncBatchSize = 2
		Expect(indices.IndexlogRecoveryRoutine(c)).To(BeNil())

		wantedDocs := []indexResult{}
		for i := 0; i < total; i++ {
			if i == 2 {
				continue // obsoleted
			}
			d := work.WorkDetail{Work: domain.Work{ID: types.ID(i + 1)}, State: state.State{Name: "test"}}
			wantedDocs = append(wantedDocs, indexResult{indices.WorkIndexName, types.ID(i + 1),
				indices.WorkDocument{d},
			})
		}
		Expect(len(docs)).To(Equal(4))
		Expect(docs).To(Equal(wantedDocs))
		Expect(obsoleteIndexLogs).To(Equal([]types.ID{3}))
	})

	t.Run("should continue to next batch when failed in: load index logs, detail work, obsolete index log, index work", func(t *testing.T) {
		docs := []indexResult{}

		es.IndexFunc = func(index string, id types.ID, doc interface{}, s *session.Session) error {
			docs = append(docs, indexResult{index, id, doc})
			return nil
		}
		total := 7
		indexlog.LoadPendingIndexLogFunc = func(page, size int) ([]indexlog.IndexLogRecord, error) {
			if page == 1 {
				return nil, errors.New("error on load pending index logs")
			}
			logs := []indexlog.IndexLogRecord{}
			cur := size * (page - 1)
			n := 0
			for cur < total && n < size {
				logs = append(logs, indexlog.IndexLogRecord{ID: types.ID(cur + 1),
					IndexLog: indexlog.IndexLog{SourceId: types.ID(cur + 1)}})
				cur++
				n++
			}
			return logs, nil
		}
		work.DetailWorkFunc = func(identifier string, s *session.Session) (*work.WorkDetail, error) {
			if identifier == "3" {
				return nil, gorm.ErrRecordNotFound
			}
			if identifier == "4" {
				return nil, errors.New("error on detail work")
			}
			id, err := types.ParseID(identifier)
			if err != nil {
				return nil, err
			}
			return &work.WorkDetail{Work: domain.Work{ID: id}, State: state.State{Name: "test"}}, nil
		}
		indexlog.ObsoleteIndexLogFunc = func(id types.ID) error {
			return errors.New("error on obsolete index log")
		}
		es.IndexFunc = func(index string, id types.ID, doc interface{}, s *session.Session) error {
			if int(id-1)/indices.SyncBatchSize == 2 {
				return errors.New("error on load works")
			}
			docs = append(docs, indexResult{index, id, doc})
			return nil
		}

		indices.SyncBatchSize = 2
		Expect(indices.IndexlogRecoveryRoutine(c)).To(BeNil())

		wantedDocs := []indexResult{}
		d := work.WorkDetail{Work: domain.Work{ID: types.ID(7)}, State: state.State{Name: "test"}}
		wantedDocs = append(wantedDocs, indexResult{indices.WorkIndexName, types.ID(7),
			indices.WorkDocument{d},
		})

		Expect(len(docs)).To(Equal(1))
		Expect(docs).To(Equal(wantedDocs))
	})
}
