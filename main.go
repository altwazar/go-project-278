// Входная точка сервиса.
// Из полученной конфигурации создаётся подключение к БД и запускется веб-сервис
package main

import (
	"context"
	"github.com/gin-gonic/gin"
	"log"
	"urlshortener/internal/api"
	"urlshortener/internal/config"
	"urlshortener/internal/repository"
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
	// Путь к базе данных, порт и url сервиса на выходе
	cfg := config.Load()

	ctx := context.Background()
	// Получаем пул для работы с базой
	repo, err := repository.New(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer repo.Close()

	// Создаем обработчики
	// repo передаём для функций работы с базой
	// cfg для url на выходе
	handlers := api.NewHandlers(repo, cfg)

	// Настраиваем роутер
	router := setupRouter(handlers)

	// Запускаем сервер
	if err := router.Run(":" + cfg.Port); err != nil {
		log.Printf("Failed to start server: %v", err)
	}
}

// Для теста доступности сервиса
func PingHandler(c *gin.Context) {
	c.String(200, "pong")
}

// Для теста обработки паники
func CrashHandler(_ *gin.Context) {
	log.Println("Вызов ошибуки")
	panic("Ошибка!")
}
