package indices

import (
	"encoding/json"
	"flywheel/bizerror"
	"flywheel/domain"
	"flywheel/domain/flow"
	"flywheel/es"
	"flywheel/session"
	"fmt"
	"strings"

	"github.com/fundwit/go-commons/types"
)

var (
	WorkIndexName   = "works"
	ExtendWorksFunc = ExtendWorks
)

type WorkDocument struct {
	domain.Work
}

type BatchActionError map[types.ID]error

func (e BatchActionError) Error() string {
	return fmt.Sprintf("%v", map[types.ID]error(e))
}

func SearchWorks(q domain.WorkQuery, sec *session.Context) ([]domain.Work, error) {
	visibleProjects := sec.VisibleProjects()
	if len(visibleProjects) == 0 {
		return []domain.Work{}, nil
	}

	/*
		{
			"query": {
				"bool": {
					"filter": [
						{"term": {"projectId": 111}},
						{"terms": {"projectId": [111, 222]}},

						{"match": {"name": {"query": "xxx", "operator": "AND"}}},
						{"terms": {"stateCategory": ["xxx"]}},

						{"exists": {"field": "archiveTime"}},
						{"bool": {"must_not": {"exists": {"field": "archiveTime"}}}}

						{"terms": {"projectId": []}}
					]
				}
			},
			"size": 10000,
			"sort": [
				{"@timestamp": {"order": "asc", "format": "strict_date_optional_time_nanos"}},
				{"_shard_doc": "desc"}
			]
		}
	*/
	filters := make([]es.H, 0, 10)
	filters = append(filters, es.H{"term": es.H{"projectId": q.ProjectID}})
	// filters = q.Where("project_id in (?)", visibleProjects).Order("order_in_state ASC")
	filters = append(filters, es.H{"terms": es.H{"projectId": visibleProjects}})

	if q.Name != "" {
		filters = append(filters, es.H{"match": es.H{"name": es.H{"query": q.Name, "operator": "AND"}}})
	}
	if len(q.StateCategories) > 0 {
		filters = append(filters, es.H{"terms": es.H{"stateCategory": q.StateCategories}})
	}
	if q.ArchiveState == domain.StatusOn {
		filters = append(filters, es.H{"exists": es.H{"field": "archivedTime"}})
	} else if q.ArchiveState == domain.StatusOff {
		filters = append(filters, es.H{"bool": es.H{"must_not": es.H{"exists": es.H{"field": "archivedTime"}}}})
	}

	sorts := make([]es.H, 0, 1)
	sorts = append(sorts, es.H{"orderInState": es.H{"order": "asc"}})

	root := es.H{"bool": es.H{"filter": filters}}
	r, err := es.SearchFunc(WorkIndexName, es.H{"size": 10000, "query": root, "sort": sorts})
	if err != nil {
		return nil, err
	}
	works := make([]domain.Work, 0, len(r.Hits.Hits))
	for _, hit := range r.Hits.Hits {
		r := domain.Work{}
		if err := json.NewDecoder(strings.NewReader(string(hit.Source))).Decode(&r); err != nil {
			return nil, fmt.Errorf(string(hit.Source))
		}
		works = append(works, r)
	}

	if err := ExtendWorksFunc(works, sec); err != nil {
		return nil, err
	}

	return works, nil
}

// append Work.state
func ExtendWorks(works []domain.Work, sec *session.Context) error {
	var err error
	workflowCache := map[types.ID]*domain.WorkflowDetail{}
	for i := len(works) - 1; i >= 0; i-- {
		work := works[i]
		workflow := workflowCache[work.FlowID]
		if workflow == nil {
			workflow, err = flow.DetailWorkflowFunc(work.FlowID, sec)
			if err != nil {
				return err
			}
			workflowCache[work.FlowID] = workflow
		}

		stateFound, found := workflow.FindState(work.StateName)
		if !found {
			return bizerror.ErrStateInvalid
		}
		works[i].State = stateFound
	}
	return nil
}

func IndexWorks(works []domain.Work) error {
	docs, err := buildWorkDocuments(works)
	if err != nil {
		return err
	}
	if err := saveWorkDocuments(docs); err != nil {
		return err
	}
	return nil
}

func buildWorkDocuments(works []domain.Work) ([]WorkDocument, error) {
	docs := make([]WorkDocument, 0, len(works))
	for _, work := range works {
		docs = append(docs, WorkDocument{Work: work})
	}
	return docs, nil
}

func saveWorkDocuments(workDocs []WorkDocument) BatchActionError {
	errs := BatchActionError{}

	for _, doc := range workDocs {
		if err := es.IndexFunc(WorkIndexName, doc.ID, doc); err != nil {
			errs[doc.ID] = err
		}
	}
	if len(errs) == 0 {
		return nil
	}
	return errs
}
