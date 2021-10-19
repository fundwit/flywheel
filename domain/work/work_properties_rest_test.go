package work_test

import (
	"errors"
	"flywheel/bizerror"
	"flywheel/domain"
	"flywheel/domain/work"
	"flywheel/session"
	"flywheel/testinfra"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/fundwit/go-commons/types"
	"github.com/gin-gonic/gin"
	. "github.com/onsi/gomega"
)

func TestAssignWorkPropertyValueAPI(t *testing.T) {
	RegisterTestingT(t)

	router := gin.Default()
	router.Use(bizerror.ErrorHandling())
	work.RegisterWorkPropertiesRestAPI(router)

	t.Run("should be able to validate parameters", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPatch, work.PathWorkProperties, strings.NewReader("{}"))
		status, body, _ := testinfra.ExecuteRequest(req, router)
		Expect(body).To(MatchJSON(`{"code":"common.bad_param",
		"message": "Key: 'WorkPropertyAssign.WorkId' Error:Field validation for 'WorkId' failed on the 'required' tag\n` +
			`Key: 'WorkPropertyAssign.Name' Error:Field validation for 'Name' failed on the 'required' tag",
		"data":null}`))
		Expect(status).To(Equal(http.StatusBadRequest))

		req = httptest.NewRequest(http.MethodPatch, work.PathWorkProperties, nil)
		status, body, _ = testinfra.ExecuteRequest(req, router)
		Expect(status).To(Equal(http.StatusBadRequest))
		Expect(body).To(MatchJSON(`{"code": "common.bad_param", "message": "EOF", "data": null}`))

		req = httptest.NewRequest(http.MethodPatch, work.PathWorkProperties, strings.NewReader(" \t "))
		status, body, _ = testinfra.ExecuteRequest(req, router)
		Expect(status).To(Equal(http.StatusBadRequest))
		Expect(body).To(MatchJSON(`{"code": "common.bad_param", "message": "EOF", "data": null}`))

		req = httptest.NewRequest(http.MethodPatch, work.PathWorkProperties, strings.NewReader(" xx "))
		status, body, _ = testinfra.ExecuteRequest(req, router)
		Expect(status).To(Equal(http.StatusBadRequest))
		Expect(body).To(MatchJSON(`{"code": "common.bad_param", "message": "invalid character 'x' looking for beginning of value", "data": null}`))
	})

	t.Run("should be able to handle error", func(t *testing.T) {
		work.AssignWorkPropertyValueFunc = func(req work.WorkPropertyAssign, c *session.Session) (*work.WorkPropertyValueRecord, error) {
			return nil, errors.New("some error")
		}

		reqBody := `{"workId":"10", "name":"xxx"}`
		req := httptest.NewRequest(http.MethodPatch, work.PathWorkProperties, strings.NewReader(reqBody))
		status, body, _ := testinfra.ExecuteRequest(req, router)
		Expect(status).To(Equal(http.StatusInternalServerError))
		Expect(body).To(MatchJSON(`{"code":"common.internal_server_error", "message":"some error", "data":null}`))
	})

	t.Run("should be able to assign work property value successfully", func(t *testing.T) {
		work.AssignWorkPropertyValueFunc = func(req work.WorkPropertyAssign, c *session.Session) (*work.WorkPropertyValueRecord, error) {
			return &work.WorkPropertyValueRecord{WorkId: req.WorkId, Name: req.Name, Value: req.Value, Type: "text", PropertyDefinitionId: 3000}, nil
		}

		reqBody := `{"workId":"123", "name":"propName", "value":"propValue"}`
		req := httptest.NewRequest(http.MethodPatch, work.PathWorkProperties, strings.NewReader(reqBody))
		status, body, _ := testinfra.ExecuteRequest(req, router)
		Expect(body).To(MatchJSON(`{"workId": "123", "name": "propName", "value": "propValue", "type":"text", "propertyDefinitionId":"3000"}`))
		Expect(status).To(Equal(http.StatusOK))
	})
}

func TestQueryWorkPropertyValuesAPI(t *testing.T) {
	RegisterTestingT(t)

	router := gin.Default()
	router.Use(bizerror.ErrorHandling())
	work.RegisterWorkPropertiesRestAPI(router)

	t.Run("should be able to validate parameters", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, work.PathWorkProperties, nil)
		status, body, _ := testinfra.ExecuteRequest(req, router)

		Expect(body).To(MatchJSON(`{"code":"common.bad_param",
		"message": "Key: 'workPropertyValuesQuery.WorkIds' Error:Field validation for 'WorkIds' failed on the 'gte' tag",
		"data": null}`))
		Expect(status).To(Equal(http.StatusBadRequest))
	})

	t.Run("should be able to handle error", func(t *testing.T) {
		work.QueryWorkPropertyValuesFunc = func(reqWorkIds []types.ID, s *session.Session) ([]work.WorksPropertyValueDetail, error) {
			return nil, errors.New("some error")
		}

		req := httptest.NewRequest(http.MethodGet, work.PathWorkProperties+"?workId=123", nil)
		status, body, _ := testinfra.ExecuteRequest(req, router)
		Expect(status).To(Equal(http.StatusInternalServerError))
		Expect(body).To(MatchJSON(`{"code":"common.internal_server_error", "message":"some error", "data":null}`))
	})

	t.Run("should be able to query work property values successfully", func(t *testing.T) {
		work.QueryWorkPropertyValuesFunc = func(reqWorkIds []types.ID, s *session.Session) ([]work.WorksPropertyValueDetail, error) {
			return []work.WorksPropertyValueDetail{
				{
					WorkId: reqWorkIds[0],
					PropertyValues: []work.WorkPropertyValueDetail{
						{PropertyDefinitionId: 1245, Value: "value of test prop", PropertyDefinition: domain.PropertyDefinition{
							Name: "testProp", Type: "text", Title: "Test Property",
						}},
					},
				},
			}, nil
		}

		req := httptest.NewRequest(http.MethodGet, work.PathWorkProperties+"?workId=1111", nil)
		status, body, _ := testinfra.ExecuteRequest(req, router)
		Expect(body).To(MatchJSON(`
		[{
			"workId": "1111",
			"propertyValues": [{
				"name": "testProp", "value": "value of test prop", "type":"text", "propertyDefinitionId":"1245", "title": "Test Property"
			}]
		}]`))
		Expect(status).To(Equal(http.StatusOK))
	})
}
