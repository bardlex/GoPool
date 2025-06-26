package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// UserRepository handles user-related database operations
type UserRepository struct {
	db *sql.DB
}

// NewUserRepository creates a new user repository
func NewUserRepository(db *sql.DB) *UserRepository {
	return &UserRepository{db: db}
}

// CreateUser creates a new user
func (r *UserRepository) CreateUser(ctx context.Context, user *User) error {
	query := `
		INSERT INTO users (address, username, email, hashed_password, minimum_payout, payout_address, is_active, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING id`

	now := time.Now()
	err := r.db.QueryRowContext(ctx, query,
		user.Address, user.Username, user.Email, user.HashedPassword,
		user.MinimumPayout, user.PayoutAddress, user.IsActive, now, now,
	).Scan(&user.ID)

	if err != nil {
		return fmt.Errorf("failed to create user: %w", err)
	}

	user.CreatedAt = now
	user.UpdatedAt = now
	return nil
}

// GetUserByAddress retrieves a user by their Bitcoin address
func (r *UserRepository) GetUserByAddress(ctx context.Context, address string) (*User, error) {
	query := `
		SELECT id, address, username, email, hashed_password, minimum_payout, payout_address, 
		       is_active, created_at, updated_at, last_seen_at
		FROM users WHERE address = $1`

	user := &User{}
	err := r.db.QueryRowContext(ctx, query, address).Scan(
		&user.ID, &user.Address, &user.Username, &user.Email, &user.HashedPassword,
		&user.MinimumPayout, &user.PayoutAddress, &user.IsActive,
		&user.CreatedAt, &user.UpdatedAt, &user.LastSeenAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("user not found")
		}
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	return user, nil
}

// UpdateLastSeen updates the user's last seen timestamp
func (r *UserRepository) UpdateLastSeen(ctx context.Context, userID int64) error {
	query := `UPDATE users SET last_seen_at = $1, updated_at = $2 WHERE id = $3`
	now := time.Now()

	_, err := r.db.ExecContext(ctx, query, now, now, userID)
	if err != nil {
		return fmt.Errorf("failed to update last seen: %w", err)
	}

	return nil
}

// ShareRepository handles share-related database operations
type ShareRepository struct {
	db *sql.DB
}

// NewShareRepository creates a new share repository
func NewShareRepository(db *sql.DB) *ShareRepository {
	return &ShareRepository{db: db}
}

// CreateShare creates a new share record
func (r *ShareRepository) CreateShare(ctx context.Context, share *Share) error {
	query := `
		INSERT INTO shares (user_id, worker_id, job_id, block_height, difficulty, network_difficulty,
		                   is_valid, is_block_candidate, hash, nonce, extra_nonce2, ntime, submitted_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
		RETURNING id`

	err := r.db.QueryRowContext(ctx, query,
		share.UserID, share.WorkerID, share.JobID, share.BlockHeight,
		share.Difficulty, share.NetworkDifficulty, share.IsValid, share.IsBlockCandidate,
		share.Hash, share.Nonce, share.ExtraNonce2, share.Ntime, share.SubmittedAt,
	).Scan(&share.ID)

	if err != nil {
		return fmt.Errorf("failed to create share: %w", err)
	}

	return nil
}

// GetSharesByUser retrieves shares for a specific user with pagination
func (r *ShareRepository) GetSharesByUser(ctx context.Context, userID int64, limit, offset int) ([]*Share, error) {
	query := `
		SELECT id, user_id, worker_id, job_id, block_height, difficulty, network_difficulty,
		       is_valid, is_block_candidate, hash, nonce, extra_nonce2, ntime, submitted_at, processed_at
		FROM shares 
		WHERE user_id = $1 
		ORDER BY submitted_at DESC 
		LIMIT $2 OFFSET $3`

	rows, err := r.db.QueryContext(ctx, query, userID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to query shares: %w", err)
	}
	defer func() {
		if err := rows.Close(); err != nil {
			// Note: Consider adding structured logging here in the future
			_ = err // Ignore close errors for now
		}
	}()

	var shares []*Share
	for rows.Next() {
		share := &Share{}
		err := rows.Scan(
			&share.ID, &share.UserID, &share.WorkerID, &share.JobID, &share.BlockHeight,
			&share.Difficulty, &share.NetworkDifficulty, &share.IsValid, &share.IsBlockCandidate,
			&share.Hash, &share.Nonce, &share.ExtraNonce2, &share.Ntime,
			&share.SubmittedAt, &share.ProcessedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan share: %w", err)
		}
		shares = append(shares, share)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating shares: %w", err)
	}

	return shares, nil
}

// MarkShareProcessed marks a share as processed
func (r *ShareRepository) MarkShareProcessed(ctx context.Context, shareID int64) error {
	query := `UPDATE shares SET processed_at = $1 WHERE id = $2`
	now := time.Now()

	_, err := r.db.ExecContext(ctx, query, now, shareID)
	if err != nil {
		return fmt.Errorf("failed to mark share processed: %w", err)
	}

	return nil
}

// BlockRepository handles block-related database operations
type BlockRepository struct {
	db *sql.DB
}

