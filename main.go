package main

import (
	"github.com/gin-gonic/gin"
	"log"
)

func setupRouter() *gin.Engine {
	router := gin.New()
	//router.Use(gin.Recovery())
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

	router.GET("/ping", PingHandler)
	// Тест логов и восстановления
	router.GET("/crash", CrashHandler)
	return router
}

func main() {
	router := setupRouter()
	router.Run(":8080")
}
func PingHandler(c *gin.Context) {
	c.String(200, "pong")
}
func CrashHandler(c *gin.Context) {
	log.Println("Вызов ошибуки")
	panic("Ошибка!")
}
