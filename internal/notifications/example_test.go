package notifications_test

import (
	"context"
	"fmt"
	"log"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ajitpratap0/cryptofunk/internal/notifications"
)

// Example_basicUsage demonstrates basic notification service usage
func Example_basicUsage() {
	ctx := context.Background()

	// In production, use real database connection
	// db, _ := pgxpool.New(ctx, "postgres://...")
	var db *pgxpool.Pool // nil for example

	// Initialize FCM backend (mock mode for development)
	backend, _ := notifications.NewFCMBackend(ctx, "")

	// Create notification service
	service := notifications.NewService(db, backend)
	defer service.Close()

	// Create notification
	notification := notifications.Notification{
		Type:     notifications.NotificationTypeTradeExecution,
		Title:    "Trade Executed",
		Body:     "Your BTC/USDT order has been filled",
		Data:     notifications.TradeNotificationData("order-123", "BTC/USDT", "BUY", 0.5, 50000.0),
		Priority: "high",
	}

	fmt.Println("Notification type:", notification.Type)
	fmt.Println("Title:", notification.Title)
	// Output:
	// Notification type: trade_execution
	// Title: Trade Executed
}

// Example_helperMethods demonstrates using notification helper methods
func Example_helperMethods() {
	ctx := context.Background()

	// Setup (mock for demonstration)
	backend, _ := notifications.NewFCMBackend(ctx, "")
	service := notifications.NewService(nil, backend)
	helper := notifications.NewHelper(service)

	// These would normally send notifications to users
	_ = helper
	_ = ctx

	// Example notification data
	tradeData := notifications.TradeNotificationData("order-123", "BTC/USDT", "BUY", 0.5, 50000.0)
	fmt.Println("Order ID:", tradeData["order_id"])
	fmt.Println("Symbol:", tradeData["symbol"])
	fmt.Println("Price:", tradeData["price"])

	// Output:
	// Order ID: order-123
	// Symbol: BTC/USDT
	// Price: 50000.00
}

// Example_preferences demonstrates notification preferences
func Example_preferences() {
	// Default preferences (all enabled)
	prefs := notifications.DefaultPreferences()
	fmt.Println("Trade executions:", prefs.TradeExecutions)
	fmt.Println("P&L alerts:", prefs.PnLAlerts)
	fmt.Println("Circuit breaker:", prefs.CircuitBreaker)
	fmt.Println("Consensus failures:", prefs.ConsensusFailures)

	// Check if specific type is enabled
	enabled := prefs.IsEnabled(notifications.NotificationTypeTradeExecution)
	fmt.Println("Trade notifications enabled:", enabled)

	// Output:
	// Trade executions: true
	// P&L alerts: true
	// Circuit breaker: true
	// Consensus failures: true
	// Trade notifications enabled: true
}

// Example_integrationWithOrchestrator shows how to integrate with the orchestrator
func Example_integrationWithOrchestrator() {
	// This example shows how to integrate notifications in the orchestrator

	// 1. Initialize the notification service in main()
	ctx := context.Background()
	backend, err := notifications.NewFCMBackend(ctx, "/path/to/fcm-credentials.json")
	if err != nil {
		log.Fatal(err)
	}

	// In production, connect to real database
	// db, _ := pgxpool.New(ctx, "postgres://...")
	var db *pgxpool.Pool

	service := notifications.NewService(db, backend)
	helper := notifications.NewHelper(service)

	// 2. Inject into orchestrator/components
	_ = helper

	// 3. Send notifications on events
	// Example: After trade execution
	// helper.SendTradeExecution(ctx, userID, orderID, symbol, side, quantity, price)

	// Example: On P&L threshold breach
	// helper.SendPnLAlert(ctx, userID, sessionID, pnlPercent, pnlAmount)

	// Example: Circuit breaker triggered
	// helper.SendCircuitBreakerAlert(ctx, userID, reason, threshold)

	// Example: Consensus failure
	// helper.SendConsensusFailure(ctx, userID, symbol, reason, agentCount)

	fmt.Println("Notification service initialized")
	// Output:
	// Notification service initialized
}

// Example_deviceRegistration demonstrates device registration flow
func Example_deviceRegistration() {
	ctx := context.Background()

	// Mock setup
	backend, _ := notifications.NewFCMBackend(ctx, "")
	service := notifications.NewService(nil, backend)

	// Simulate device registration (would require database in production)
	userID := "user-123"
	deviceToken := "fcm-device-token-here"
	platform := notifications.PlatformIOS

	fmt.Printf("Registering device for user %s\n", userID)
	fmt.Printf("Platform: %s\n", platform)
	fmt.Printf("Token length: %d\n", len(deviceToken))

	// In production:
	// err := service.RegisterDevice(ctx, userID, deviceToken, platform)
	// if err != nil {
	//     log.Error().Err(err).Msg("Failed to register device")
	// }

	_ = service

	// Output:
	// Registering device for user user-123
	// Platform: ios
	// Token length: 21
}