// NewBlockRepository creates a new block repository
func NewBlockRepository(db *sql.DB) *BlockRepository {
	return &BlockRepository{db: db}
}

// CreateBlock creates a new block record
func (r *BlockRepository) CreateBlock(ctx context.Context, block *Block) error {
	query := `
		INSERT INTO blocks (height, hash, prev_hash, merkle_root, timestamp, bits, nonce,
		                   difficulty, share_id, user_id, worker_id, status, confirmations, reward, found_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)
		RETURNING id`

	err := r.db.QueryRowContext(ctx, query,
		block.Height, block.Hash, block.PrevHash, block.MerkleRoot, block.Timestamp,
		block.Bits, block.Nonce, block.Difficulty, block.ShareID, block.UserID,
		block.WorkerID, block.Status, block.Confirmations, block.Reward, block.FoundAt,
	).Scan(&block.ID)

	if err != nil {
		return fmt.Errorf("failed to create block: %w", err)
	}

	return nil
}

// UpdateBlockStatus updates the status and confirmations of a block
func (r *BlockRepository) UpdateBlockStatus(ctx context.Context, blockID int64, status string, confirmations int) error {
	query := `UPDATE blocks SET status = $1, confirmations = $2`
	args := []any{status, confirmations}

	if status == "confirmed" {
		query += `, confirmed_at = $3`
		args = append(args, time.Now())
	}

	query += ` WHERE id = $` + fmt.Sprintf("%d", len(args)+1)
	args = append(args, blockID)

	_, err := r.db.ExecContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("failed to update block status: %w", err)
	}

	return nil
}

// GetRecentBlocks retrieves recent blocks with pagination
func (r *BlockRepository) GetRecentBlocks(ctx context.Context, limit, offset int) ([]*Block, error) {
	query := `
		SELECT id, height, hash, prev_hash, merkle_root, timestamp, bits, nonce,
		       difficulty, share_id, user_id, worker_id, status, confirmations, reward, found_at, confirmed_at
		FROM blocks 
		ORDER BY found_at DESC 
		LIMIT $1 OFFSET $2`

	rows, err := r.db.QueryContext(ctx, query, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to query blocks: %w", err)
	}
	defer func() {
		if err := rows.Close(); err != nil {
			// Note: Consider adding structured logging here in the future
			_ = err // Ignore close errors for now
		}
	}()

	var blocks []*Block
	for rows.Next() {
		block := &Block{}
		err := rows.Scan(
			&block.ID, &block.Height, &block.Hash, &block.PrevHash, &block.MerkleRoot,
			&block.Timestamp, &block.Bits, &block.Nonce, &block.Difficulty,
			&block.ShareID, &block.UserID, &block.WorkerID, &block.Status,
			&block.Confirmations, &block.Reward, &block.FoundAt, &block.ConfirmedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan block: %w", err)
		}
		blocks = append(blocks, block)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating blocks: %w", err)
	}

	return blocks, nil
}

// WorkerRepository handles worker-related database operations
type WorkerRepository struct {
	db *sql.DB
}

// NewWorkerRepository creates a new worker repository
func NewWorkerRepository(db *sql.DB) *WorkerRepository {
	return &WorkerRepository{db: db}
}

// CreateWorker creates a new worker
func (r *WorkerRepository) CreateWorker(ctx context.Context, worker *Worker) error {
	query := `
		INSERT INTO workers (user_id, name, password, difficulty, is_active, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id`

	now := time.Now()
	err := r.db.QueryRowContext(ctx, query,
		worker.UserID, worker.Name, worker.Password, worker.Difficulty,
		worker.IsActive, now, now,
	).Scan(&worker.ID)

	if err != nil {
		return fmt.Errorf("failed to create worker: %w", err)
	}

	worker.CreatedAt = now
	worker.UpdatedAt = now
	return nil
}

// GetWorkerByName retrieves a worker by user ID and name
func (r *WorkerRepository) GetWorkerByName(ctx context.Context, userID int64, name string) (*Worker, error) {
	query := `
		SELECT id, user_id, name, password, difficulty, is_active, created_at, updated_at, last_seen_at
		FROM workers 
		WHERE user_id = $1 AND name = $2`

	worker := &Worker{}
	err := r.db.QueryRowContext(ctx, query, userID, name).Scan(
		&worker.ID, &worker.UserID, &worker.Name, &worker.Password,
		&worker.Difficulty, &worker.IsActive, &worker.CreatedAt,
		&worker.UpdatedAt, &worker.LastSeenAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("worker not found")
		}
		return nil, fmt.Errorf("failed to get worker: %w", err)
	}

	return worker, nil
}

// UpdateWorkerLastSeen updates the worker's last seen timestamp
func (r *WorkerRepository) UpdateWorkerLastSeen(ctx context.Context, workerID int64) error {
	query := `UPDATE workers SET last_seen_at = $1, updated_at = $2 WHERE id = $3`
	now := time.Now()

	_, err := r.db.ExecContext(ctx, query, now, now, workerID)
	if err != nil {
		return fmt.Errorf("failed to update worker last seen: %w", err)
	}

	return nil
}
