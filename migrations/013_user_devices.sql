-- Migration: User Devices and Notification Preferences
-- Description: Add tables for push notification device tokens and user preferences

-- User devices table for push notification tokens
-- Note: user_id is stored without FK constraint as users table doesn't exist yet
-- FK constraint can be added when a central users table is introduced
CREATE TABLE IF NOT EXISTS user_devices (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL,
    device_token TEXT NOT NULL UNIQUE,
    platform VARCHAR(20) NOT NULL CHECK (platform IN ('ios', 'android', 'web')),
    enabled BOOLEAN DEFAULT TRUE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    last_used_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- Indexes for efficient lookups
CREATE INDEX IF NOT EXISTS idx_user_devices_user_id ON user_devices(user_id) WHERE enabled = TRUE;
CREATE INDEX IF NOT EXISTS idx_user_devices_token ON user_devices(device_token) WHERE enabled = TRUE;
CREATE INDEX IF NOT EXISTS idx_user_devices_platform ON user_devices(platform);

-- Notification preferences table
-- Note: user_id is stored without FK constraint as users table doesn't exist yet
CREATE TABLE IF NOT EXISTS notification_preferences (
    user_id UUID PRIMARY KEY,
    trade_executions BOOLEAN DEFAULT TRUE,
    pnl_alerts BOOLEAN DEFAULT TRUE,
    circuit_breaker BOOLEAN DEFAULT TRUE,
    consensus_failures BOOLEAN DEFAULT TRUE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- Notification log table for tracking sent notifications
-- Note: user_id is stored without FK constraint as users table doesn't exist yet
CREATE TABLE IF NOT EXISTS notification_log (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL,
    device_token TEXT,
    notification_type VARCHAR(50) NOT NULL,
    title TEXT NOT NULL,
    body TEXT NOT NULL,
    data JSONB,
    status VARCHAR(20) NOT NULL CHECK (status IN ('pending', 'sent', 'failed')),
    error_message TEXT,
    sent_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- Indexes for notification log
CREATE INDEX IF NOT EXISTS idx_notification_log_user_id ON notification_log(user_id);
CREATE INDEX IF NOT EXISTS idx_notification_log_type ON notification_log(notification_type);
CREATE INDEX IF NOT EXISTS idx_notification_log_status ON notification_log(status);
CREATE INDEX IF NOT EXISTS idx_notification_log_sent_at ON notification_log(sent_at DESC);

-- Convert notification_log to hypertable for efficient time-series queries
SELECT create_hypertable('notification_log', 'sent_at', if_not_exists => TRUE);

-- Trigger to update updated_at timestamp
CREATE OR REPLACE FUNCTION update_notification_preferences_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = CURRENT_TIMESTAMP;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER notification_preferences_updated_at
    BEFORE UPDATE ON notification_preferences
    FOR EACH ROW
    EXECUTE FUNCTION update_notification_preferences_updated_at();

-- Comments for documentation
COMMENT ON TABLE user_devices IS 'Stores device tokens for push notifications';
COMMENT ON TABLE notification_preferences IS 'User preferences for different notification types';
COMMENT ON TABLE notification_log IS 'Audit log of all sent notifications';
COMMENT ON COLUMN user_devices.device_token IS 'FCM or APNs device token';
COMMENT ON COLUMN user_devices.platform IS 'Device platform: ios, android, or web';
COMMENT ON COLUMN user_devices.last_used_at IS 'Last time this device received a notification';
