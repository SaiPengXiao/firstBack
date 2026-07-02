package main

import (
	"fmt"
	"log"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"

	"firstgo-back/internal/config"
	"firstgo-back/internal/database"
	"firstgo-back/internal/handler"
	"firstgo-back/internal/middleware"
	"firstgo-back/internal/store"
)

func main() {
	_ = godotenv.Load() // 读取 firstBack 目录下的 .env（可选，没有则只用系统环境变量）
	cfg := config.Load()

	db, err := database.Open(cfg)
	if err != nil {
		log.Fatalf("database (mysql): %v", err)
	}
	defer db.Close()

	userStore := store.NewUserStore(db)
	menuStore := store.NewMenuStore(db)
	authHandler := handler.NewAuthHandler(cfg, userStore)
	menuHandler := handler.NewMenuHandler(menuStore)

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

		// 菜单：GET 可匿名（点菜页）；写操作需登录
		api.GET("/menu", menuHandler.GetMenu)
		menuRead := api.Group("/menu")
		{
			menuRead.GET("/categories", menuHandler.ListCategories)
			menuRead.GET("/items", menuHandler.ListItems)
			menuRead.GET("/items/:id", menuHandler.GetItem)
		}
		menuWrite := api.Group("/menu")
		menuWrite.Use(middleware.JWTAuth(cfg.JWTSecret))
		{
			menuWrite.POST("/categories", menuHandler.CreateCategory)
			menuWrite.PUT("/categories/:id", menuHandler.UpdateCategory)
			menuWrite.DELETE("/categories/:id", menuHandler.DeleteCategory)
			menuWrite.POST("/items", menuHandler.CreateItem)
			menuWrite.PUT("/items/:id", menuHandler.UpdateItem)
			menuWrite.DELETE("/items/:id", menuHandler.DeleteItem)
		}
	}

	addr := fmt.Sprintf(":%s", cfg.Port)
	log.Printf("server listening on %s", addr)
	if err := r.Run(addr); err != nil {
		log.Fatal(err)
	}
}