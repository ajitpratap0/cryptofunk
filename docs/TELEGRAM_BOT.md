# Telegram Bot Integration

CryptoFunk includes a Telegram bot that allows you to monitor and control your trading system from your mobile device.

## Features

- Real-time trading status and position monitoring
- Emergency pause/resume trading controls
- Performance and P&L tracking
- Recent agent decision insights
- Alert notifications directly to your Telegram

## Quick Start

### 1. Create a Telegram Bot

1. Open Telegram and search for `@BotFather`
2. Start a chat and send `/newbot`
3. Follow the prompts to create your bot
4. Save the bot token provided by BotFather

### 2. Configure the Bot

Add the bot token to your `.env` file:

```bash
TELEGRAM_BOT_TOKEN=your_bot_token_here
TELEGRAM_ENABLED=true
```

Or update `configs/config.yaml`:

```yaml
telegram:
  enabled: true
  bot_token: "${TELEGRAM_BOT_TOKEN}"
```

### 3. Run the Database Migration

The Telegram bot requires database tables:

```bash
task db-migrate
```

This will create the `telegram_users`, `telegram_messages`, and `telegram_alert_queue` tables.

### 4. Start the Bot

```bash
task build-telegram-bot
task run-telegram-bot
```

Or run directly:

```bash
./bin/telegram-bot
```

### 5. Verify Your Account

1. Find your bot on Telegram (search for the username you created)
2. Send `/start` to the bot
3. Generate a verification code from the CryptoFunk dashboard or via SQL:
   ```sql
   SELECT * FROM generate_verification_code();
   ```
4. Send the verification code to the bot:
   ```
   /verify ABC123
   ```
5. Once verified, you'll receive alerts and can use all bot commands

## Available Commands

### Monitoring Commands

- `/status` - Show active trading sessions and current positions
- `/positions` - List all open positions with detailed P&L
- `/pl` - Show session profit/loss (realized + unrealized)
- `/decisions` - Show the last 5 agent decisions with reasoning

### Control Commands

- `/pause` - Emergency pause all trading (positions remain open)
- `/resume` - Resume trading after pause

### Settings Commands

- `/settings` - View notification preferences
- `/verify <code>` - Verify your account to receive alerts
- `/help` - Show help message
- `/start` - Show welcome message

## Alert Types

The bot can send you various alerts:

- **Critical Alerts** 🚨
  - Order failures
  - Connection errors
  - System errors
  - Position risk violations

- **Warning Alerts** ⚠️
  - High volatility
  - Approaching risk limits
  - Order cancellations

- **Info Alerts** ℹ️
  - Trade executions
  - Position updates
  - Daily performance summaries

## Notification Preferences

Control which notifications you receive:

1. Use the CryptoFunk dashboard to manage preferences
2. Or update directly in the database:
   ```sql
   UPDATE telegram_users
   SET receive_alerts = true,
       receive_trade_notifications = true,
       receive_daily_summary = false
   WHERE telegram_id = YOUR_TELEGRAM_ID;
   ```

## Integration with Alerts

To send alerts via Telegram from your code:

```go
import (
    "github.com/ajitpratap0/cryptofunk/internal/alerts"
    "github.com/ajitpratap0/cryptofunk/internal/telegram"
)

// Create Telegram alerter
telegramAlerter, err := alerts.NewTelegramAlerter(botToken, chatIDs)
if err != nil {
    log.Fatal(err)
}

// Add to alert manager
alertManager := alerts.NewManager(
    alerts.NewLogAlerter(),
    alerts.NewConsoleAlerter(),
    telegramAlerter,
)

// Send alert
alertManager.SendCritical(ctx, "Trade Failed", "Failed to execute order", metadata)
```

## Advanced Configuration

### Webhook Mode (Production)

For production deployments, use webhook mode instead of polling:

```yaml
telegram:
  enabled: true
  bot_token: "${TELEGRAM_BOT_TOKEN}"
  webhook_url: "https://your-domain.com/telegram/webhook"
```

### Multiple Users

The bot supports multiple verified users. Each user must:

1. Generate their own verification code
2. Verify their account with `/verify`
3. Configure their notification preferences

### Database Schema

The bot uses three main tables:

- `telegram_users` - User accounts and preferences
- `telegram_messages` - Audit log of all interactions
- `telegram_alert_queue` - Queue for pending alerts

### Verification Code Management

Verification codes:
- Are 6 characters (alphanumeric, excluding confusing characters)
- Expire after 1 hour
- Are single-use (deleted after successful verification)
- Can be regenerated if expired

Generate a new code:

```sql
SELECT verification_code
FROM telegram_users
WHERE verification_code IS NOT NULL
AND verification_expires_at > CURRENT_TIMESTAMP;

-- Or create a new one
SELECT * FROM generate_verification_code();
```

## Security Considerations

1. **Bot Token**: Keep your bot token secret. Never commit it to version control.
2. **Verification**: Always require verification before allowing access to sensitive data.
3. **Rate Limiting**: The bot has built-in rate limiting to prevent abuse.
4. **HTTPS**: Use webhook mode with HTTPS in production.
5. **Audit Logging**: All bot interactions are logged in `telegram_messages`.

## Troubleshooting

### Bot Not Responding

1. Check if the bot is running: `ps aux | grep telegram-bot`
2. Check logs: `tail -f logs/telegram-bot.log`
3. Verify bot token is correct
4. Ensure database is accessible

### Verification Failed

1. Check if code has expired:
   ```sql
   SELECT verification_code, verification_expires_at
   FROM telegram_users
   WHERE verification_code = 'YOUR_CODE';
   ```
2. Generate a new code if expired
3. Ensure code is entered correctly (case-sensitive)

### Not Receiving Alerts

1. Check notification preferences:
   ```sql
   SELECT * FROM telegram_users WHERE telegram_id = YOUR_ID;
   ```
2. Verify you're authenticated: `/status`
3. Check alert queue:
   ```sql
   SELECT * FROM telegram_alert_queue WHERE sent = false;
   ```

## Development

### Running Tests

```bash
go test -v ./internal/telegram/
go test -v ./internal/alerts/
```

### Building

```bash
task build-telegram-bot
```

### Running in Development

```bash
LOG_LEVEL=debug task run-telegram-bot
```

## API Endpoints Used

The bot queries these orchestrator endpoints:

- `GET /api/v1/orchestrator/status` - System status
- `POST /api/v1/orchestrator/pause` - Pause trading
- `POST /api/v1/orchestrator/resume` - Resume trading

Ensure the API server is running and accessible at the configured `orchestrator_url`.

## Future Enhancements

Potential future features:

- Interactive trade approval
- Chart visualization
- Strategy backtesting triggers
- Portfolio rebalancing commands
- Multi-language support
- Inline keyboards for quick actions
- Group chat support for team monitoring

## License

Same as CryptoFunk main project license.
