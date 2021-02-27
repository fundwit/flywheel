package common

import (
	"github.com/fundwit/go-commons/types"
	"github.com/sony/sonyflake"
)

func NextId(idWorker *sonyflake.Sonyflake) types.ID {
	id, err := idWorker.NextID()
	if err != nil {
		panic(err) // errors.New("over the time limit")
	}
	return types.ID(id)
}
