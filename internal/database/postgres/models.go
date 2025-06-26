package postgres

import (
	"time"
)

// User represents a mining pool user
type User struct {
	ID             int64      `db:"id"`
	Address        string     `db:"address"`
	Username       string     `db:"username"`
	Email          string     `db:"email"`
	HashedPassword string     `db:"hashed_password"`
	MinimumPayout  float64    `db:"minimum_payout"`
	PayoutAddress  string     `db:"payout_address"`
	IsActive       bool       `db:"is_active"`
	CreatedAt      time.Time  `db:"created_at"`
	UpdatedAt      time.Time  `db:"updated_at"`
	LastSeenAt     *time.Time `db:"last_seen_at"`
}

// Worker represents a mining worker
type Worker struct {
	ID         int64      `db:"id"`
	UserID     int64      `db:"user_id"`
	Name       string     `db:"name"`
	Password   string     `db:"password"`
	Difficulty float64    `db:"difficulty"`
	IsActive   bool       `db:"is_active"`
	CreatedAt  time.Time  `db:"created_at"`
	UpdatedAt  time.Time  `db:"updated_at"`
	LastSeenAt *time.Time `db:"last_seen_at"`
}

// Share represents a submitted mining share
type Share struct {
	ID                int64      `db:"id"`
	UserID            int64      `db:"user_id"`
	WorkerID          int64      `db:"worker_id"`
	JobID             string     `db:"job_id"`
	BlockHeight       int64      `db:"block_height"`
	Difficulty        float64    `db:"difficulty"`
	NetworkDifficulty float64    `db:"network_difficulty"`
	IsValid           bool       `db:"is_valid"`
	IsBlockCandidate  bool       `db:"is_block_candidate"`
	Hash              string     `db:"hash"`
	Nonce             string     `db:"nonce"`
	ExtraNonce2       string     `db:"extra_nonce2"`
	Ntime             string     `db:"ntime"`
	SubmittedAt       time.Time  `db:"submitted_at"`
	ProcessedAt       *time.Time `db:"processed_at"`
}

// Block represents a found block
type Block struct {
	ID            int64      `db:"id"`
	Height        int64      `db:"height"`
	Hash          string     `db:"hash"`
	PrevHash      string     `db:"prev_hash"`
	MerkleRoot    string     `db:"merkle_root"`
	Timestamp     time.Time  `db:"timestamp"`
	Bits          string     `db:"bits"`
	Nonce         string     `db:"nonce"`
	Difficulty    float64    `db:"difficulty"`
	ShareID       *int64     `db:"share_id"`
	UserID        *int64     `db:"user_id"`
	WorkerID      *int64     `db:"worker_id"`
	Status        string     `db:"status"` // pending, confirmed, orphaned
	Confirmations int        `db:"confirmations"`
	Reward        float64    `db:"reward"`
	FoundAt       time.Time  `db:"found_at"`
	ConfirmedAt   *time.Time `db:"confirmed_at"`
}

// Payout represents a payout to a user
type Payout struct {
	ID          int64      `db:"id"`
	UserID      int64      `db:"user_id"`
	Amount      float64    `db:"amount"`
	Address     string     `db:"address"`
	TxID        *string    `db:"tx_id"`
	Status      string     `db:"status"` // pending, sent, confirmed, failed
	BlockHeight *int64     `db:"block_height"`
	CreatedAt   time.Time  `db:"created_at"`
	SentAt      *time.Time `db:"sent_at"`
	ConfirmedAt *time.Time `db:"confirmed_at"`
}

// UserStats represents aggregated user statistics
type UserStats struct {
	UserID          int64      `db:"user_id"`
	ValidShares     int64      `db:"valid_shares"`
	InvalidShares   int64      `db:"invalid_shares"`
	BlocksFound     int64      `db:"blocks_found"`
	TotalEarnings   float64    `db:"total_earnings"`
	PendingEarnings float64    `db:"pending_earnings"`
	LastShareAt     *time.Time `db:"last_share_at"`
}

// PoolStats represents overall pool statistics
type PoolStats struct {
	ID              int64     `db:"id"`
	TotalHashrate   float64   `db:"total_hashrate"`
	ActiveWorkers   int64     `db:"active_workers"`
	ActiveUsers     int64     `db:"active_users"`
	BlocksFound     int64     `db:"blocks_found"`
	TotalShares     int64     `db:"total_shares"`
	ValidShares     int64     `db:"valid_shares"`
	InvalidShares   int64     `db:"invalid_shares"`
	NetworkHashrate float64   `db:"network_hashrate"`
	NetworkDiff     float64   `db:"network_difficulty"`
	Timestamp       time.Time `db:"timestamp"`
}
