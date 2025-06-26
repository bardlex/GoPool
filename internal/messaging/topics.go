package messaging

// Topic constants for the mining pool messaging system
const (
	// Core mining workflow topics
	TopicJobs            = "mining.jobs"             // jobmanager → stratumd
	TopicShares          = "mining.shares"           // stratumd → shareproc
	TopicBlockCandidates = "mining.block_candidates" // shareproc → blocksubmit (HOT PATH)
	TopicBlockResults    = "mining.block_results"    // blocksubmit → statsd
	TopicShareResults    = "mining.share_results"    // shareproc → statsd

	// Statistics and monitoring topics
	TopicUserStats  = "mining.user_stats" // shareproc → statsd
	TopicMinerStats = "miner.stats"       // stratumd → statsd
	TopicPoolStats  = "pool.stats"        // statsd → apiserver
)
