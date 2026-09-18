package models

// FruitAttributes holds the mutable attributes of a fruit, shared by the stored
// record and the create payload so the field set is defined once.
type FruitAttributes struct {
	Fruit string `json:"fruit" binding:"required" example:"apple"`
	Color string `json:"color" binding:"required" example:"red"`
}

// Fruit is a fruit record stored in the fruits table.
type Fruit struct {
	ID int `json:"id" example:"1"`
	FruitAttributes
}

// CreateFruitRequest is the payload accepted by POST /fruits.
type CreateFruitRequest struct {
	FruitAttributes
}
