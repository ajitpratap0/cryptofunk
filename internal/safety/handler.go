package safety

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// RegisterRoutes registers safety API endpoints on the given router group.
func RegisterRoutes(rg *gin.RouterGroup, guard *Guard) {
	safety := rg.Group("/safety")
	{
		safety.GET("/status", handleStatus(guard))
		safety.POST("/killswitch", handleEnableKillSwitch(guard))
		safety.DELETE("/killswitch", handleDisableKillSwitch(guard))
		safety.POST("/resume", handleResume(guard))
		safety.PUT("/limits", handleUpdateLimits(guard))
	}
}

func handleStatus(g *Guard) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.JSON(http.StatusOK, g.Status())
	}
}

func handleEnableKillSwitch(g *Guard) gin.HandlerFunc {
	return func(c *gin.Context) {
		g.EnableKillSwitch()
		c.JSON(http.StatusOK, gin.H{"kill_switch": true, "message": "kill switch enabled — all trading halted"})
	}
}

func handleDisableKillSwitch(g *Guard) gin.HandlerFunc {
	return func(c *gin.Context) {
		g.DisableKillSwitch()
		c.JSON(http.StatusOK, gin.H{"kill_switch": false, "message": "kill switch disabled — trading resumed"})
	}
}

func handleResume(g *Guard) gin.HandlerFunc {
	return func(c *gin.Context) {
		g.ResetConsecutiveLosses()
		c.JSON(http.StatusOK, gin.H{"message": "consecutive loss counter reset — trading resumed"})
	}
}

func handleUpdateLimits(g *Guard) gin.HandlerFunc {
	return func(c *gin.Context) {
		var lim Limits
		if err := c.ShouldBindJSON(&lim); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		g.LimitsConfig().SetGlobal(lim)
		c.JSON(http.StatusOK, gin.H{"message": "limits updated", "limits": lim})
	}
}
