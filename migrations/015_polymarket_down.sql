-- Rollback Polymarket tables
DROP TRIGGER IF EXISTS update_polymarket_positions_updated_at ON polymarket_positions;
DROP TABLE IF EXISTS polymarket_predictions CASCADE;
DROP TABLE IF EXISTS polymarket_trades CASCADE;
DROP TABLE IF EXISTS polymarket_positions CASCADE;
DROP TABLE IF EXISTS polymarket_markets CASCADE;
