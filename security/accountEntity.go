package security

type BasicAuthUpdating struct {
	OriginalSecret string `json:"originalSecret"`
	NewSecret      string `json:"newSecret" binding:"required,gte=6"`
}

type UserCreation struct {
	Name   string `json:"name" binding:"required"`
	Secret string `json:"secret" binding:"required,gte=6"`
}
