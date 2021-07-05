package misc

import (
	"github.com/fundwit/go-commons/types"
	"github.com/gin-gonic/gin"
)

type requestPath struct {
	ID types.ID `uri:"id" binding:"required"`
}

func BindingPathID(c *gin.Context) (types.ID, error) {
	p := requestPath{}
	if err := c.ShouldBindUri(&p); err != nil {
		return 0, err
	}
	return p.ID, nil
}
