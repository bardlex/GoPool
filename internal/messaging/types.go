package messaging

import "time"

// JobMessage represents a mining job distributed to stratumd services
type JobMessage struct {
	JobID        string    `json:"job_id"`
	PrevHash     string    `json:"prev_hash"`
	Coinb1       string    `json:"coinb1"`
	Coinb2       string    `json:"coinb2"`
	MerkleBranch []string  `json:"merkle_branch"`
	Version      string    `json:"version"`
	NBits        string    `json:"nbits"`
	NTime        string    `json:"ntime"`
	CleanJobs    bool      `json:"clean_jobs"`
	BlockHeight  int64     `json:"block_height"`
	Difficulty   float64   `json:"difficulty"`
	CreatedAt    time.Time `json:"created_at"`
}

// ShareMessage represents a share submission from stratumd to shareproc
type ShareMessage struct {
	ShareID      string    `json:"share_id"`
	JobID        string    `json:"job_id"`
	UserID       int64     `json:"user_id"`
	WorkerID     int64     `json:"worker_id"`
	MinerAddress string    `json:"miner_address"`
	WorkerName   string    `json:"worker_name"`
	ExtraNonce2  string    `json:"extra_nonce2"`
	Ntime        string    `json:"ntime"`
	Nonce        string    `json:"nonce"`
	Difficulty   float64   `json:"difficulty"`
	BlockHeight  int64     `json:"block_height"`
	SessionID    string    `json:"session_id"`
	RemoteAddr   string    `json:"remote_addr"`
	SubmittedAt  time.Time `json:"submitted_at"`
}

// BlockCandidateMessage represents a block candidate from shareproc to blocksubmit
type BlockCandidateMessage struct {
	ShareID      string    `json:"share_id"`
	JobID        string    `json:"job_id"`
	BlockHash    string    `json:"block_hash"`
	BlockHex     string    `json:"block_hex"`
	BlockHeight  int64     `json:"block_height"`
	Difficulty   float64   `json:"difficulty"`
	UserID       int64     `json:"user_id"`
	WorkerID     int64     `json:"worker_id"`
	MinerAddress string    `json:"miner_address"`
	WorkerName   string    `json:"worker_name"`
	ExtraNonce2  string    `json:"extra_nonce2"`
	Ntime        string    `json:"ntime"`
	Nonce        string    `json:"nonce"`
	FoundAt      time.Time `json:"found_at"`
}

// BlockSubmissionResult represents the result of block submission
type BlockSubmissionResult struct {
	ShareID        string    `json:"share_id"`
	BlockHash      string    `json:"block_hash"`
	BlockHeight    int64     `json:"block_height"`
	Status         string    `json:"status"` // "accepted", "rejected", "duplicate"
	ErrorMessage   string    `json:"error_message,omitempty"`
	SubmissionTime time.Time `json:"submission_time"`
	LatencyMs      float64   `json:"latency_ms"`
}

// ShareValidationResult represents the result of share validation
type ShareValidationResult struct {
	ShareID          string    `json:"share_id"`
	JobID            string    `json:"job_id"`
	Status           string    `json:"status"` // "valid", "invalid", "stale", "duplicate"
	ErrorMessage     string    `json:"error_message,omitempty"`
	IsBlockCandidate bool      `json:"is_block_candidate"`
	ProcessedAt      time.Time `json:"processed_at"`
	ProcessingTimeMs float64   `json:"processing_time_ms"`
}

// UserStatsUpdate represents user statistics updates
type UserStatsUpdate struct {
	UserID        int64     `json:"user_id"`
	WorkerID      int64     `json:"worker_id"`
	ValidShares   int64     `json:"valid_shares"`
	InvalidShares int64     `json:"invalid_shares"`
	Hashrate      float64   `json:"hashrate"`
	LastShareAt   time.Time `json:"last_share_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}
