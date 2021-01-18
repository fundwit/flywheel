package common

import (
	"github.com/fundwit/go-commons/types"
	"github.com/sony/sonyflake"
)

func NextId(idWorker *sonyflake.Sonyflake) types.ID {
	id, err := idWorker.NextID()
	if err != nil {
		panic(err)
	}
	return types.ID(id)
}
