package search

import (
	"flywheel/domain"
	"flywheel/domain/state"
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

func TestSearchWorks(t *testing.T) {
	RegisterTestingT(t)

	t.Run("should be able to search works", func(t *testing.T) {
		defer afterEach(t)
		beforeEach(t)

		ts := types.TimestampOfDate(2020, 1, 2, 3, 4, 5, 0, time.Local)
		w1000 := domain.Work{ID: 1000, Name: "test", ProjectID: 100, CreateTime: types.CurrentTimestamp(),
			FlowID: 100, Identifier: "DEM-1000",
			OrderInState: 1, StateName: "PENDING", StateCategory: 1,
			StateBeginTime: ts, ProcessBeginTime: ts, ProcessEndTime: ts, ArchiveTime: types.Timestamp{}}

		w1001 := domain.Work{ID: 1001, Name: "demo1-1001", ProjectID: 100, CreateTime: types.CurrentTimestamp(),
			FlowID: 100, Identifier: "DEM-1001",
			OrderInState: 1, StateName: "DOING", StateCategory: 2,
			StateBeginTime: ts, ProcessBeginTime: ts, ProcessEndTime: ts, ArchiveTime: types.Timestamp{}}

		w1002 := domain.Work{ID: 1002, Name: "demo2-1002", ProjectID: 100, CreateTime: types.CurrentTimestamp(),
			FlowID: 100, Identifier: "DEM-1002",
			OrderInState: 1624588781665, StateName: "DONE", StateCategory: 3,
			StateBeginTime: ts, ProcessBeginTime: ts, ProcessEndTime: ts, ArchiveTime: types.Timestamp{}}

		w1003 := domain.Work{ID: 1003, Name: "demo3-1003", ProjectID: 100, CreateTime: types.CurrentTimestamp(),
			FlowID: 100, Identifier: "DEM-1003",
			OrderInState: 1624585966518, StateName: "DONE", StateCategory: 3,
			StateBeginTime: ts, ProcessBeginTime: ts, ProcessEndTime: ts, ArchiveTime: ts}

		w2002 := domain.Work{ID: 2002, Name: "test-2002", ProjectID: 200, CreateTime: types.CurrentTimestamp(),
			FlowID: 100, Identifier: "DEM-2002",
			OrderInState: 1, StateName: "PENDING", StateCategory: 1,
			StateBeginTime: ts, ProcessBeginTime: ts, ProcessEndTime: ts, ArchiveTime: types.Timestamp{}}

		// do: create doc
		Expect(indices.IndexWorks([]work.WorkDetail{{Work: w1000}})).To(BeNil())
		Expect(indices.IndexWorks([]work.WorkDetail{{Work: w1001}})).To(BeNil())
		Expect(indices.IndexWorks([]work.WorkDetail{{Work: w1002}})).To(BeNil())
		Expect(indices.IndexWorks([]work.WorkDetail{{Work: w1003}})).To(BeNil()) // archived
		Expect(indices.IndexWorks([]work.WorkDetail{{Work: w2002}})).To(BeNil())

		// assert: visible project limit
		works, err := SearchWorks(domain.WorkQuery{}, &session.Context{})
		Expect(err).To(BeNil())
		Expect(len(works)).To(BeZero())

		works, err = SearchWorks(domain.WorkQuery{}, &session.Context{Perms: []string{"manager_111", "common_222"}})
		Expect(err).To(BeNil())
		Expect(len(works)).To(BeZero())

		works, err = SearchWorks(domain.WorkQuery{ProjectID: 100}, &session.Context{Perms: []string{"manager_200", "common_222"}})
		Expect(err).To(BeNil())
		Expect(len(works)).To(BeZero())

		works, err = SearchWorks(domain.WorkQuery{ProjectID: 200}, &session.Context{Perms: []string{"manager_200", "common_222"}})
		Expect(err).To(BeNil())
		Expect(len(works)).To(Equal(1))
		Expect(works[0]).To(Equal(work.WorkDetail{Work: w2002}))

		works, err = SearchWorks(domain.WorkQuery{ProjectID: 100, Name: "demo1"}, &session.Context{Perms: []string{"manager_200", "common_100"}})
		Expect(err).To(BeNil())
		Expect(len(works)).To(Equal(1))
		Expect(works[0]).To(Equal(work.WorkDetail{Work: w1001}))

		works, err = SearchWorks(domain.WorkQuery{ProjectID: 100, ArchiveState: "ALL",
			StateCategories: []state.Category{state.InProcess, state.Done}},
			&session.Context{Perms: []string{"common_100"}})
		Expect(err).To(BeNil())
		Expect(len(works)).To(Equal(3))
		Expect(works[0]).To(Equal(work.WorkDetail{Work: w1001}))
		Expect(works[1]).To(Equal(work.WorkDetail{Work: w1003}))
		Expect(works[2]).To(Equal(work.WorkDetail{Work: w1002}))

		works, err = SearchWorks(domain.WorkQuery{ProjectID: 100, ArchiveState: "ON",
			StateCategories: []state.Category{state.InProcess, state.Done}},
			&session.Context{Perms: []string{"common_100"}})
		Expect(err).To(BeNil())
		Expect(len(works)).To(Equal(1))
		Expect(works[0]).To(Equal(work.WorkDetail{Work: w1003}))

		works, err = SearchWorks(domain.WorkQuery{ProjectID: 100, ArchiveState: "OFF",
			StateCategories: []state.Category{state.InProcess, state.Done}},
			&session.Context{Perms: []string{"common_100"}})
		Expect(err).To(BeNil())
		Expect(len(works)).To(Equal(2))
		Expect(works[0]).To(Equal(work.WorkDetail{Work: w1001}))
		Expect(works[1]).To(Equal(work.WorkDetail{Work: w1002}))

		// default archive state is OFF
		works, err = SearchWorks(domain.WorkQuery{ProjectID: 100,
			StateCategories: []state.Category{state.InProcess, state.Done}},
			&session.Context{Perms: []string{"common_100"}})
		Expect(err).To(BeNil())
		Expect(len(works)).To(Equal(2))
		Expect(works[0]).To(Equal(work.WorkDetail{Work: w1001}))
		Expect(works[1]).To(Equal(work.WorkDetail{Work: w1002}))
	})
}

func beforeEach(t *testing.T) {
	es.CreateClientFromEnv()
	es.IndexFunc = es.Index
	work.ExtendWorksFunc = func(works []domain.Work, sec *session.Context) ([]work.WorkDetail, error) {
		details := make([]work.WorkDetail, 0, len(works))
		for _, w := range works {
			details = append(details, work.WorkDetail{Work: w})
		}
		return details, nil
	}

	indices.WorkIndexName = "works_test_" + strings.ReplaceAll(uuid.New().String(), "-", "")
}

func afterEach(t *testing.T) {
	work.ExtendWorksFunc = work.ExtendWorks
	if strings.Contains(indices.WorkIndexName, "_test_") {
		Expect(es.DropIndex(indices.WorkIndexName)).To(BeNil())
	}
}
