package handler

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"firstgo-back/internal/model"
	"firstgo-back/internal/store"
)

// MenuHandler serves menu read/write APIs.
type MenuHandler struct {
	store     *store.MenuStore
	permStore *store.PermissionStore
}

// NewMenuHandler creates a MenuHandler.
func NewMenuHandler(menuStore *store.MenuStore, permStore *store.PermissionStore) *MenuHandler {
	return &MenuHandler{store: menuStore, permStore: permStore}
}

// canSeeAll reports whether the current user may view unavailable items.
// Granted via the menu:read permission (held by admin). Basic logged-in users
// see only available items.
func (h *MenuHandler) canSeeAll(c *gin.Context) bool {
	uid, _ := c.Get("userID")
	id, _ := uid.(string)
	if id == "" {
		return false
	}
	ok, err := h.permStore.HasPermission(id, model.PermMenuRead)
	if err != nil {
		return false
	}
	return ok
}

// GetMenu GET /api/menu — categories with nested items.
// Admin (menu:read) sees all items incl. unavailable; others see available only.
func (h *MenuHandler) GetMenu(c *gin.Context) {
	data, err := h.store.ListMenu(h.canSeeAll(c))
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.ErrorResponse{Message: "获取菜单失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"categories": data})
}

// ListCategories GET /api/menu/categories
func (h *MenuHandler) ListCategories(c *gin.Context) {
	list, err := h.store.ListCategories()
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.ErrorResponse{Message: "获取分类失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"categories": list})
}

// CreateCategory POST /api/menu/categories
func (h *MenuHandler) CreateCategory(c *gin.Context) {
	var req model.CreateCategoryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, model.ErrorResponse{Message: "请求参数无效"})
		return
	}
	cat, err := h.store.CreateCategory(req.Name, req.SortOrder)
	if err != nil {
		if err.Error() == "category name already exists" {
			c.JSON(http.StatusConflict, model.ErrorResponse{Message: "分类名称已存在"})
			return
		}
		c.JSON(http.StatusInternalServerError, model.ErrorResponse{Message: "创建分类失败"})
		return
	}
	c.JSON(http.StatusCreated, cat)
}

// UpdateCategory PUT /api/menu/categories/:id
func (h *MenuHandler) UpdateCategory(c *gin.Context) {
	id := c.Param("id")
	var req model.UpdateCategoryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, model.ErrorResponse{Message: "请求参数无效"})
		return
	}
	if req.Name == nil && req.SortOrder == nil {
		c.JSON(http.StatusBadRequest, model.ErrorResponse{Message: "请提供要更新的字段"})
		return
	}
	cat, err := h.store.UpdateCategory(id, req.Name, req.SortOrder)
	if err != nil {
		if errors.Is(err, store.ErrCategoryNotFound) {
			c.JSON(http.StatusNotFound, model.ErrorResponse{Message: "分类不存在"})
			return
		}
		if err.Error() == "category name already exists" {
			c.JSON(http.StatusConflict, model.ErrorResponse{Message: "分类名称已存在"})
			return
		}
		c.JSON(http.StatusInternalServerError, model.ErrorResponse{Message: "更新分类失败"})
		return
	}
	c.JSON(http.StatusOK, cat)
}

// DeleteCategory DELETE /api/menu/categories/:id
func (h *MenuHandler) DeleteCategory(c *gin.Context) {
	id := c.Param("id")
	if err := h.store.DeleteCategory(id); err != nil {
		if errors.Is(err, store.ErrCategoryNotFound) {
			c.JSON(http.StatusNotFound, model.ErrorResponse{Message: "分类不存在"})
			return
		}
		c.JSON(http.StatusInternalServerError, model.ErrorResponse{Message: "删除分类失败，请先删除该分类下的菜品"})
		return
	}
	c.Status(http.StatusNoContent)
}

// ListItems GET /api/menu/items — optional ?categoryId=; availability by role.
func (h *MenuHandler) ListItems(c *gin.Context) {
	includeAll := h.canSeeAll(c)
	categoryID := c.Query("categoryId")
	var (
		list []model.MenuItem
		err  error
	)
	if categoryID != "" {
		list, err = h.store.ListItemsByCategory(categoryID, includeAll)
	} else {
		list, err = h.store.ListItems(includeAll)
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.ErrorResponse{Message: "获取菜品失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": list})
}

// GetItem GET /api/menu/items/:id
func (h *MenuHandler) GetItem(c *gin.Context) {
	id := c.Param("id")
	item, err := h.store.GetItemByID(id)
	if err != nil {
		if errors.Is(err, store.ErrMenuItemNotFound) {
			c.JSON(http.StatusNotFound, model.ErrorResponse{Message: "菜品不存在"})
			return
		}
		c.JSON(http.StatusInternalServerError, model.ErrorResponse{Message: "获取菜品失败"})
		return
	}
	// Non-admin may not view unavailable items.
	if !item.IsAvailable && !h.canSeeAll(c) {
		c.JSON(http.StatusNotFound, model.ErrorResponse{Message: "菜品不存在"})
		return
	}
	c.JSON(http.StatusOK, item)
}

// CreateItem POST /api/menu/items
func (h *MenuHandler) CreateItem(c *gin.Context) {
	var req model.CreateMenuItemRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, model.ErrorResponse{Message: "请求参数无效"})
		return
	}
	item, err := h.store.CreateItem(req)
	if err != nil {
		if errors.Is(err, store.ErrCategoryNotFound) {
			c.JSON(http.StatusBadRequest, model.ErrorResponse{Message: "分类不存在"})
			return
		}
		c.JSON(http.StatusInternalServerError, model.ErrorResponse{Message: "创建菜品失败"})
		return
	}
	c.JSON(http.StatusCreated, item)
}

// UpdateItem PUT /api/menu/items/:id
func (h *MenuHandler) UpdateItem(c *gin.Context) {
	id := c.Param("id")
	var req model.UpdateMenuItemRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, model.ErrorResponse{Message: "请求参数无效"})
		return
	}
	item, err := h.store.UpdateItem(id, req)
	if err != nil {
		if errors.Is(err, store.ErrMenuItemNotFound) {
			c.JSON(http.StatusNotFound, model.ErrorResponse{Message: "菜品不存在"})
			return
		}
		if errors.Is(err, store.ErrCategoryNotFound) {
			c.JSON(http.StatusBadRequest, model.ErrorResponse{Message: "分类不存在"})
			return
		}
		c.JSON(http.StatusInternalServerError, model.ErrorResponse{Message: "更新菜品失败"})
		return
	}
	c.JSON(http.StatusOK, item)
}

// DeleteItem DELETE /api/menu/items/:id
func (h *MenuHandler) DeleteItem(c *gin.Context) {
	id := c.Param("id")
	if err := h.store.DeleteItem(id); err != nil {
		if errors.Is(err, store.ErrMenuItemNotFound) {
			c.JSON(http.StatusNotFound, model.ErrorResponse{Message: "菜品不存在"})
			return
		}
		c.JSON(http.StatusInternalServerError, model.ErrorResponse{Message: "删除菜品失败"})
		return
	}
	c.Status(http.StatusNoContent)
}