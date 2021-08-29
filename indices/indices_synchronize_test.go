package indices_test

import (
	"errors"
	"flywheel/account"
	"flywheel/authority"
	"flywheel/bizerror"
	"flywheel/domain"
	"flywheel/domain/work"
	"flywheel/es"
	"flywheel/event"
	"flywheel/indices"
	"flywheel/session"
	"testing"
	"time"

	"github.com/fundwit/go-commons/types"
	. "github.com/onsi/gomega"
)

func TestScheduleNewSyncRun(t *testing.T) {
	RegisterTestingT(t)

	t.Run("only system admin can schedule sync run", func(t *testing.T) {
		sec := session.Context{Perms: authority.Permissions{account.SystemViewPermission.ID}}
		success, err := indices.ScheduleNewSyncRun(&sec)
		Expect(err).To(Equal(bizerror.ErrForbidden))
		Expect(success).To(BeFalse())
	})

	t.Run("schedule sync run channel should works", func(t *testing.T) {
		indices.IndicesFullSyncFunc = func() error {
			time.Sleep(100 * time.Millisecond)
			return nil
		}

		sec := session.Context{Perms: authority.Permissions{account.SystemAdminPermission.ID}}
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
		es.DeleteDocumentByIdFunc = func(index string, id types.ID) error {
			return nil
		}
		ev := event.EventRecord{Event: event.Event{SourceType: "WORK", SourceId: 100, EventCategory: event.EventCategoryDeleted}}

		expectedResult := event.EventHandleResult{Success: true, HandlerIdentifier: indices.WorkIndexEventHandlerName}
		Expect(*indices.IndexWorkEventHandle(&ev)).To(Equal(expectedResult))
	})
	t.Run("work delete event handle failed", func(t *testing.T) {
		es.DeleteDocumentByIdFunc = func(index string, id types.ID) error {
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
		es.IndexFunc = func(index string, id types.ID, doc interface{}) error {
			return nil
		}
		work.DetailWorkFunc = func(identifier string, sec *session.Context) (*domain.WorkDetail, error) {
			return &domain.WorkDetail{}, nil
		}
		ev := event.EventRecord{Event: event.Event{SourceType: "WORK", SourceId: 100, EventCategory: event.EventCategoryCreated}}

		expectedResult := event.EventHandleResult{Success: true, HandlerIdentifier: indices.WorkIndexEventHandlerName}
		Expect(*indices.IndexWorkEventHandle(&ev)).To(Equal(expectedResult))
	})

	t.Run("failed in detail work progress for work creation event or work updating event", func(t *testing.T) {
		es.IndexFunc = func(index string, id types.ID, doc interface{}) error {
			return nil
		}
		work.DetailWorkFunc = func(identifier string, sec *session.Context) (*domain.WorkDetail, error) {
			return nil, errors.New("error on detail work")
		}
		ev := event.EventRecord{Event: event.Event{SourceType: "WORK", SourceId: 100, EventCategory: event.EventCategoryCreated}}

		expectedResult := event.EventHandleResult{
			Success:           false,
			HandlerIdentifier: indices.WorkIndexEventHandlerName,
			Message:           "detail work when index work 100, error on detail work",
		}
		Expect(*indices.IndexWorkEventHandle(&ev)).To(Equal(expectedResult))
	})

	t.Run("failed in index progress for work creation event or work updating event", func(t *testing.T) {
		es.IndexFunc = func(index string, id types.ID, doc interface{}) error {
			return errors.New("error on index document")
		}
		work.DetailWorkFunc = func(identifier string, sec *session.Context) (*domain.WorkDetail, error) {
			id, err := types.ParseID(identifier)
			if err != nil {
				return nil, err
			}
			return &domain.WorkDetail{Work: domain.Work{ID: id}}, nil
		}
		ev := event.EventRecord{Event: event.Event{SourceType: "WORK", SourceId: 100, EventCategory: event.EventCategoryCreated}}

		expectedResult := event.EventHandleResult{
			Success:           false,
			HandlerIdentifier: indices.WorkIndexEventHandlerName,
			Message:           "index work 100, map[100:error on index document]",
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
		work.LoadWorksFunc = func(page, size int) ([]domain.Work, error) {
			panic(raisedErr)
		}
		err := indices.IndicesFullSync()
		Expect(err).To(Equal(raisedErr))

		work.LoadWorksFunc = func(page, size int) ([]domain.Work, error) {
			panic("error on load works")
		}
		err = indices.IndicesFullSync()
		Expect(err).To(Equal(errors.New("error on indices full sync: error on load works")))
	})

	t.Run("should be able to index all works", func(t *testing.T) {
		docs := []indexResult{}

		es.IndexFunc = func(index string, id types.ID, doc interface{}) error {
			docs = append(docs, indexResult{index, id, doc})
			return nil
		}
		total := 5
		work.LoadWorksFunc = func(page, size int) ([]domain.Work, error) {
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

		indices.SyncBatchSize = 2
		Expect(indices.IndicesFullSync()).To(BeNil())

		wantedDocs := []indexResult{}
		for i := 0; i < total; i++ {
			wantedDocs = append(wantedDocs, indexResult{indices.WorkIndexName, types.ID(i + 1),
				indices.WorkDocument{domain.Work{ID: types.ID(i + 1)}},
			})
		}
		Expect(len(docs)).To(Equal(5))
		Expect(docs).To(Equal(wantedDocs))
	})

	t.Run("should continue to next batch when failed in load works", func(t *testing.T) {
		docs := []indexResult{}

		es.IndexFunc = func(index string, id types.ID, doc interface{}) error {
			docs = append(docs, indexResult{index, id, doc})
			return nil
		}
		total := 5
		work.LoadWorksFunc = func(page, size int) ([]domain.Work, error) {
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

		indices.SyncBatchSize = 2
		Expect(indices.IndicesFullSync()).To(BeNil())

		wantedDocs := []indexResult{}
		for i := 0; i < total; i++ {
			if i/indices.SyncBatchSize == 1 {
				continue
			}
			wantedDocs = append(wantedDocs, indexResult{indices.WorkIndexName, types.ID(i + 1),
				indices.WorkDocument{domain.Work{ID: types.ID(i + 1)}},
			})
		}
		Expect(len(docs)).To(Equal(3))
		Expect(docs).To(Equal(wantedDocs))
	})

	t.Run("should continue to next batch when failed in index works", func(t *testing.T) {
		docs := []indexResult{}

		es.IndexFunc = func(index string, id types.ID, doc interface{}) error {
			if int(id-1)/indices.SyncBatchSize == 1 {
				return errors.New("error on load works")
			}
			docs = append(docs, indexResult{index, id, doc})
			return nil
		}
		total := 5
		work.LoadWorksFunc = func(page, size int) ([]domain.Work, error) {
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

		indices.SyncBatchSize = 2
		Expect(indices.IndicesFullSync()).To(BeNil())

		wantedDocs := []indexResult{}
		for i := 0; i < total; i++ {
			if i/indices.SyncBatchSize == 1 {
				continue
			}
			wantedDocs = append(wantedDocs, indexResult{indices.WorkIndexName, types.ID(i + 1),
				indices.WorkDocument{domain.Work{ID: types.ID(i + 1)}},
			})
		}
		Expect(len(docs)).To(Equal(3))
		Expect(docs).To(Equal(wantedDocs))
	})
}
