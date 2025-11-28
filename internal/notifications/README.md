# Push Notification Infrastructure

This package provides a flexible push notification infrastructure for CryptoFunk with pluggable backends.

## Features

- **Pluggable Backends**: Start with Firebase Cloud Messaging (FCM), easily extend to APNs, web push, etc.
- **User Preferences**: Fine-grained control over notification types
- **Device Management**: Track and manage multiple devices per user
- **Notification Logging**: Audit trail of all sent notifications
- **Mock Support**: Development-friendly mock backend when FCM credentials aren't configured

## Architecture

```
NotificationService (service.go)
    ├─ Backend Interface (pluggable)
    │   └─ FCMBackend (fcm.go)
    ├─ Database Layer (user_devices, notification_preferences, notification_log)
    └─ Helper Methods (helpers.go)
```

## Database Schema

### user_devices
Stores device tokens for push notifications.

| Column | Type | Description |
|--------|------|-------------|
| id | UUID | Primary key |
| user_id | UUID | Foreign key to users |
| device_token | TEXT | FCM/APNs device token |
| platform | VARCHAR(20) | ios, android, or web |
| enabled | BOOLEAN | Whether device is active |
| created_at | TIMESTAMP | Registration time |
| last_used_at | TIMESTAMP | Last notification sent |

### notification_preferences
User preferences for different notification types.

| Column | Type | Description |
|--------|------|-------------|
| user_id | UUID | Primary key, foreign key to users |
| trade_executions | BOOLEAN | Trade execution notifications |
| pnl_alerts | BOOLEAN | P&L change alerts |
| circuit_breaker | BOOLEAN | Circuit breaker triggers |
| consensus_failures | BOOLEAN | Agent consensus failures |

### notification_log
Audit log of all sent notifications (TimescaleDB hypertable).

| Column | Type | Description |
|--------|------|-------------|
| id | UUID | Primary key |
| user_id | UUID | Recipient user |
| device_token | TEXT | Target device |
| notification_type | VARCHAR(50) | Type of notification |
| title | TEXT | Notification title |
| body | TEXT | Notification body |
| data | JSONB | Additional payload |
| status | VARCHAR(20) | pending, sent, or failed |
| error_message | TEXT | Error if failed |
| sent_at | TIMESTAMP | When sent |

## Usage

### Initialize the Service

```go
package main

import (
    "context"
    "github.com/ajitpratap0/cryptofunk/internal/notifications"
    "github.com/jackc/pgx/v5/pgxpool"
)

func main() {
    ctx := context.Background()

    // Connect to database
    db, _ := pgxpool.New(ctx, "postgres://...")
    defer db.Close()

    // Initialize FCM backend (uses mock if no credentials)
    backend, _ := notifications.NewFCMBackend(ctx, "/path/to/fcm-credentials.json")

    // Create notification service
    service := notifications.NewService(db, backend)
    defer service.Close()
}
```

### Register a Device

```go
err := service.RegisterDevice(ctx, userID, deviceToken, notifications.PlatformIOS)
if err != nil {
    log.Error().Err(err).Msg("Failed to register device")
}
```

### Send Notifications

#### Using the Service Directly

```go
notification := notifications.Notification{
    Type:     notifications.NotificationTypeTradeExecution,
    Title:    "Trade Executed",
    Body:     "Your BTC/USDT order has been filled",
    Data: map[string]string{
        "order_id": "12345",
        "symbol":   "BTC/USDT",
    },
    Priority: "high",
}

err := service.SendToUser(ctx, userID, notification)
```

#### Using Helper Methods

```go
helper := notifications.NewHelper(service)

// Trade execution
helper.SendTradeExecution(ctx, userID, "order-123", "BTC/USDT", "BUY", 0.5, 50000.0)

// P&L alert (when change exceeds 5%)
helper.SendPnLAlert(ctx, userID, "session-456", 7.5, 1500.0)

// Circuit breaker triggered
helper.SendCircuitBreakerAlert(ctx, userID, "max_drawdown_exceeded", 10.0)

// Consensus failure
helper.SendConsensusFailure(ctx, userID, "ETH/USDT", "insufficient_confidence", 5)
```

### Manage User Preferences

```go
// Update preferences
prefs := notifications.Preferences{
    TradeExecutions:   true,
    PnLAlerts:         true,
    CircuitBreaker:    true,
    ConsensusFailures: false, // User opts out
}
err := service.UpdatePreferences(ctx, userID, prefs)

// Get current preferences
prefs, err := service.GetPreferences(ctx, userID)
```

### Integration Points

#### 1. Trade Execution (in order execution code)

```go
// In internal/exchange/executor.go or similar
func (e *Executor) notifyTradeExecution(order *Order) {
    helper := notifications.NewHelper(e.notificationService)
    helper.SendTradeExecution(
        context.Background(),
        order.UserID,
        order.ID,
        order.Symbol,
        order.Side,
        order.Quantity,
        order.FilledPrice,
    )
}
```

#### 2. P&L Monitoring (in position management)

