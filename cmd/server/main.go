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
	"firstgo-back/internal/model"
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
	permStore := store.NewPermissionStore(db)
	authHandler := handler.NewAuthHandler(cfg, userStore, permStore)
	menuHandler := handler.NewMenuHandler(menuStore, permStore)
	orderStore := store.NewOrderStore(db)
	orderHandler := handler.NewOrderHandler(orderStore)
	imgProxyHandler := handler.NewImgProxyHandler()

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

	// 图片代理：小程序只配置本域，由后端拉取外部图片（公开接口，无鉴权）
	r.GET("/img-proxy", imgProxyHandler.Proxy)

	api := r.Group("/api")
	{
		authGroup := api.Group("/auth")
		{
			authGroup.POST("/login", authHandler.Login)
			authGroup.POST("/register", authHandler.Register)
			authGroup.GET("/me", middleware.JWTAuth(cfg.JWTSecret), authHandler.Me)
		}

		// 菜单：读取需登录（admin 看全部含未上架，普通用户只看上架）；写操作需对应权限
		menuRead := api.Group("/menu")
		menuRead.Use(middleware.JWTAuth(cfg.JWTSecret))
		{
			menuRead.GET("", menuHandler.GetMenu)
			menuRead.GET("/categories", menuHandler.ListCategories)
			menuRead.GET("/items", menuHandler.ListItems)
			menuRead.GET("/items/:id", menuHandler.GetItem)
		}
		menuWrite := api.Group("/menu")
		menuWrite.Use(middleware.JWTAuth(cfg.JWTSecret))
		{
			menuWrite.POST("/categories", middleware.RequirePermission(permStore, model.PermCategoryCreate), menuHandler.CreateCategory)
			menuWrite.PUT("/categories/:id", middleware.RequirePermission(permStore, model.PermCategoryUpdate), menuHandler.UpdateCategory)
			menuWrite.DELETE("/categories/:id", middleware.RequirePermission(permStore, model.PermCategoryDelete), menuHandler.DeleteCategory)
			menuWrite.POST("/items", middleware.RequirePermission(permStore, model.PermItemCreate), menuHandler.CreateItem)
			menuWrite.PUT("/items/:id", middleware.RequirePermission(permStore, model.PermItemUpdate), menuHandler.UpdateItem)
			menuWrite.DELETE("/items/:id", middleware.RequirePermission(permStore, model.PermItemDelete), menuHandler.DeleteItem)
		}

		// 订单：下单=登录即可；看单=仅管理员（order:read）
		orders := api.Group("/orders")
		orders.Use(middleware.JWTAuth(cfg.JWTSecret))
		{
			orders.POST("", orderHandler.Create)
			orders.GET("", middleware.RequirePermission(permStore, model.PermOrderRead), orderHandler.List)
		}
	}

	addr := fmt.Sprintf(":%s", cfg.Port)
	log.Printf("server listening on %s", addr)
	if err := r.Run(addr); err != nil {
		log.Fatal(err)
	}
}