package handlers

import (
	"io"
	"net/http"

	"github.com/gclm/octopus/internal/assets"
	"github.com/gclm/octopus/internal/server/router"
	"github.com/gclm/octopus/internal/utils/log"
	"github.com/gin-gonic/gin"
)

func GetProviders(c *gin.Context) {
	file, err := assets.ProvidersFS.Open("providers.json")
	if err != nil {
		log.Errorf("Failed to open embedded providers.json: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load providers"})
		return
	}
	defer file.Close()

	data, err := io.ReadAll(file)
	if err != nil {
		log.Errorf("Failed to read embedded providers.json: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load providers"})
		return
	}

	c.Data(http.StatusOK, "application/json", data)
}

func init() {
	router.NewGroupRouter("/api/v1/providers").
		AddRoute(
			router.NewRoute("", http.MethodGet).Handle(GetProviders),
		)
}
