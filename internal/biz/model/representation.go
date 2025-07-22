package model

type Representation struct {
	Data               JsonObject         `gorm:"type:jsonb;column:data"`
	RepresentationType RepresentationType `gorm:"-"`
}
