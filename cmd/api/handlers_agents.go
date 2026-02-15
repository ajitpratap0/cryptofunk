package main

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Agent handlers
func (s *APIServer) handleListAgents(c *gin.Context) {
	ctx := c.Request.Context()

	// Query agent status from database
	agents, err := s.db.GetAllAgentStatuses(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "failed to retrieve agents",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"agents": agents,
		"count":  len(agents),
	})
}

func (s *APIServer) handleGetAgent(c *gin.Context) {
	name := c.Param("name")
	ctx := c.Request.Context()

	// Query specific agent
	agent, err := s.db.GetAgentStatus(ctx, name)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "agent not found",
			"name":  name,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"agent": agent,
	})
}

func (s *APIServer) handleGetAgentStatus(c *gin.Context) {
	name := c.Param("name")
	ctx := c.Request.Context()

	// Query agent status
	agent, err := s.db.GetAgentStatus(ctx, name)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "agent not found",
			"name":  name,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"name":           agent.Name,
		"type":           agent.Type,
		"status":         agent.Status,
		"last_heartbeat": agent.LastHeartbeat,
		"started_at":     agent.StartedAt,
		"total_signals":  agent.TotalSignals,
		"error_count":    agent.ErrorCount,
	})
}
