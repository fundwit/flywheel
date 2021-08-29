package search

import (
	"encoding/json"
	"flywheel/domain"
	"flywheel/es"
	"flywheel/indices"
	"flywheel/session"
	"fmt"
	"strings"
)

var (
	SearchWorksFunc = SearchWorks
)

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
	} else if q.ArchiveState == domain.StatusAll {
		// do nothing
	} else {
		filters = append(filters, es.H{"bool": es.H{"must_not": es.H{"exists": es.H{"field": "archivedTime"}}}})
	}

	sorts := make([]es.H, 0, 1)
	sorts = append(sorts, es.H{"orderInState": es.H{"order": "asc"}})

	root := es.H{"bool": es.H{"filter": filters}}
	r, err := es.SearchFunc(indices.WorkIndexName, es.H{"size": 10000, "query": root, "sort": sorts})
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

	if err := indices.ExtendWorksFunc(works, sec); err != nil {
		return nil, err
	}

	return works, nil
}
