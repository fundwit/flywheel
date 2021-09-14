package workcontribution_test

import (
	"bytes"
	"encoding/json"
	"flywheel/bizerror"
	"flywheel/domain/workcontribution"
	"flywheel/session"
	"flywheel/testinfra"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/fundwit/go-commons/types"
	"github.com/gin-gonic/gin"
	. "github.com/onsi/gomega"
)

func TestHandleQueryContributionsAPI(t *testing.T) {
	RegisterTestingT(t)

	router := gin.Default()
	router.Use(bizerror.ErrorHandling())
	workcontribution.RegisterWorkContributionsHandlers(router)

	t.Run("should be able to handle work contributions query rest api request and response", func(t *testing.T) {
		var reqBody *workcontribution.WorkContributionsQuery
		workcontribution.QueryWorkContributionsFunc = func(query workcontribution.WorkContributionsQuery, sec *session.Context) (*[]workcontribution.WorkContributionRecord, error) {
			reqBody = &query
			return &[]workcontribution.WorkContributionRecord{{
				WorkContribution: workcontribution.WorkContribution{WorkKey: "TEST-1", ContributorId: 1000},
				ID:               100, ContributorName: "user 1000", WorkProjectId: 100, BeginTime: types.TimestampOfDate(2021, 1, 1, 12, 0, 0, 0, time.UTC),
				EndTime: types.TimestampOfDate(2021, 1, 1, 12, 1, 0, 0, time.UTC), Effective: true,
			}}, nil
		}

		creation := workcontribution.WorkContributionsQuery{WorkKeys: []string{"TEST-3", "TEST-4"}}
		reqBodyJson, _ := json.Marshal(creation)
		req := httptest.NewRequest(http.MethodPost, workcontribution.PathWorkContributionsQueryRoot+"?workKey=TEST-1&workKey=TEST-2", bytes.NewReader(reqBodyJson))
		status, respbody, _ := testinfra.ExecuteRequest(req, router)
		Expect(status).To(Equal(http.StatusOK))
		Expect(respbody).To(MatchJSON(`[{"id": "100", "workKey": "TEST-1", "contributorId": "1000", "contributorName":"user 1000", "workProjectId": "100",
			"checkitemId":"0", "beginTime": "2021-01-01T12:00:00Z", "endTime":"2021-01-01T12:01:00Z", "effective": true}]`))

		Expect(*reqBody).To(Equal(workcontribution.WorkContributionsQuery{WorkKeys: []string{"TEST-1", "TEST-2", "TEST-3", "TEST-4"}}))
	})
}

func TestHandleBeginContributionAPI(t *testing.T) {
	RegisterTestingT(t)

	router := gin.Default()
	router.Use(bizerror.ErrorHandling())
	workcontribution.RegisterWorkContributionsHandlers(router)

	t.Run("should be able to handle work contribution begining rest api request and response", func(t *testing.T) {
		var reqBody *workcontribution.WorkContribution
		workcontribution.BeginWorkContributionFunc = func(d *workcontribution.WorkContribution, sec *session.Context) (types.ID, error) {
			reqBody = d
			return 12345, nil
		}

		reqBodyJson := `{"workKey": "TEST-1", "contributorId": 200, "checkitemId": 300}`
		req := httptest.NewRequest(http.MethodPost, workcontribution.PathWorkContributionsRoot, strings.NewReader(reqBodyJson))
		status, respbody, _ := testinfra.ExecuteRequest(req, router)
		Expect(status).To(Equal(http.StatusCreated))
		Expect(respbody).To(MatchJSON(`{"id": "12345"}`))

		Expect(*reqBody).To(Equal(workcontribution.WorkContribution{WorkKey: "TEST-1", ContributorId: 200, CheckitemId: 300}))
	})
}

func TestHandleFinishContributionAPI(t *testing.T) {
	RegisterTestingT(t)

	router := gin.Default()
	router.Use(bizerror.ErrorHandling())
	workcontribution.RegisterWorkContributionsHandlers(router)

	t.Run("should be able to handle work contribution finish rest api request and response", func(t *testing.T) {
		var reqBody *workcontribution.WorkContributionFinishBody
		workcontribution.FinishWorkContributionFunc = func(d *workcontribution.WorkContributionFinishBody, sec *session.Context) error {
			reqBody = d
			return nil
		}

		originBody := workcontribution.WorkContributionFinishBody{
			WorkContribution: workcontribution.WorkContribution{WorkKey: "TEST-1", ContributorId: 200}, Effective: true}
		reqBodyJson, _ := json.Marshal(originBody)
		req := httptest.NewRequest(http.MethodPut, workcontribution.PathWorkContributionsRoot, bytes.NewReader(reqBodyJson))
		status, respbody, _ := testinfra.ExecuteRequest(req, router)
		Expect(status).To(Equal(http.StatusOK))
		Expect(respbody).To(BeZero())

		Expect(*reqBody).To(Equal(originBody))
	})
}
