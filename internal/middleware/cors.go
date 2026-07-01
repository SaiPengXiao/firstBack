package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// CORS allows the configured frontend origin.
func CORS(allowOrigin string) gin.HandlerFunc {
    return func(c *gin.Context) {
        // ✅ 你的 allowOrigin 来自环境变量，已经是具体域名，符合 Credentials 要求
        c.Header("Access-Control-Allow-Origin", allowOrigin)
        c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
        c.Header("Access-Control-Allow-Headers", "Origin, Content-Type, Accept, Authorization")
        c.Header("Access-Control-Max-Age", "86400")
        
        // ✅ 新增：支持携带凭证（Cookie / Authorization）
        c.Header("Access-Control-Allow-Credentials", "true")

        if c.Request.Method == http.MethodOptions {
            c.AbortWithStatus(http.StatusNoContent)
            return
        }
        c.Next()
    }
}