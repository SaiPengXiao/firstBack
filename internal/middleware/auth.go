package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"firstgo-back/internal/auth"
	"firstgo-back/internal/model"
	"firstgo-back/internal/store"
)

// JWTAuth validates Bearer tokens and sets userID on context.
func JWTAuth(secret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		header := c.GetHeader("Authorization")
		if header == "" {
			c.JSON(http.StatusUnauthorized, model.ErrorResponse{Message: "缺少 Authorization 头"})
			c.Abort()
			return
		}

		parts := strings.SplitN(header, " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			c.JSON(http.StatusUnauthorized, model.ErrorResponse{Message: "Authorization 格式错误"})
			c.Abort()
			return
		}

		claims, err := auth.ParseToken(secret, parts[1])
		if err != nil {
			c.JSON(http.StatusUnauthorized, model.ErrorResponse{Message: "令牌无效或已过期"})
			c.Abort()
			return
		}

		c.Set("userID", claims.UserID)
		c.Set("username", claims.Username)
		c.Next()
	}
}

// RequirePermission checks the user's permissions against the DB on every
// request (so role changes take effect immediately, without re-login). It must
// run after JWTAuth, which sets userID on the context.
func RequirePermission(permStore *store.PermissionStore, code string) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID, _ := c.Get("userID")
		id, ok := userID.(string)
		if !ok || id == "" {
			c.JSON(http.StatusUnauthorized, model.ErrorResponse{Message: "未登录"})
			c.Abort()
			return
		}

		ok, err := permStore.HasPermission(id, code)
		if err != nil {
			c.JSON(http.StatusInternalServerError, model.ErrorResponse{Message: "权限校验失败"})
			c.Abort()
			return
		}
		if !ok {
			c.JSON(http.StatusForbidden, model.ErrorResponse{Message: "无操作权限"})
			c.Abort()
			return
		}
		c.Next()
	}
}