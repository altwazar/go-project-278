package main

import (
	"github.com/gin-gonic/gin"
	"log"
)

func main() {
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

	router.GET("/ping", func(c *gin.Context) {
		c.String(200, "pong")
	})
	// Тест логов и восстановления
	router.GET("/crash", func(c *gin.Context) {
		log.Println("Вызов ошибуки")
		panic("Ошибка!")
	})

	router.Run(":8080")
}
