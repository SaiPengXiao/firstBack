package handler

import (
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"firstgo-back/internal/auth"
	"firstgo-back/internal/config"
	"firstgo-back/internal/model"
	"firstgo-back/internal/store"
)

const tokenTTL = 24 * time.Hour

// AuthHandler handles login, register, and profile.
type AuthHandler struct {
	cfg       config.Config
	store     *store.UserStore
	permStore *store.PermissionStore
}

// NewAuthHandler creates an AuthHandler.
func NewAuthHandler(cfg config.Config, userStore *store.UserStore, permStore *store.PermissionStore) *AuthHandler {
	return &AuthHandler{cfg: cfg, store: userStore, permStore: permStore}
}

// attachRoles fills the user's Roles from the RBAC tables.
func (h *AuthHandler) attachRoles(user *model.User) {
	if user == nil {
		return
	}
	roles, err := h.permStore.GetRolesByUserID(user.ID)
	if err != nil || roles == nil {
		user.Roles = []string{}
		return
	}
	user.Roles = roles
}

// Register godoc
// POST /api/auth/register
func (h *AuthHandler) Register(c *gin.Context) {
	var req model.RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, model.ErrorResponse{Message: "请求参数无效"})
		return
	}

	user, err := h.store.Register(req.Username, req.Email, req.Password)
	if err != nil {
		if errors.Is(err, store.ErrUsernameTaken) {
			c.JSON(http.StatusConflict, model.ErrorResponse{Message: "用户名已被占用"})
			return
		}
		if errors.Is(err, store.ErrEmailTaken) {
			c.JSON(http.StatusConflict, model.ErrorResponse{Message: "邮箱已被注册"})
			return
		}
		c.JSON(http.StatusInternalServerError, model.ErrorResponse{Message: "注册失败"})
		return
	}

	token, err := auth.IssueToken(h.cfg.JWTSecret, user.ID, user.Username, tokenTTL)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.ErrorResponse{Message: "签发令牌失败"})
		return
	}

	h.attachRoles(&user)
	c.JSON(http.StatusOK, model.AuthResponse{User: user, Token: token})
}

// Login godoc
// POST /api/auth/login
func (h *AuthHandler) Login(c *gin.Context) {
	var req model.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, model.ErrorResponse{Message: "请输入用户名和密码"})
		return
	}

	user, err := h.store.Login(req.Username, req.Password)
	if err != nil {
		if errors.Is(err, store.ErrInvalidCredentials) {
			c.JSON(http.StatusUnauthorized, model.ErrorResponse{Message: "用户名或密码错误"})
			return
		}
		c.JSON(http.StatusInternalServerError, model.ErrorResponse{Message: "登录失败"})
		return
	}

	token, err := auth.IssueToken(h.cfg.JWTSecret, user.ID, user.Username, tokenTTL)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.ErrorResponse{Message: "签发令牌失败"})
		return
	}

	h.attachRoles(&user)
	c.JSON(http.StatusOK, model.AuthResponse{User: user, Token: token})
}

// Me godoc
// GET /api/auth/me
func (h *AuthHandler) Me(c *gin.Context) {
	userID, _ := c.Get("userID")
	id, _ := userID.(string)

	user, err := h.store.GetByID(id)
	if err != nil {
		c.JSON(http.StatusUnauthorized, model.ErrorResponse{Message: "未登录或用户不存在"})
		return
	}

	h.attachRoles(&user)
	c.JSON(http.StatusOK, user)
}

// Health godoc
// GET /health
func Health(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}