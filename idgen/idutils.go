package idgen

import (
	"github.com/fundwit/go-commons/types"
	"github.com/sony/sonyflake"
)

func NextID(idWorker *sonyflake.Sonyflake) types.ID {
	id, err := idWorker.NextID()
	if err != nil {
		panic(err)
	}
	return types.ID(id)
}
