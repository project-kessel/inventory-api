package model

type LocalInventoryToResource struct {
	// Our local resource id
	ResourceId uint64 `gorm:"primarykey"`
	Resource   Resource

	// Id in reporter's side
	LocalResourceId string `gorm:"primarykey"`
	ResourceType    string `gorm:"primarykey"`

	// Reporter identification
	ReporterId string `gorm:"primarykey"`
}
