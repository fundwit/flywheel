package security

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
