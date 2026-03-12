package main

import (
	"context"
	"log"
	"urlshortener/internal/api"
	"urlshortener/internal/config"
	"urlshortener/internal/repository"

	"github.com/gin-gonic/gin"
)

func setupRouter(handlers *api.Handlers) *gin.Engine {
	router := gin.New()

	// Middleware для восстановления после паники
	router.Use(func(c *gin.Context) {
		defer func() {
			if err := recover(); err != nil {
				url := c.Request.URL.String()
				method := c.Request.Method

				log.Printf("Паника на %s %s: %v", method, url, err)

				c.String(500, "Ошибка на %s %s: сервер сломался", method, url)
				c.Abort()
			}
		}()
		c.Next()
	})

	router.Use(gin.Logger())

	// API routes
	apiGroup := router.Group("/api")
	{
		apiGroup.GET("/links", handlers.ListLinks)
		apiGroup.POST("/links", handlers.CreateLink)
		apiGroup.GET("/links/:id", handlers.GetLink)
		apiGroup.PUT("/links/:id", handlers.UpdateLink)
		apiGroup.DELETE("/links/:id", handlers.DeleteLink)
	}

	// Redirect route
	router.GET("/r/:shortName", handlers.RedirectHandler)

	// Test routes
	router.GET("/ping", PingHandler)
	router.GET("/crash", CrashHandler)

	return router
}

func main() {
	cfg := config.Load()

	// Подключаемся к базе данных через наш репозиторий
	ctx := context.Background()
	repo, err := repository.New(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer repo.Close()

	// Создаем обработчики
	handlers := api.NewHandlers(repo, cfg)

	// Настраиваем роутер
	router := setupRouter(handlers)

	// Запускаем сервер
	if err := router.Run(":" + cfg.Port); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}

func PingHandler(c *gin.Context) {
	c.String(200, "pong")
}

func CrashHandler(c *gin.Context) {
	log.Println("Вызов ошибуки")
	panic("Ошибка!")
}
