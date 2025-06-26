-- Initial database schema for GOMP mining pool

-- Users table
CREATE TABLE users (
    id BIGSERIAL PRIMARY KEY,
    address VARCHAR(64) UNIQUE NOT NULL,
    username VARCHAR(50) UNIQUE NOT NULL,
    email VARCHAR(255) UNIQUE,
    hashed_password VARCHAR(255) NOT NULL,
    minimum_payout DECIMAL(16,8) DEFAULT 0.01,
    payout_address VARCHAR(64) NOT NULL,
    is_active BOOLEAN DEFAULT true,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    last_seen_at TIMESTAMP WITH TIME ZONE
);

-- Workers table
CREATE TABLE workers (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name VARCHAR(50) NOT NULL,
    password VARCHAR(255),
    difficulty DECIMAL(16,8) DEFAULT 1.0,
    is_active BOOLEAN DEFAULT true,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    last_seen_at TIMESTAMP WITH TIME ZONE,
    UNIQUE(user_id, name)
);

-- Shares table
CREATE TABLE shares (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id),
    worker_id BIGINT NOT NULL REFERENCES workers(id),
    job_id VARCHAR(64) NOT NULL,
    block_height BIGINT NOT NULL,
    difficulty DECIMAL(16,8) NOT NULL,
    network_difficulty DECIMAL(16,8) NOT NULL,
    is_valid BOOLEAN NOT NULL,
    is_block_candidate BOOLEAN DEFAULT false,
    hash VARCHAR(64),
    nonce VARCHAR(8) NOT NULL,
    extra_nonce2 VARCHAR(16) NOT NULL,
    ntime VARCHAR(8) NOT NULL,
    submitted_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    processed_at TIMESTAMP WITH TIME ZONE
);

-- Blocks table
CREATE TABLE blocks (
    id BIGSERIAL PRIMARY KEY,
    height BIGINT UNIQUE NOT NULL,
    hash VARCHAR(64) UNIQUE NOT NULL,
    prev_hash VARCHAR(64) NOT NULL,
    merkle_root VARCHAR(64) NOT NULL,
    timestamp TIMESTAMP WITH TIME ZONE NOT NULL,
    bits VARCHAR(8) NOT NULL,
    nonce VARCHAR(8) NOT NULL,
    difficulty DECIMAL(16,8) NOT NULL,
    share_id BIGINT REFERENCES shares(id),
    user_id BIGINT REFERENCES users(id),
    worker_id BIGINT REFERENCES workers(id),
    status VARCHAR(20) DEFAULT 'pending' CHECK (status IN ('pending', 'confirmed', 'orphaned')),
    confirmations INTEGER DEFAULT 0,
    reward DECIMAL(16,8) DEFAULT 0,
    found_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    confirmed_at TIMESTAMP WITH TIME ZONE
);

-- Payouts table
CREATE TABLE payouts (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id),
    amount DECIMAL(16,8) NOT NULL,
    address VARCHAR(64) NOT NULL,
    tx_id VARCHAR(64),
    status VARCHAR(20) DEFAULT 'pending' CHECK (status IN ('pending', 'sent', 'confirmed', 'failed')),
    block_height BIGINT,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    sent_at TIMESTAMP WITH TIME ZONE,
    confirmed_at TIMESTAMP WITH TIME ZONE
);

-- User statistics view (materialized for performance)
CREATE MATERIALIZED VIEW user_stats AS
SELECT 
    u.id as user_id,
    COALESCE(s.valid_shares, 0) as valid_shares,
    COALESCE(s.invalid_shares, 0) as invalid_shares,
    COALESCE(b.blocks_found, 0) as blocks_found,
    COALESCE(p.total_earnings, 0) as total_earnings,
    COALESCE(p.pending_earnings, 0) as pending_earnings,
    s.last_share_at
FROM users u
LEFT JOIN (
    SELECT 
        user_id,
        SUM(CASE WHEN is_valid THEN 1 ELSE 0 END) as valid_shares,
        SUM(CASE WHEN NOT is_valid THEN 1 ELSE 0 END) as invalid_shares,
        MAX(submitted_at) as last_share_at
    FROM shares 
    GROUP BY user_id
) s ON u.id = s.user_id
LEFT JOIN (
    SELECT 
        user_id,
        COUNT(*) as blocks_found
    FROM blocks 
    WHERE user_id IS NOT NULL
    GROUP BY user_id
) b ON u.id = b.user_id
LEFT JOIN (
    SELECT 
        user_id,
        SUM(CASE WHEN status = 'confirmed' THEN amount ELSE 0 END) as total_earnings,
        SUM(CASE WHEN status = 'pending' THEN amount ELSE 0 END) as pending_earnings
    FROM payouts 
    GROUP BY user_id
) p ON u.id = p.user_id;

-- Pool statistics table
CREATE TABLE pool_stats (
    id BIGSERIAL PRIMARY KEY,
    total_hashrate DECIMAL(20,8) DEFAULT 0,
    active_workers BIGINT DEFAULT 0,
    active_users BIGINT DEFAULT 0,
    blocks_found BIGINT DEFAULT 0,
    total_shares BIGINT DEFAULT 0,
    valid_shares BIGINT DEFAULT 0,
    invalid_shares BIGINT DEFAULT 0,
    network_hashrate DECIMAL(20,8) DEFAULT 0,
    network_difficulty DECIMAL(20,8) DEFAULT 0,
    timestamp TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Indexes for performance
CREATE INDEX idx_shares_user_id ON shares(user_id);
CREATE INDEX idx_shares_worker_id ON shares(worker_id);
CREATE INDEX idx_shares_submitted_at ON shares(submitted_at);
CREATE INDEX idx_shares_block_height ON shares(block_height);
CREATE INDEX idx_shares_is_valid ON shares(is_valid);
CREATE INDEX idx_shares_is_block_candidate ON shares(is_block_candidate);

CREATE INDEX idx_workers_user_id ON workers(user_id);
CREATE INDEX idx_workers_is_active ON workers(is_active);

CREATE INDEX idx_blocks_height ON blocks(height);
CREATE INDEX idx_blocks_status ON blocks(status);
CREATE INDEX idx_blocks_found_at ON blocks(found_at);

CREATE INDEX idx_payouts_user_id ON payouts(user_id);
CREATE INDEX idx_payouts_status ON payouts(status);
CREATE INDEX idx_payouts_created_at ON payouts(created_at);

CREATE INDEX idx_users_address ON users(address);
CREATE INDEX idx_users_is_active ON users(is_active);

-- Triggers for updated_at timestamps
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ language 'plpgsql';

CREATE TRIGGER update_users_updated_at BEFORE UPDATE ON users
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_workers_updated_at BEFORE UPDATE ON workers
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- Function to refresh user stats
CREATE OR REPLACE FUNCTION refresh_user_stats()
RETURNS void AS $$
BEGIN
    REFRESH MATERIALIZED VIEW user_stats;
END;
$$ LANGUAGE plpgsql;

-- Comments for documentation
COMMENT ON TABLE users IS 'Mining pool users and their settings';
COMMENT ON TABLE workers IS 'Mining workers belonging to users';
COMMENT ON TABLE shares IS 'Submitted mining shares for validation and accounting';
COMMENT ON TABLE blocks IS 'Found blocks and their confirmation status';
COMMENT ON TABLE payouts IS 'Payout transactions to users';
COMMENT ON MATERIALIZED VIEW user_stats IS 'Aggregated statistics per user (refreshed periodically)';
COMMENT ON TABLE pool_stats IS 'Overall pool statistics snapshots';