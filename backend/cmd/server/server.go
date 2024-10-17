package main

import (
	"log"
	"net/http"

	"github.com/dmarc-analyzer/dmarc-analyzer/backend/handler"
	"github.com/gin-contrib/cors"
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

	r.GET("/api/domains", handler.HandleDomainList)
	r.GET("/api/domains/:domain/report", handler.HandleDomainSummary)
	r.GET("/api/domains/:domain/report/detail", handler.HandleDmarcDetail)
	r.GET("/api/domains/:domain/chart/dmarc", handler.HandleDmarcChart)

	if err := http.ListenAndServe(":6767", r); err != nil {
		log.Fatalf("start server error: %+v", err)
	}
}
