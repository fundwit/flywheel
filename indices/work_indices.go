package indices

import (
	"flywheel/domain"
	"flywheel/domain/work"
	"flywheel/es"
	"fmt"
	"log"

	"github.com/fundwit/go-commons/types"
)

var (
	WorkIndexName   = "works"
	ExtendWorksFunc = work.ExtendWorks
)

type WorkDocument struct {
	domain.Work
}

type BatchActionError map[types.ID]error

func (e BatchActionError) Error() string {
	return fmt.Sprintf("%v", map[types.ID]error(e))
}

func IndexWorks(works []domain.Work) error {
	docs := make([]WorkDocument, 0, len(works))
	for _, work := range works {
		docs = append(docs, WorkDocument{Work: work})
	}

	if err := saveWorkDocuments(docs); err != nil {
		return err
	}
	return nil
}

func saveWorkDocuments(workDocs []WorkDocument) BatchActionError {
	errs := BatchActionError{}

	for _, doc := range workDocs {
		if err := es.IndexFunc(WorkIndexName, doc.ID, doc); err != nil {
			errs[doc.ID] = err
			log.Printf("index work %d %s\n", doc.ID, err)
		} else {
			log.Printf("index work %d successfully\n", doc.ID)
		}
	}
	if len(errs) == 0 {
		return nil
	}
	return errs
}
