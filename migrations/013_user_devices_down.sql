-- Rollback: User Devices and Notification Preferences
-- Description: Remove tables for push notification device tokens and user preferences

DROP TRIGGER IF EXISTS notification_preferences_updated_at ON notification_preferences;
DROP FUNCTION IF EXISTS update_notification_preferences_updated_at();

DROP TABLE IF EXISTS notification_log;
DROP TABLE IF EXISTS notification_preferences;
DROP TABLE IF EXISTS user_devices;
