-- Rollback Telegram Users Integration

-- Drop tables in reverse order (respecting foreign key constraints)
DROP TABLE IF EXISTS telegram_alert_queue;
DROP TABLE IF EXISTS telegram_messages;
DROP TABLE IF EXISTS telegram_users;

-- Drop functions
DROP FUNCTION IF EXISTS generate_verification_code();
DROP FUNCTION IF EXISTS update_telegram_users_updated_at();
