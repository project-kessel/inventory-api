package model

type LocalInventoryToResource struct {
	// Our local resource id
	ResourceId uint64 `gorm:"primarykey"`
	Resource   Resource

	// Id in reporter's side
	LocalResourceId string `gorm:"primarykey"`
	ResourceType    string `gorm:"primarykey"`

	// Reporter identification
	// Todo: Do we need to keep the reporter_type or the reporter_id is enough?
	ReporterId   string `gorm:"primarykey"`
	ReporterType string `gorm:"primarykey"`
}
