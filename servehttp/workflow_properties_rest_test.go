package servehttp_test

import (
	"bytes"
	"errors"
	"flywheel/bizerror"
	"flywheel/domain"
	"flywheel/domain/flow"
	"flywheel/servehttp"
	"flywheel/session"
	"flywheel/testinfra"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/fundwit/go-commons/types"
	"github.com/gin-gonic/gin"
	. "github.com/onsi/gomega"
)

func TestCreateWorkflowPropertyRestAPI(t *testing.T) {
	RegisterTestingT(t)

	router := gin.Default()
	router.Use(bizerror.ErrorHandling())
	servehttp.RegisterWorkflowHandler(router)

	t.Run("should be able to handle bind error", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/v1/workflows/bad/properties", nil)
		status, body, _ := testinfra.ExecuteRequest(req, router)
		Expect(status).To(Equal(http.StatusBadRequest))
		Expect(body).To(MatchJSON(`{"code":"common.bad_param","message":"invalid id 'bad'","data":null}`))

		req = httptest.NewRequest(http.MethodPost, "/v1/workflows/100/properties", bytes.NewReader([]byte(`bad json`)))
		status, body, _ = testinfra.ExecuteRequest(req, router)
		Expect(status).To(Equal(http.StatusBadRequest))
		Expect(body).To(MatchJSON(`{"code":"common.bad_param","message":"invalid character 'b' looking for beginning of value","data":null}`))
	})

	t.Run("should be able to handle validate error", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/v1/workflows/100/properties", bytes.NewReader([]byte(`{}`)))
		status, body, _ := testinfra.ExecuteRequest(req, router)
		Expect(status).To(Equal(http.StatusBadRequest))
		Expect(body).To(MatchJSON(`{"code":"common.bad_param","message":
			"Key: 'PropertyDefinition.Name' Error:Field validation for 'Name' failed on the 'required' tag\n` +
			`Key: 'PropertyDefinition.Type' Error:Field validation for 'Type' failed on the 'required' tag","data":null}`))
	})

	t.Run("should be able to handle service error", func(t *testing.T) {
		flow.CreatePropertyDefinitionFunc = func(workflowId types.ID, p domain.PropertyDefinition, s *session.Session) (*flow.WorkflowPropertyDefinition, error) {
			return nil, errors.New("a mocked error")
		}
		req := httptest.NewRequest(http.MethodPost, "/v1/workflows/100/properties", bytes.NewReader([]byte(
			`{"name":"test", "type": "text", "title": "Test"}`)))
		status, body, _ := testinfra.ExecuteRequest(req, router)
		Expect(status).To(Equal(http.StatusInternalServerError))
		Expect(body).To(MatchJSON(`{"code":"common.internal_server_error","message":"a mocked error","data":null}`))
	})

	t.Run("should be able to create successfully", func(t *testing.T) {
		flow.CreatePropertyDefinitionFunc = func(workflowId types.ID, p domain.PropertyDefinition, s *session.Session) (*flow.WorkflowPropertyDefinition, error) {
			return &flow.WorkflowPropertyDefinition{ID: 123, WorkflowID: workflowId, PropertyDefinition: p}, nil
		}

		req := httptest.NewRequest(http.MethodPost, "/v1/workflows/100/properties", bytes.NewReader([]byte(
			`{"name":"test", "type": "text", "title": "Test"}`)))
		status, body, _ := testinfra.ExecuteRequest(req, router)
		Expect(status).To(Equal(http.StatusCreated))
		Expect(body).To(MatchJSON(`{"id":"123", "workflowId":"100", "name":"test", "type": "text", "title": "Test"}`))
	})
}

func TestQueryWorkflowPropertiesRestAPI(t *testing.T) {
	RegisterTestingT(t)

	router := gin.Default()
	router.Use(bizerror.ErrorHandling())
	servehttp.RegisterWorkflowHandler(router)

	t.Run("should be able to handle bind error", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/v1/workflows/bad/properties", nil)
		status, body, _ := testinfra.ExecuteRequest(req, router)
		Expect(status).To(Equal(http.StatusBadRequest))
		Expect(body).To(MatchJSON(`{"code":"common.bad_param","message":"invalid id 'bad'","data":null}`))
	})

	t.Run("should be able to handle service error", func(t *testing.T) {
		flow.QueryPropertyDefinitionsFunc = func(workflowId types.ID, s *session.Session) ([]flow.WorkflowPropertyDefinition, error) {
			return nil, errors.New("a mocked error")
		}
		req := httptest.NewRequest(http.MethodGet, "/v1/workflows/100/properties", nil)
		status, body, _ := testinfra.ExecuteRequest(req, router)
		Expect(status).To(Equal(http.StatusInternalServerError))
		Expect(body).To(MatchJSON(`{"code":"common.internal_server_error","message":"a mocked error","data":null}`))
	})

	t.Run("should be able to query result successfully", func(t *testing.T) {
		flow.QueryPropertyDefinitionsFunc = func(workflowId types.ID, s *session.Session) ([]flow.WorkflowPropertyDefinition, error) {
			return []flow.WorkflowPropertyDefinition{
				{ID: 101, WorkflowID: workflowId, PropertyDefinition: domain.PropertyDefinition{Name: "prop1", Type: "string", Title: "Prop1"}},
				{ID: 102, WorkflowID: workflowId, PropertyDefinition: domain.PropertyDefinition{Name: "prop2", Type: "string", Title: "Prop2"}},
			}, nil
		}

		req := httptest.NewRequest(http.MethodGet, "/v1/workflows/100/properties", nil)
		status, body, _ := testinfra.ExecuteRequest(req, router)
		Expect(status).To(Equal(http.StatusOK))
		Expect(body).To(MatchJSON(`[
			{"id":"101", "workflowId":"100", "name":"prop1", "type": "string", "title": "Prop1"},
			{"id":"102", "workflowId":"100", "name":"prop2", "type": "string", "title": "Prop2"}
		]`))
	})
}

func TestDeleteWorkflowPropertiesRestAPI(t *testing.T) {
	RegisterTestingT(t)

	router := gin.Default()
	router.Use(bizerror.ErrorHandling())
	servehttp.RegisterWorkflowHandler(router)

	t.Run("should be able to handle bind error", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodDelete, "/v1/workflows/properties/bad", nil)
		status, body, _ := testinfra.ExecuteRequest(req, router)
		Expect(status).To(Equal(http.StatusBadRequest))
		Expect(body).To(MatchJSON(`{"code":"common.bad_param","message":"invalid id 'bad'","data":null}`))
	})

	t.Run("should be able to handle service error", func(t *testing.T) {
		flow.DeletePropertyDefinitionFunc = func(workflowId types.ID, s *session.Session) error {
			return errors.New("a mocked error")
		}
		req := httptest.NewRequest(http.MethodDelete, "/v1/workflows/properties/100", nil)
		status, body, _ := testinfra.ExecuteRequest(req, router)
		Expect(status).To(Equal(http.StatusInternalServerError))
		Expect(body).To(MatchJSON(`{"code":"common.internal_server_error","message":"a mocked error","data":null}`))
	})

	t.Run("should be able to delete successfully", func(t *testing.T) {
		flow.DeletePropertyDefinitionFunc = func(workflowId types.ID, s *session.Session) error {
			return nil
		}

		req := httptest.NewRequest(http.MethodDelete, "/v1/workflows/properties/100", nil)
		status, body, _ := testinfra.ExecuteRequest(req, router)
		Expect(status).To(Equal(http.StatusNoContent))
		Expect(body).To(BeZero())
	})
}
