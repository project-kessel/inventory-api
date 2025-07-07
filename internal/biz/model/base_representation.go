package model

type BaseRepresentation struct {
	Data JsonObject `gorm:"type:jsonb;column:data"`
}
