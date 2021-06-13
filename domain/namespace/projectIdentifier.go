package namespace

import (
	"github.com/fundwit/go-chars/chars"
)

func RecommendProjectIdentifier(name string) string {
	return chars.Abbreviate(name, 3)
}
