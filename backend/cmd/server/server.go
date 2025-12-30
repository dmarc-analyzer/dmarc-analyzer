package main

import (
	"log"
	"net/http"

	"github.com/dmarc-analyzer/dmarc-analyzer/backend/handler"
	"github.com/gin-contrib/cors"
	"github.com/gin-contrib/static"
	"github.com/gin-gonic/gin"
)

func main() {

	r := gin.Default()

	corsConfig := cors.Config{
		AllowAllOrigins: true,
		AllowMethods:    []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:    []string{"Accept, Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization"},
	}
	r.Use(cors.New(corsConfig))
	r.Use(gin.Recovery())

	handler.RegisterRoutes(r)

	// Serve static files from the ./frontend/dist directory
	r.Use(static.Serve("/", static.LocalFile("./frontend/dist", false)))

	// Handle SPA routes - serve index.html for any non-API, non-asset routes
	r.GET("/report/*path", func(c *gin.Context) {
		c.File("./frontend/dist/index.html")
	})

	if err := http.ListenAndServe(":6767", r); err != nil {
		log.Fatalf("start server error: %+v", err)
	}
}
