package main

import (
	"fmt"
	"log"

	"github.com/gin-gonic/gin"

	"firstgo-back/internal/config"
	"firstgo-back/internal/database"
	"firstgo-back/internal/handler"
	"firstgo-back/internal/middleware"
	"firstgo-back/internal/store"
)

func main() {
	cfg := config.Load()

	db, err := database.OpenSQLite(cfg.SQLitePath)
	if err != nil {
		log.Fatalf("database: %v", err)
	}
	defer db.Close()

	userStore := store.NewUserStore(db)
	authHandler := handler.NewAuthHandler(cfg, userStore)

	r := gin.Default()
	r.Use(middleware.CORS(cfg.AllowOrigin))

	// 添加根路径，避免浏览器直接访问时 404
	r.GET("/", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"service": "firstgo-back",
			"status":  "running",
		})
	})

	r.GET("/health", handler.Health)

	api := r.Group("/api")
	{
		authGroup := api.Group("/auth")
		{
			authGroup.POST("/login", authHandler.Login)
			authGroup.POST("/register", authHandler.Register)
			authGroup.GET("/me", middleware.JWTAuth(cfg.JWTSecret), authHandler.Me)
		}
	}

	addr := fmt.Sprintf(":%s", cfg.Port)
	log.Printf("server listening on %s", addr)
	if err := r.Run(addr); err != nil {
		log.Fatal(err)
	}
}