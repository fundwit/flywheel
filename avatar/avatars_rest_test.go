package avatar

import (
	"bytes"
	"flywheel/bizerror"
	"flywheel/session"
	"flywheel/testinfra"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/fundwit/go-commons/types"
	"github.com/gin-gonic/gin"
)

func TestHandleGetAvatar(t *testing.T) {
	engine := gin.Default()
	engine.Use(bizerror.ErrorHandling())
	RegisterAvatarAPI(engine)

	DetailAvatarFunc = func(id types.ID, s *session.Session) ([]byte, error) {
		return []byte(id.String() + ":abcd"), nil
	}

	t.Run("Show be able to handle get avatar REST API", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, APIAccountAvatarsRoot+"/123", nil)
		status, body, resp := testinfra.ExecuteRequest(req, engine)
		if status != http.StatusOK || body != "123:abcd" || resp.Header.Get("CONTENT-TYPE") != "image/png" {
			t.Errorf("get avatar REST API of %v, returned: %v, %v, %v, wanted: %v, %v, %v",
				123,
				status, resp.Header.Get("CONTENT-TYPE"), body,
				http.StatusOK, "image/png", "123:abcd",
			)
		}
	})
}

func TestHandleCreateAvatar(t *testing.T) {
	engine := gin.Default()
	engine.Use(bizerror.ErrorHandling())
	RegisterAvatarAPI(engine)

	buff := &bytes.Buffer{}
	CreateAvatarFunc = func(id types.ID, r io.Reader, s *session.Session) error {
		if _, err := io.Copy(buff, r); err != nil {
			return err
		}
		return nil
	}

	t.Run("Show be able to handle create avatar REST API", func(t *testing.T) {
		data := "------WebKitFormBoundaryWdDAe6hxfa4nl2Ig\n" +
			"Content-Disposition: form-data; name=\"file\"; filename=\"out.png\"\n" +
			"Content-Type: image/png\n" +
			"\n" +
			"binary-data\n" +
			"------WebKitFormBoundaryWdDAe6hxfa4nl2Ig--"

		req := httptest.NewRequest(http.MethodPost, APIAccountAvatarsRoot+"/123", bytes.NewBufferString(data))
		req.Header.Set("CONTENT-TYPE", "multipart/form-data; boundary=----WebKitFormBoundaryWdDAe6hxfa4nl2Ig")
		status, _, _ := testinfra.ExecuteRequest(req, engine)
		if status != http.StatusOK {
			t.Errorf("get avatar REST API of %v, returned: %v, wanted: %v", 123, status, http.StatusOK)
		}
		all, _ := ioutil.ReadAll(buff)
		if string(all) != "binary-data" {
			t.Errorf("get avatar REST API of %v, updated: %v, wanted: %v", 123, all, "binary-data")
		}
	})
}
