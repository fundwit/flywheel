package testinfra

import (
	"github.com/gin-gonic/gin"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
)

func ExecuteRequest(req *http.Request, engine *gin.Engine) (int, string, *http.Response) {
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, req)
	resp := w.Result()
	defer func() {
		_ = resp.Body.Close()
	}()
	bodyBytes, _ := ioutil.ReadAll(resp.Body)
	return resp.StatusCode, string(bodyBytes), resp
}
