package search

import (
	"context"
	"flywheel/domain"
	"flywheel/domain/state"
	"flywheel/domain/work"
	"flywheel/domain/work/checklist"
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
		s := &session.Session{Context: context.Background()}

		ts := types.TimestampOfDate(2020, 1, 2, 3, 4, 5, 0, time.Local)
		w1000 := work.WorkDetail{
			Work: domain.Work{ID: 1000, Name: "test", ProjectID: 100, CreateTime: types.CurrentTimestamp(),
				FlowID: 100, Identifier: "DEM-1000",
				OrderInState: 1, StateName: "PENDING", StateCategory: 1,
				StateBeginTime: ts, ProcessBeginTime: ts, ProcessEndTime: ts, ArchiveTime: types.Timestamp{}},
			CheckList: []checklist.CheckItem{{ID: 1000, Name: "checkitem 1000"}},
		}

		w1001 := work.WorkDetail{
			Work: domain.Work{ID: 1001, Name: "demo1-1001", ProjectID: 100, CreateTime: types.CurrentTimestamp(),
				FlowID: 100, Identifier: "DEM-1001",
				OrderInState: 1, StateName: "DOING", StateCategory: 2,
				StateBeginTime: ts, ProcessBeginTime: ts, ProcessEndTime: ts, ArchiveTime: types.Timestamp{}},
			CheckList: []checklist.CheckItem{{ID: 1001, Name: "checkitem 1001"}},
		}

		w1002 := work.WorkDetail{
			Work: domain.Work{ID: 1002, Name: "demo2-1002", ProjectID: 100, CreateTime: types.CurrentTimestamp(),
				FlowID: 100, Identifier: "DEM-1002",
				OrderInState: 1624588781665, StateName: "DONE", StateCategory: 3,
				StateBeginTime: ts, ProcessBeginTime: ts, ProcessEndTime: ts, ArchiveTime: types.Timestamp{}},
			CheckList: []checklist.CheckItem{{ID: 1002, Name: "checkitem 1002"}},
		}

		w1003 := work.WorkDetail{
			Work: domain.Work{ID: 1003, Name: "demo3-1003", ProjectID: 100, CreateTime: types.CurrentTimestamp(),
				FlowID: 100, Identifier: "DEM-1003",
				OrderInState: 1624585966518, StateName: "DONE", StateCategory: 3,
				StateBeginTime: ts, ProcessBeginTime: ts, ProcessEndTime: ts, ArchiveTime: ts},
			CheckList: []checklist.CheckItem{{ID: 1003, Name: "checkitem 1003"}},
		}

		w2002 := work.WorkDetail{
			Work: domain.Work{ID: 2002, Name: "test-2002", ProjectID: 200, CreateTime: types.CurrentTimestamp(),
				FlowID: 100, Identifier: "DEM-2002",
				OrderInState: 1, StateName: "PENDING", StateCategory: 1,
				StateBeginTime: ts, ProcessBeginTime: ts, ProcessEndTime: ts, ArchiveTime: types.Timestamp{}},
			CheckList: []checklist.CheckItem{{ID: 2002, Name: "checkitem 2002"}},
		}

		// do: create doc
		Expect(indices.IndexWorks([]work.WorkDetail{w1000}, s)).To(BeNil())
		Expect(indices.IndexWorks([]work.WorkDetail{w1001}, s)).To(BeNil())
		Expect(indices.IndexWorks([]work.WorkDetail{w1002}, s)).To(BeNil())
		Expect(indices.IndexWorks([]work.WorkDetail{w1003}, s)).To(BeNil()) // archived
		Expect(indices.IndexWorks([]work.WorkDetail{w2002}, s)).To(BeNil())

		// assert: visible project limit
		works, err := SearchWorks(domain.WorkQuery{}, &session.Session{})
		Expect(err).To(BeNil())
		Expect(len(works)).To(BeZero())

		works, err = SearchWorks(domain.WorkQuery{}, &session.Session{Perms: []string{"manager_111", "common_222"}})
		Expect(err).To(BeNil())
		Expect(len(works)).To(BeZero())

		works, err = SearchWorks(domain.WorkQuery{ProjectID: 100}, &session.Session{Perms: []string{"manager_200", "common_222"}})
		Expect(err).To(BeNil())
		Expect(len(works)).To(BeZero())

		works, err = SearchWorks(domain.WorkQuery{ProjectID: 200}, &session.Session{Perms: []string{"manager_200", "common_222"}})
		Expect(err).To(BeNil())
		Expect(len(works)).To(Equal(1))
		Expect(works[0]).To(Equal(w2002))

		works, err = SearchWorks(domain.WorkQuery{ProjectID: 100, Name: "demo1"}, &session.Session{Perms: []string{"manager_200", "common_100"}})
		Expect(err).To(BeNil())
		Expect(len(works)).To(Equal(1))
		Expect(works[0]).To(Equal(w1001))

		works, err = SearchWorks(domain.WorkQuery{ProjectID: 100, ArchiveState: "ALL",
			StateCategories: []state.Category{state.InProcess, state.Done}},
			&session.Session{Perms: []string{"common_100"}})
		Expect(err).To(BeNil())
		Expect(len(works)).To(Equal(3))
		Expect(works[0]).To(Equal(w1001))
		Expect(works[1]).To(Equal(w1003))
		Expect(works[2]).To(Equal(w1002))

		works, err = SearchWorks(domain.WorkQuery{ProjectID: 100, ArchiveState: "ON",
			StateCategories: []state.Category{state.InProcess, state.Done}},
			&session.Session{Perms: []string{"common_100"}})
		Expect(err).To(BeNil())
		Expect(len(works)).To(Equal(1))
		Expect(works[0]).To(Equal(w1003))

		works, err = SearchWorks(domain.WorkQuery{ProjectID: 100, ArchiveState: "OFF",
			StateCategories: []state.Category{state.InProcess, state.Done}},
			&session.Session{Perms: []string{"common_100"}})
		Expect(err).To(BeNil())
		Expect(len(works)).To(Equal(2))
		Expect(works[0]).To(Equal(w1001))
		Expect(works[1]).To(Equal(w1002))

		// default archive state is OFF
		works, err = SearchWorks(domain.WorkQuery{ProjectID: 100,
			StateCategories: []state.Category{state.InProcess, state.Done}},
			&session.Session{Perms: []string{"common_100"}})
		Expect(err).To(BeNil())
		Expect(len(works)).To(Equal(2))
		Expect(works[0]).To(Equal(w1001))
		Expect(works[1]).To(Equal(w1002))
	})
}

func beforeEach(t *testing.T) {
	es.CreateClientFromEnv()
	es.IndexFunc = es.Index
	work.ExtendWorksFunc = func(details []work.WorkDetail, s *session.Session) ([]work.WorkDetail, error) {
		return details, nil
	}

	indices.WorkIndexName = "works_test_" + strings.ReplaceAll(uuid.New().String(), "-", "")
}

func afterEach(t *testing.T) {
	work.ExtendWorksFunc = work.ExtendWorks
	if strings.Contains(indices.WorkIndexName, "_test_") {
		Expect(es.DropIndex(indices.WorkIndexName, &session.Session{Context: context.Background()})).To(BeNil())
	}
}
