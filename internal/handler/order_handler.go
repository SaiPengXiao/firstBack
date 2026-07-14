package handler

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"firstgo-back/internal/model"
	"firstgo-back/internal/store"
)

// OrderHandler serves order create/list APIs.
type OrderHandler struct {
	store *store.OrderStore
}

// NewOrderHandler creates an OrderHandler.
func NewOrderHandler(orderStore *store.OrderStore) *OrderHandler {
	return &OrderHandler{store: orderStore}
}

// Create POST /api/orders — any logged-in user.
func (h *OrderHandler) Create(c *gin.Context) {
	var req model.CreateOrderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, model.ErrorResponse{Message: "请求参数无效"})
		return
	}
	uid, _ := c.Get("userID")
	userID, _ := uid.(string)
	if userID == "" {
		c.JSON(http.StatusUnauthorized, model.ErrorResponse{Message: "未登录"})
		return
	}
	order, err := h.store.Create(userID, req)
	if err != nil {
		if errors.Is(err, store.ErrMenuItemNotFound) {
			c.JSON(http.StatusBadRequest, model.ErrorResponse{Message: "菜品不存在"})
			return
		}
		if errors.Is(err, store.ErrMenuItemUnavailable) {
			c.JSON(http.StatusBadRequest, model.ErrorResponse{Message: "菜品已下架"})
			return
		}
		c.JSON(http.StatusInternalServerError, model.ErrorResponse{Message: "下单失败"})
		return
	}
	c.JSON(http.StatusCreated, order)
}

// List GET /api/orders — admin only (RequirePermission order:read on the route).
func (h *OrderHandler) List(c *gin.Context) {
	orders, err := h.store.List()
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.ErrorResponse{Message: "获取订单失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"orders": orders})
}

// ListMine GET /api/orders — any logged-in user, filtered to their own orders.
func (h *OrderHandler) ListMine(c *gin.Context) {
	uid, _ := c.Get("userID")
	userID, _ := uid.(string)
	if userID == "" {
		c.JSON(http.StatusUnauthorized, model.ErrorResponse{Message: "未登录"})
		return
	}
	orders, err := h.store.ListByUser(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.ErrorResponse{Message: "获取订单失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"orders": orders})
}
