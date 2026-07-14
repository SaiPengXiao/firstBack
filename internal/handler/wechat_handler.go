package handler

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"firstgo-back/internal/auth"
	"firstgo-back/internal/config"
	"firstgo-back/internal/model"
	"firstgo-back/internal/store"
	"firstgo-back/internal/wechat"
)

const wechatTokenTTL = 7 * 24 * time.Hour

// WechatHandler handles WeChat Mini Program login.
type WechatHandler struct {
	cfg       config.Config
	wechat    *wechat.Client
	userStore *store.UserStore
}

// NewWechatHandler creates a WechatHandler.
func NewWechatHandler(cfg config.Config, wechatClient *wechat.Client, userStore *store.UserStore) *WechatHandler {
	return &WechatHandler{cfg: cfg, wechat: wechatClient, userStore: userStore}
}

// Login POST /api/auth/wechat-login
func (h *WechatHandler) Login(c *gin.Context) {
	var req model.WechatLoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, model.ErrorResponse{
			Code:    "INVALID_PARAMS",
			Message: "微信登录凭证不能为空",
		})
		return
	}

	code := strings.TrimSpace(req.Code)
	if code == "" {
		c.JSON(http.StatusBadRequest, model.ErrorResponse{
			Code:    "INVALID_PARAMS",
			Message: "微信登录凭证不能为空",
		})
		return
	}

	session, err := h.wechat.CodeToSession(c.Request.Context(), code)
	if err != nil {
		if errors.Is(err, wechat.ErrNotConfigured) {
			c.JSON(http.StatusNotImplemented, model.ErrorResponse{
				Code:    "NOT_CONFIGURED",
				Message: "微信登录未配置",
			})
			return
		}
		c.JSON(http.StatusUnauthorized, model.ErrorResponse{
			Code:    "WECHAT_LOGIN_FAILED",
			Message: "微信登录失败，请重试",
		})
		return
	}

	user, err := h.userStore.FindOrCreateByOpenID(session.OpenID, session.UnionID)
	if err != nil {
		if errors.Is(err, store.ErrUserDisabled) {
			c.JSON(http.StatusForbidden, model.ErrorResponse{
				Code:    "USER_DISABLED",
				Message: "该用户已被禁用",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, model.ErrorResponse{Message: "登录失败"})
		return
	}

	displayName := "顾客" + user.ID[:5]
	token, err := auth.IssueToken(h.cfg.JWTSecret, user.ID, displayName, wechatTokenTTL)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.ErrorResponse{Message: "签发令牌失败"})
		return
	}

	c.JSON(http.StatusOK, model.WechatLoginResponse{
		Token:     token,
		ExpiresIn: int(wechatTokenTTL.Seconds()),
		User: model.WechatUserInfo{
			ID:          user.ID,
			DisplayName: displayName,
		},
	})
}
