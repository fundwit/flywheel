package servehttp

import (
	"errors"
	"flywheel/common"
	"flywheel/i18n"
	"fmt"
	"github.com/gin-gonic/gin"
)

func ErrorHandling() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer handle(c)
		c.Next()
	}
}

func handle(c *gin.Context) {
	if ret := recover(); ret != nil {
		err, ok := ret.(error)
		if !ok {
			err = errors.New(fmt.Sprintf("%s", ret))
		}
		handleError(err, c)
	} else {
		if err := c.Errors.Last(); err != nil {
			handleError(err, c)
		}
	}
}

func handleError(err error, c *gin.Context) {
	common.Log.Error(err)

	genericErr := err
	var ginErr *gin.Error
	if errors.As(err, &ginErr) {
		genericErr = ginErr.Err
	}

	if bizErr, ok := genericErr.(common.BizError); ok {
		respond := bizErr.Respond()
		c.JSON(respond.Status, &common.ErrorBody{Code: respond.Code, Message: respond.Message, Data: respond.Data})
		return
	}

	c.JSON(500, &common.ErrorBody{Code: i18n.CommonInternalServerError, Message: err.Error()})
}
