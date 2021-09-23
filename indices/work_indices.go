package indices

import (
	"flywheel/client/es"
	"flywheel/domain/work"
	"flywheel/session"
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

func IndexWorks(works []work.WorkDetail, s *session.Session) error {
	docs := make([]WorkDocument, 0, len(works))
	for _, work := range works {

		docs = append(docs, WorkDocument{WorkDetail: work})
	}

	if err := saveWorkDocuments(docs, s); err != nil {
		return err
	}
	return nil
}

func saveWorkDocuments(workDocs []WorkDocument, s *session.Session) BatchActionError {
	errs := BatchActionError{}

	for _, doc := range workDocs {
		if err := es.IndexFunc(WorkIndexName, doc.ID, doc, s); err != nil {
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
