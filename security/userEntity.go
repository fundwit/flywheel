package security

type BasicAuthUpdating struct {
	OriginalSecret string `json:"originalSecret"`
	NewSecret      string `json:"newSecret" binding:"required,gte=6"`
}
