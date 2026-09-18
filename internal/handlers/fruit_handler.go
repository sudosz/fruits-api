package handlers

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/sudosz/fruits-api/internal/models"
	"github.com/sudosz/fruits-api/internal/service"
)

// FruitHandler serves the /fruits routes and the health probe. It depends only
// on the service layer.
type FruitHandler struct {
	service service.FruitService
}

// NewFruitHandler returns a handler bound to the given service.
func NewFruitHandler(svc service.FruitService) *FruitHandler {
	return &FruitHandler{service: svc}
}

// RegisterRoutes wires every route this handler owns onto the router.
func (h *FruitHandler) RegisterRoutes(r gin.IRoutes) {
	r.GET("/healthz", h.Health)
	r.GET("/fruits", h.List)
	r.GET("/fruits/:id", h.Get)
	r.POST("/fruits", h.Create)
}

// List godoc
//
//	@Summary		List all fruits
//	@Description	Returns every fruit in the database, or an empty array when there are none.
//	@Tags			fruits
//	@Produce		json
//	@Success		200	{array}		models.Fruit
//	@Failure		500	{object}	map[string]string
//	@Router			/fruits [get]
func (h *FruitHandler) List(c *gin.Context) {
	fruits, err := h.service.List(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch fruits"})
		return
	}
	c.JSON(http.StatusOK, fruits)
}

// Get godoc
//
//	@Summary		Get a fruit by ID
//	@Description	Returns a single fruit.
//	@Tags			fruits
//	@Produce		json
//	@Param			id	path		int	true	"Fruit ID"
//	@Success		200	{object}	models.Fruit
//	@Failure		400	{object}	map[string]string
//	@Failure		404	{object}	map[string]string
//	@Failure		500	{object}	map[string]string
//	@Router			/fruits/{id} [get]
func (h *FruitHandler) Get(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	fruit, err := h.service.Get(c.Request.Context(), id)
	switch {
	case errors.Is(err, service.ErrNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "fruit not found"})
	case err != nil:
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch fruit"})
	default:
		c.JSON(http.StatusOK, fruit)
	}
}

// Create godoc
//
//	@Summary		Create a fruit
//	@Description	Adds a fruit and returns it with the generated ID.
//	@Tags			fruits
//	@Accept			json
//	@Produce		json
//	@Param			fruit	body		models.CreateFruitRequest	true	"Fruit to create"
//	@Success		201		{object}	models.Fruit
//	@Failure		400		{object}	map[string]string
//	@Failure		500		{object}	map[string]string
//	@Router			/fruits [post]
func (h *FruitHandler) Create(c *gin.Context) {
	var req models.CreateFruitRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "fruit and color are required"})
		return
	}

	fruit, err := h.service.Create(c.Request.Context(), req)
	switch {
	case errors.Is(err, service.ErrInvalidInput):
		c.JSON(http.StatusBadRequest, gin.H{"error": "fruit and color must not be empty"})
	case err != nil:
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create fruit"})
	default:
		c.JSON(http.StatusCreated, fruit)
	}
}

// Health godoc
//
//	@Summary		Health check
//	@Description	Reports service readiness, including database connectivity.
//	@Tags			operations
//	@Produce		json
//	@Success		200	{object}	map[string]string
//	@Failure		503	{object}	map[string]string
//	@Router			/healthz [get]
func (h *FruitHandler) Health(c *gin.Context) {
	if err := h.service.Health(c.Request.Context()); err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "database unreachable"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}
