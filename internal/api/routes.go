package api

// setupRoutes configures all API routes
func (s *Server) setupRoutes() {
	// API v1 group
	v1 := s.router.Group("/api/v1")
	{
		// Status endpoints (T152)
		v1.GET("/status", s.handleGetStatus)
		v1.GET("/health", s.handleGetHealth)

		// Agent endpoints (T153) - TODO: Implement in next step
		agents := v1.Group("/agents")
		{
			agents.GET("", s.handleListAgents)
			agents.GET("/:name", s.handleGetAgent)
			agents.GET("/:name/status", s.handleGetAgentStatus)
		}

		// Position endpoints (T154) - TODO: Implement in next step
		positions := v1.Group("/positions")
		{
			positions.GET("", s.handleListPositions)
			positions.GET("/:symbol", s.handleGetPosition)
		}

		// Order endpoints (T155) - TODO: Implement in next step
		orders := v1.Group("/orders")
		{
			orders.GET("", s.handleListOrders)
			orders.GET("/:id", s.handleGetOrder)
			orders.POST("", s.handlePlaceOrder)
			orders.DELETE("/:id", s.handleCancelOrder)
		}

		// Control endpoints (T156) - TODO: Implement in next step
		trade := v1.Group("/trade")
		{
			trade.POST("/start", s.handleStartTrading)
			trade.POST("/stop", s.handleStopTrading)
			trade.POST("/pause", s.handlePauseTrading)
		}

		// Config endpoints (T157) - TODO: Implement in next step
		v1.GET("/config", s.handleGetConfig)
		v1.PATCH("/config", s.handleUpdateConfig)
	}

	// Root endpoint
	s.router.GET("/", s.handleRoot)
}
