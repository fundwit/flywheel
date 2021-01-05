package security

import "github.com/fundwit/go-commons/types"

type Context struct {
	Token    string   `json:"token"`
	Identity Identity `json:"identity"`
}

type Identity struct {
	ID   types.ID `json:"id"`
	Name string   `json:"name"`
}
