-- Telegram Users Integration
-- Enables users to link their accounts with Telegram for receiving alerts and controlling the bot

-- =============================================================================
-- TELEGRAM USERS TABLE
-- =============================================================================

CREATE TABLE telegram_users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    telegram_id BIGINT UNIQUE NOT NULL,
    telegram_username VARCHAR(255),
    chat_id BIGINT NOT NULL,
    is_verified BOOLEAN DEFAULT FALSE,
    verification_code VARCHAR(32),
    verification_expires_at TIMESTAMPTZ,
    -- Notification preferences
    receive_alerts BOOLEAN DEFAULT TRUE,
    receive_trade_notifications BOOLEAN DEFAULT TRUE,
    receive_daily_summary BOOLEAN DEFAULT TRUE,
    -- Metadata
    first_name VARCHAR(255),
    last_name VARCHAR(255),
    language_code VARCHAR(10),
    -- Timestamps
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    last_interaction_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
);

-- Create indexes
CREATE INDEX idx_telegram_users_telegram_id ON telegram_users (telegram_id);
CREATE INDEX idx_telegram_users_chat_id ON telegram_users (chat_id);
CREATE INDEX idx_telegram_users_is_verified ON telegram_users (is_verified);
CREATE INDEX idx_telegram_users_verification_code ON telegram_users (verification_code) WHERE verification_code IS NOT NULL;

-- Add trigger to update updated_at timestamp
CREATE OR REPLACE FUNCTION update_telegram_users_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = CURRENT_TIMESTAMP;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER telegram_users_updated_at
    BEFORE UPDATE ON telegram_users
    FOR EACH ROW
    EXECUTE FUNCTION update_telegram_users_updated_at();

-- =============================================================================
-- TELEGRAM MESSAGES LOG (Optional - for tracking bot interactions)
-- =============================================================================

CREATE TABLE telegram_messages (
    id BIGSERIAL PRIMARY KEY,
    telegram_user_id UUID REFERENCES telegram_users(id) ON DELETE CASCADE,
    message_id INTEGER NOT NULL,
    chat_id BIGINT NOT NULL,
    command VARCHAR(50),
    text TEXT,
    response TEXT,
    success BOOLEAN DEFAULT TRUE,
    error_message TEXT,
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_telegram_messages_user_id ON telegram_messages (telegram_user_id, created_at DESC);
CREATE INDEX idx_telegram_messages_chat_id ON telegram_messages (chat_id, created_at DESC);
CREATE INDEX idx_telegram_messages_command ON telegram_messages (command, created_at DESC);

-- =============================================================================
-- TELEGRAM ALERT QUEUE
-- =============================================================================
-- Queue for alerts to be sent via Telegram

CREATE TABLE telegram_alert_queue (
    id BIGSERIAL PRIMARY KEY,
    telegram_user_id UUID REFERENCES telegram_users(id) ON DELETE CASCADE,
    chat_id BIGINT NOT NULL,
    title VARCHAR(255) NOT NULL,
    message TEXT NOT NULL,
    severity VARCHAR(20) NOT NULL,
    metadata JSONB,
    sent BOOLEAN DEFAULT FALSE,
    sent_at TIMESTAMPTZ,
    error_message TEXT,
    retry_count INTEGER DEFAULT 0,
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_telegram_alert_queue_sent ON telegram_alert_queue (sent, created_at);
CREATE INDEX idx_telegram_alert_queue_user_id ON telegram_alert_queue (telegram_user_id, created_at DESC);

-- Add function to generate verification codes
CREATE OR REPLACE FUNCTION generate_verification_code()
RETURNS VARCHAR(32) AS $$
DECLARE
    chars VARCHAR := 'ABCDEFGHJKLMNPQRSTUVWXYZ23456789';
    result VARCHAR := '';
    i INTEGER;
BEGIN
    FOR i IN 1..6 LOOP
        result := result || substr(chars, floor(random() * length(chars) + 1)::integer, 1);
    END LOOP;
    RETURN result;
END;
$$ LANGUAGE plpgsql;

-- Add comment explaining table purpose
COMMENT ON TABLE telegram_users IS 'Stores Telegram user information for bot interactions and alerts';
COMMENT ON TABLE telegram_messages IS 'Logs all bot interactions for audit and debugging';
COMMENT ON TABLE telegram_alert_queue IS 'Queue for alerts to be sent to Telegram users';
