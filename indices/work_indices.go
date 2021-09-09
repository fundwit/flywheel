package indices

import (
	"flywheel/domain/work"
	"flywheel/es"
	"fmt"

	"github.com/fundwit/go-commons/types"
	"github.com/sirupsen/logrus"
)

var (
	WorkIndexName = "works"
)

type WorkDocument struct {
	work.WorkDetail
}

type BatchActionError map[types.ID]error

func (e BatchActionError) Error() string {
	return fmt.Sprintf("%v", map[types.ID]error(e))
}

func IndexWorks(works []work.WorkDetail) error {
	docs := make([]WorkDocument, 0, len(works))
	for _, work := range works {

		docs = append(docs, WorkDocument{WorkDetail: work})
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
			logrus.Warnf("index work %d %s %s\n", doc.ID, doc.Identifier, err)
		} else {
			logrus.Infof("index work %d %s successfully\n", doc.ID, doc.Identifier)
		}
	}
	if len(errs) == 0 {
		return nil
	}
	return errs
}
