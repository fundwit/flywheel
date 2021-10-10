package domain

type PropertyDefinition struct {
	Name string `json:"name" binding:"required" gorm:"unique_index:uni_workflow_prop"`
	Type string `json:"type" binding:"required,oneof=text number"`

	Title string `json:"title"`
}
