package validation

import "time"

// Share represents a mining share submission (simplified version)
type Share struct {
	ID                string
	JobID             string
	MinerAddress      string
	WorkerName        string
	ExtraNonce2       string
	Ntime             string
	Nonce             string
	Difficulty        float64
	NetworkDifficulty float64
	BlockHeight       int64
	SubmittedAt       time.Time
}

// MiningJob represents a work template for miners (simplified version)
type MiningJob struct {
	JobID             string
	PrevHash          string
	Coinb1            string
	Coinb2            string
	MerkleBranch      []string
	Version           string
	Nbits             string
	Ntime             string
	CleanJobs         bool
	BlockHeight       int64
	NetworkDifficulty float64
	CreatedAt         time.Time
	ExpiresAt         *time.Time
}