```go
// In internal/positions/manager.go or similar
func (m *Manager) checkPnLThreshold(userID, sessionID string, oldPnL, newPnL float64) {
    helper := notifications.NewHelper(m.notificationService)
    helper.CheckPnLThresholdAndNotify(
        context.Background(),
        userID,
        sessionID,
        oldPnL,
        newPnL,
        5.0, // 5% threshold
    )
}
```

#### 3. Circuit Breaker (in risk management)

```go
// In internal/risk/circuit_breaker.go or similar
func (cb *CircuitBreaker) triggerBreaker(reason string) {
    helper := notifications.NewHelper(cb.notificationService)

    // Notify all active users
    users := cb.getActiveUsers()
    for _, userID := range users {
        helper.SendCircuitBreakerAlert(
            context.Background(),
            userID,
            reason,
            cb.threshold,
        )
    }
}
```

#### 4. Consensus Failures (in orchestrator)

```go
// In internal/orchestrator/consensus.go or similar
func (o *Orchestrator) handleConsensusFailure(symbol, reason string, agentCount int) {
    helper := notifications.NewHelper(o.notificationService)

    // Notify subscribed users
    users := o.getSubscribedUsers(symbol)
    for _, userID := range users {
        helper.SendConsensusFailure(
            context.Background(),
            userID,
            symbol,
            reason,
            agentCount,
        )
    }
}
```

## Configuration

### FCM Setup

1. Create a Firebase project at https://console.firebase.google.com
2. Generate a service account key:
   - Project Settings > Service Accounts > Generate New Private Key
3. Save the JSON file securely
4. Set the path in your configuration or environment:

```yaml
# configs/config.yaml
notifications:
  fcm_credentials_path: "/path/to/firebase-credentials.json"
```

### Environment Variables

```bash
export FCM_CREDENTIALS_PATH="/path/to/firebase-credentials.json"
```

### Mock Mode (Development)

If no FCM credentials are configured, the service automatically uses a mock backend that logs notifications to stderr instead of sending them. This is perfect for development and testing.

```go
// This will use mock mode if file doesn't exist
backend, _ := notifications.NewFCMBackend(ctx, "")
// Logs: "Mock FCM notification (not actually sent)"
```

## Notification Types

### Trade Execution
- **When**: Order is filled
- **Priority**: High
- **Default**: Enabled

### P&L Alerts
- **When**: P&L changes by ±5% or more
- **Priority**: High
- **Default**: Enabled

### Circuit Breaker
- **When**: Trading is halted due to risk limits
- **Priority**: High
- **Default**: Enabled

### Consensus Failures
- **When**: Agents fail to reach consensus on a trade
- **Priority**: Normal
- **Default**: Enabled

## Testing

```bash
# Run all notification tests
go test -v ./internal/notifications/...

# Test with coverage
go test -v -cover ./internal/notifications/...

# Test specific function
go test -v -run TestFCMBackend ./internal/notifications/...
```

## Security Considerations

1. **Device Token Security**: Device tokens are sensitive and should be transmitted over HTTPS only
2. **FCM Credentials**: Store Firebase credentials securely, never commit to git
3. **User Privacy**: Respect user preferences, provide easy opt-out
4. **Rate Limiting**: Consider implementing rate limits to prevent notification spam
5. **Token Masking**: Device tokens are automatically masked in logs (e.g., "abcd...5678")

## Performance

- **Database Indexes**: All query patterns are indexed for fast lookups
- **TimescaleDB**: notification_log uses TimescaleDB for efficient time-series queries
- **Batch Sending**: Use FCMBackend.SendMulticast for sending to multiple devices
- **Connection Pool**: Uses pgxpool for efficient database connections

## Extending with New Backends

To add a new notification backend (e.g., APNs, Web Push):

```go
type MyBackend struct {
    // backend-specific fields
}

func (b *MyBackend) Send(ctx context.Context, deviceToken string, notification Notification) error {
    // implementation
}

func (b *MyBackend) Name() string {
    return "my_backend"
}

func (b *MyBackend) Close() error {
    return nil
}

// Use it
backend := &MyBackend{}
service := notifications.NewService(db, backend)
```

## Migration

Run the migration to create the required tables:

```bash
task db-migrate
# Or manually:
psql -U cryptofunk -d cryptofunk -f migrations/011_user_devices.sql
```

## Troubleshooting

### "Failed to send FCM message"
- Check FCM credentials path is correct
- Verify device token is valid
- Ensure Firebase project has Cloud Messaging enabled

### "Device token not found"
- Device must be registered first with `RegisterDevice`
- Check device is not disabled

### "Notification type disabled for user"
- User has opted out of this notification type
- Check user preferences with `GetPreferences`

## Future Enhancements

- [ ] Add APNs backend for iOS native support
- [ ] Add web push backend for browser notifications
- [ ] Implement notification batching for efficiency
- [ ] Add retry logic with exponential backoff
- [ ] Support notification templates
- [ ] Add notification scheduling
- [ ] Implement notification channels/topics
- [ ] Add A/B testing support for notification content
