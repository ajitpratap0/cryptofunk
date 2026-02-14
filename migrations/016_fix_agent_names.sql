-- Fix agent_status seed data to match actual agent binary names
-- Old seed names from 001_initial_schema.sql don't match cmd/agents/*/

UPDATE agent_status SET agent_name = 'technical-agent' WHERE agent_name = 'technical-analyst';
UPDATE agent_status SET agent_name = 'orderbook-agent' WHERE agent_name = 'orderbook-analyst';
UPDATE agent_status SET agent_name = 'sentiment-agent' WHERE agent_name = 'sentiment-analyst';
UPDATE agent_status SET agent_name = 'trend-agent' WHERE agent_name = 'trend-follower';
UPDATE agent_status SET agent_name = 'reversion-agent' WHERE agent_name = 'mean-reversion';
UPDATE agent_status SET agent_name = 'risk-agent' WHERE agent_name = 'risk-manager';

-- Insert missing agents that were added after initial schema
INSERT INTO agent_status (agent_name, agent_type, status) VALUES
    ('arbitrage-agent', 'strategy', 'STOPPED'),
    ('polymarket-agent', 'strategy', 'STOPPED')
ON CONFLICT (agent_name) DO NOTHING;
