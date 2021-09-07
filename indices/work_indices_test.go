package indices_test

import (
	"encoding/json"
	"flywheel/domain"
	"flywheel/domain/work"
	"flywheel/es"
	"flywheel/indices"
	"flywheel/session"
	"strings"
	"testing"
	"time"

	"github.com/fundwit/go-commons/types"
	"github.com/google/uuid"
	. "github.com/onsi/gomega"
)

func TestIndexWorks(t *testing.T) {
	RegisterTestingT(t)

	t.Run("should be able to index works", func(t *testing.T) {
		defer afterEach(t)
		beforeEach(t)

		ts := types.TimestampOfDate(2020, 1, 2, 3, 4, 5, 0, time.Local)
		w := domain.Work{ID: 1, Name: "test", ProjectID: 100, CreateTime: types.CurrentTimestamp(), FlowID: 100, Identifier: "DEM-1",
			OrderInState: 1, StateName: "PENDING", StateCategory: 1, State: domain.StatePending,
			StateBeginTime: ts, ProcessBeginTime: ts, ProcessEndTime: ts, ArchiveTime: ts}

		// do: create doc
		Expect(indices.IndexWorks([]domain.Work{w})).To(BeNil())

		// assert: doc existed
		source, err := es.GetDocument(indices.WorkIndexName, 1)
		Expect(err).To(BeNil())
		record := indices.WorkDocument{}
		err = json.Unmarshal([]byte(source), &record)
		Expect(err).To(BeNil())
		Expect(record.Work).To(Equal(w))

		// do: update doc
		w1 := domain.Work{ID: 1, Name: "test-updated", ProjectID: 100, CreateTime: types.CurrentTimestamp(), FlowID: 100, Identifier: "DEM-1",
			OrderInState: 2, StateName: "DOING", StateCategory: 2, State: domain.StateDoing,
			StateBeginTime: ts, ProcessBeginTime: ts, ProcessEndTime: ts, ArchiveTime: ts}
		Expect(indices.IndexWorks([]domain.Work{w1})).To(BeNil())

		// assert: doc existed
		source, err = es.GetDocument(indices.WorkIndexName, 1)
		Expect(err).To(BeNil())
		record = indices.WorkDocument{}
		err = json.Unmarshal([]byte(source), &record)
		Expect(err).To(BeNil())
		Expect(record.Work).To(Equal(w1))
	})
}

func beforeEach(t *testing.T) {
	es.CreateClientFromEnv()
	es.IndexFunc = es.Index

	indices.ExtendWorksFunc = func(works []domain.Work, sec *session.Context) ([]work.WorkDetail, error) {
		return nil, nil
	}
	indices.WorkIndexName = "works_test_" + strings.ReplaceAll(uuid.New().String(), "-", "")
}

func afterEach(t *testing.T) {
	indices.ExtendWorksFunc = work.ExtendWorks
	if strings.Contains(indices.WorkIndexName, "_test_") {
		Expect(es.DropIndex(indices.WorkIndexName)).To(BeNil())
	}
}
