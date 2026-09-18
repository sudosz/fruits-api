package models

// Fruit is a fruit record stored in the fruits table.
type Fruit struct {
	ID    int    `json:"id" example:"1"`
	Fruit string `json:"fruit" example:"apple"`
	Color string `json:"color" example:"red"`
}

// CreateFruitRequest is the payload accepted by POST /fruits.
type CreateFruitRequest struct {
	Fruit string `json:"fruit" binding:"required" example:"banana"`
	Color string `json:"color" binding:"required" example:"yellow"`
}

// ErrorResponse is the body returned for every 4xx/5xx response.
type ErrorResponse struct {
	Error string `json:"error" example:"fruit not found"`
}

// HealthResponse is the body returned by GET /healthz.
type HealthResponse struct {
	Status string `json:"status" example:"ok"`
}
