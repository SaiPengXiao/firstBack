package main

import (
	"fmt"
	"log"

	"github.com/gin-gonic/gin"

	"firstgo-back/internal/config"
	"firstgo-back/internal/handler"
	"firstgo-back/internal/middleware"
	"firstgo-back/internal/store"
)

func main() {
	cfg := config.Load()

	userStore := store.NewUserStore()
	authHandler := handler.NewAuthHandler(cfg, userStore)

	r := gin.Default()
	r.Use(middleware.CORS(cfg.AllowOrigin))

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
	log.Printf("server listening on http://localhost%s", addr)
	if err := r.Run(addr); err != nil {
		log.Fatal(err)
	}
}