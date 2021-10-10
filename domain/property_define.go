package domain

type PropertyDefinition struct {
	Name string `json:"name" binding:"required"`
	Type string `json:"type" binding:"required,oneof=string"`

	Title string `json:"title"`
}
