package handlers

import (
	"database/sql"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/sudosz/fruits-api/internal/models"
)

// FruitHandler serves the /fruits routes and the health probe. It talks to
// PostgreSQL directly; the API is small enough that a repository layer would
// only add indirection.
type FruitHandler struct {
	DB *sql.DB
}

// New returns a handler bound to the given database pool.
func New(db *sql.DB) *FruitHandler {
	return &FruitHandler{DB: db}
}

// RegisterRoutes wires every route this handler owns onto the router.
func (h *FruitHandler) RegisterRoutes(r gin.IRoutes) {
	r.GET("/healthz", h.Health)
	r.GET("/fruits", h.ListFruits)
	r.GET("/fruits/:id", h.GetFruit)
	r.POST("/fruits", h.CreateFruit)
}

// ListFruits godoc
//
//	@Summary		List all fruits
//	@Description	Returns every fruit in the database, or an empty array when there are none.
//	@Tags			fruits
//	@Produce		json
//	@Success		200	{array}		models.Fruit
//	@Failure		500	{object}	models.ErrorResponse
//	@Router			/fruits [get]
func (h *FruitHandler) ListFruits(c *gin.Context) {
	rows, err := h.DB.QueryContext(c.Request.Context(),
		"SELECT id, fruit, color FROM fruits ORDER BY id")
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "failed to fetch fruits"})
		return
	}
	defer func() {
		// The rows are fully drained below; a close error only matters for logs.
		_ = rows.Close()
	}()

	// Non-nil slice so an empty table marshals to [] rather than null.
	fruits := make([]models.Fruit, 0)
	for rows.Next() {
		var f models.Fruit
		if err := rows.Scan(&f.ID, &f.Fruit, &f.Color); err != nil {
			c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "failed to read fruits"})
			return
		}
		fruits = append(fruits, f)
	}
	if err := rows.Err(); err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "failed to read fruits"})
		return
	}

	c.JSON(http.StatusOK, fruits)
}

// GetFruit godoc
//
//	@Summary		Get a fruit by ID
//	@Description	Returns a single fruit.
//	@Tags			fruits
//	@Produce		json
//	@Param			id	path		int	true	"Fruit ID"
//	@Success		200	{object}	models.Fruit
//	@Failure		400	{object}	models.ErrorResponse
//	@Failure		404	{object}	models.ErrorResponse
//	@Failure		500	{object}	models.ErrorResponse
//	@Router			/fruits/{id} [get]
func (h *FruitHandler) GetFruit(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid id"})
		return
	}

	var f models.Fruit
	err = h.DB.QueryRowContext(c.Request.Context(),
		"SELECT id, fruit, color FROM fruits WHERE id = $1", id,
	).Scan(&f.ID, &f.Fruit, &f.Color)

	switch {
	case errors.Is(err, sql.ErrNoRows):
		c.JSON(http.StatusNotFound, models.ErrorResponse{Error: "fruit not found"})
	case err != nil:
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "failed to fetch fruit"})
	default:
		c.JSON(http.StatusOK, f)
	}
}

// CreateFruit godoc
//
//	@Summary		Create a fruit
//	@Description	Adds a fruit and returns it with the generated ID.
//	@Tags			fruits
//	@Accept			json
//	@Produce		json
//	@Param			fruit	body		models.CreateFruitRequest	true	"Fruit to create"
//	@Success		201		{object}	models.Fruit
//	@Failure		400		{object}	models.ErrorResponse
//	@Failure		500		{object}	models.ErrorResponse
//	@Router			/fruits [post]
func (h *FruitHandler) CreateFruit(c *gin.Context) {
	var req models.CreateFruitRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "fruit and color are required"})
		return
	}

	req.Fruit = strings.TrimSpace(req.Fruit)
	req.Color = strings.TrimSpace(req.Color)
	if req.Fruit == "" || req.Color == "" {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "fruit and color must not be empty"})
		return
	}

	fruit := models.Fruit{Fruit: req.Fruit, Color: req.Color}
	err := h.DB.QueryRowContext(c.Request.Context(),
		"INSERT INTO fruits (fruit, color) VALUES ($1, $2) RETURNING id",
		fruit.Fruit, fruit.Color,
	).Scan(&fruit.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "failed to create fruit"})
		return
	}

	c.JSON(http.StatusCreated, fruit)
}

// Health godoc
//
//	@Summary		Health check
//	@Description	Reports service readiness, including database connectivity.
//	@Tags			operations
//	@Produce		json
//	@Success		200	{object}	models.HealthResponse
//	@Failure		503	{object}	models.ErrorResponse
//	@Router			/healthz [get]
func (h *FruitHandler) Health(c *gin.Context) {
	if err := h.DB.PingContext(c.Request.Context()); err != nil {
		c.JSON(http.StatusServiceUnavailable, models.ErrorResponse{Error: "database unreachable"})
		return
	}
	c.JSON(http.StatusOK, models.HealthResponse{Status: "ok"})
}
