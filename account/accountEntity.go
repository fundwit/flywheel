package account

import "github.com/fundwit/go-commons/types"

type User struct {
	ID     types.ID `json:"id"`
	Name   string   `json:"name"`
	Secret string   `json:"secret"`

	Nickname string `json:"nickname"`
}

type UserInfo struct {
	ID       types.ID `json:"id"`
	Name     string   `json:"name"`
	Nickname string   `json:"nickname"`
}

type BasicAuthUpdating struct {
	OriginalSecret string `json:"originalSecret"`
	NewSecret      string `json:"newSecret" binding:"required,gte=6,lte=32"`
}

type UserCreation struct {
	Name     string `json:"name" binding:"required,lte=32"`
	Secret   string `json:"secret" binding:"required,gte=6,lte=32"`
	Nickname string `json:"nickname" binding:"omitempty,gte=1,lte=32"`
}

type UserUpdation struct {
	Nickname string `json:"nickname" binding:"required,lte=32"`
}

func (u User) DisplayName() string {
	if u.Nickname != "" {
		return u.Nickname
	} else {
		return u.Name
	}
}

func (u UserInfo) DisplayName() string {
	if u.Nickname != "" {
		return u.Nickname
	} else {
		return u.Name
	}
}
