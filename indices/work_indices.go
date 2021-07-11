package indices

import (
	"encoding/json"
	"flywheel/bizerror"
	"flywheel/domain"
	"flywheel/es"
	"flywheel/session"
	"fmt"
	"strings"

	"github.com/fundwit/go-commons/types"
)

var (
	WorkIndexName = "works"
)

type WorkDocument struct {
	domain.Work
}

type BatchActionError map[types.ID]error

func (e BatchActionError) Error() string {
	return fmt.Sprintf("%v", map[types.ID]error(e))
}

func QueryWork(query *domain.WorkQuery, sec *session.Context) (*[]domain.Work, error) {
	q := db.Where(domain.Work{ProjectID: query.ProjectID})
	if query.Name != "" {
		q = q.Where("name like ?", "%"+query.Name+"%")
	}
	if len(query.StateCategories) > 0 {
		q = q.Where("state_category in (?)", query.StateCategories)
	}

	if query.ArchiveState == domain.StatusOn {
		q = q.Where("archive_time != ?", types.Timestamp{})
	} else if query.ArchiveState == domain.StatusAll {
		// archive_time not in where clause
	} else {
		q = q.Where("archive_time = ?", types.Timestamp{})
	}
	visibleProjects := sec.VisibleProjects()
	if len(visibleProjects) == 0 {
		return &[]domain.Work{}, nil
	}
	q = q.Where("project_id in (?)", visibleProjects).Order("order_in_state ASC")

	if err := q.Find(&works).Error; err != nil {
		return nil, err
	}

	// append Work.state
	workflowCache := map[types.ID]*domain.WorkflowDetail{}
	var err error
	for i := len(works) - 1; i >= 0; i-- {
		work := works[i]
		workflow := workflowCache[work.FlowID]
		if workflow == nil {
			workflow, err = m.workflowManager.DetailWorkflow(work.FlowID, sec)
			if err != nil {
				return nil, err
			}
			workflowCache[work.FlowID] = workflow
		}

		stateFound, found := workflow.FindState(work.StateName)
		if !found {
			return nil, bizerror.ErrStateInvalid
		}
		works[i].State = stateFound
	}

	return &works, nil
}

func SearchWorks(domain.WorkQuery) ([]domain.Work, error) {

	r, err := es.Search(WorkIndexName, map[string]interface{}{})
	if err != nil {
		return nil, err
	}
	result := make([]domain.Work, 0, len(r.Hits.Hits))
	for _, hit := range r.Hits.Hits {
		r := domain.Work{}
		if err := json.NewDecoder(strings.NewReader(string(hit.Source))).Decode(&r); err != nil {
			return nil, fmt.Errorf(string(hit.Source))
		}
		result = append(result, r)
	}
	return result, nil
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
		if err := es.Index(WorkIndexName, doc.ID, doc); err != nil {
			errs[doc.ID] = err
		}
	}
	if len(errs) == 0 {
		return nil
	}
	return errs
}
