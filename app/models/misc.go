package models

type Vector3F struct {
	X float32 `json:"x" bson:"x"`
	Y float32 `json:"y" bson:"y"`
	Z float32 `json:"z" bson:"z"`
}

type BoundingBox struct {
	Min Vector3F `json:"min" bson:"min"`
	Max Vector3F `json:"max" bson:"max"`
}
