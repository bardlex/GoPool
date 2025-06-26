# PPLNS (Pay-Per-Last-N-Shares) Technical Specification

## 1. Foundational Principles

### 1.1. Core Mechanism

* **Definition**: Pay-Per-Last-N-Shares (PPLNS) is a mining pool reward method where payouts are based on a miner's contribution to the last 'N' shares submitted to the pool before a block is found.
* **Sliding Window**: The system uses a "sliding window" of the 'N' most recent shares. As new shares are submitted, the oldest are discarded.
* **Payout Trigger**: Rewards are distributed *only* when the pool successfully finds a block.
* **Pool Hopping Resistance**: The sliding window mechanism inherently discourages "pool hopping" (miners switching pools for short-term gain), as new miners must build up a history of shares in the window to receive significant rewards. This fosters a stable pool hashrate.

### 1.2. Economic Model

* **Risk Transfer**: PPLNS transfers the risk of mining variance (luck) from the pool operator to the miners. The pool only pays out what it earns.
* **Fee Structure**: Because the operator carries minimal financial risk, PPLNS pools can charge significantly lower fees (e.g., 0.5-2%) compared to guaranteed-payout models like PPS (2-4%).
* **Miner Earnings**: For loyal, long-term miners, lower fees can result in higher overall earnings over time, assuming the pool's luck averages to 100%.

### 1.3. The 'N' Parameter

* **Definition**: 'N' is the size of the share window. It is the most critical configuration parameter.
* **Calibration**: 'N' is often defined not as a raw share count, but as a multiple ($X$) of the network difficulty ($D_{net}$), e.g., $N = X \times D_{net}$. A common value for $X$ is 2. This ensures the window represents a consistent amount of work as network difficulty changes.
* **Impact of 'N' Size**:
  * **Large 'N'**: Reduces payout variance (smoother income). Creates a longer "ramp-up" for new miners and a "ramp-down" (buffer effect) for departing miners. Strongly favors long-term loyalty.
  * **Small 'N'**: Increases payout volatility, behaving more like a Proportional (PROP) system. Shorter ramp-up/down times.

### 1.4. Advantages & Disadvantages

* **Advantages**:
  * Higher potential long-term earnings due to lower fees.
  * Enhanced pool stability by deterring pool hopping.
  * Rewards loyalty and aligns miner incentives with the pool's health.
* **Disadvantages**:
  * High short-term payout volatility (income variance) due to dependency on pool luck.
  * Difficult onboarding for new miners due to the "ramp-up" period where initial payouts are low.
  * Casual miners may see their shares pushed out of the window before a block is found, earning nothing.

-----

## 2. Comparative Analysis of Payout Schemes

### 2.1. PPLNS vs. Other Models

* **vs. PPS (Pay-Per-Share)**: PPS offers guaranteed, predictable income per share, but with higher fees as the operator assumes all risk (variance, orphans). PPLNS offers potentially higher long-term rewards with lower fees, but income is variable.
* **vs. FPPS (Full-Pay-Per-Share) / PPS+**:
  * PPLNS includes *actual* transaction fees from a found block in the reward.
  * FPPS pays a guaranteed rate that includes an *estimated* average of transaction fees, removing fee volatility for the miner but introducing model risk for the operator.
  * PPS+ is a hybrid: the block subsidy is paid via PPS, while transaction fees are distributed via PPLNS.
* **vs. PROP (Proportional)**: PROP pays based on shares within a discrete round (between blocks), making it highly vulnerable to pool hopping. PPLNS is the successor to PROP, using a sliding window to fix this vulnerability.
* **vs. SOLO**: Solo mining offers 100% of the reward but has extremely high variance, making it unviable for most. PPLNS smooths this variance by pooling hashrate.

### 2.2. Payout Scheme Comparison Matrix

| Feature                | PPS             | FPPS                             | PPS+                                        | PPLNS                                   | SOLO             |
|------------------------|-----------------|----------------------------------|---------------------------------------------|-----------------------------------------|------------------|
| **Reward Basis**       | Fixed per share | Fixed per share (incl. fee est.) | Hybrid: Fixed (subsidy), Proportional (fees)| Proportional to work in last 'N' shares | Winner-take-all  |
| **Block Subsidy**      | Guaranteed      | Guaranteed                       | Guaranteed                                  | Luck-based                              | Luck-based       |
| **Transaction Fees**   | None            | Estimated                        | Luck-based (PPLNS)                          | Luck-based                              | Luck-based       |
| **Payout Consistency** | High            | High                             | Medium-High                                 | Low-Moderate                            | Extremely Low    |
| **Risk Bearer**        | Operator        | Operator                         | Operator (subsidy), Miners (fees)           | Miners                                  | Miner            |
| **Typical Fee**        | 2-4%.           | 2-4%.                            | Hybrid.                                     | 0.5-2%.                                 | 0-1%.            |
| **Pool Hopping**       | Resistant       | Resistant                        | Low                                         | Resistant                               | N/A              |
| **Orphan Risk**        | Operator.       | Operator                         | Operator                                    | Miners.                                 | Miner            |
| **Ideal Miner**        | Risk-averse     | Needs predictable cash flow      | Seeks stability + upside                    | Long-term, risk-tolerant                | >1% network hash |

-----

## 3. Algorithmic Implementation

### 3.1. Payout Formulas (LaTeX)

1. **Total Block Reward ($R_{total}$)**:

   $$
   R_{total} = R_{subsidy} + F_{tx}
   $$

   Where $R_{subsidy}$ is the block subsidy and $F_{tx}$ are the transaction fees from the actual found block.

2. **Distributable Reward ($R_{dist}$)**:

   $$
   R_{dist} = R_{total} \times (1 - Fee_{pool})
   $$

3. **Miner Payout ($P_i$)**:

   $$
   P_i = R_{dist} \times \frac{S_i}{S_{total}}
   $$

   Where $S_i$ is the score of miner $i$ in the window and $S_{total}$ is the total score of all miners in the window.

### 3.2. Share Scoring (Vardiff)

* To handle Variable Difficulty (Vardiff), shares cannot be counted equally. Each share must be weighted by its difficulty.
* **Share Score**: A share submitted at difficulty $d$ has a score of $d$. It represents $d$ times more work than a share of difficulty 1.
* **Miner's Score ($S_i$)**:

   $$
   S_i = \sum_{j \in \text{Window}} \text{difficulty}(\text{share}_{ij})
   $$

* **Total Score ($S_{total}$)**:

   $$
   S_{total} = \sum_{i \in \text{Miners}} S_i
   $$

* **Window Capacity ($W_{capacity}$)**: The window size 'N' should be defined as a total score capacity, e.g., $W_{capacity} = X \times D_{net}$.

### 3.3. Payout Logic Pseudocode

```pseudocode
# Configuration
const PPLNS_WINDOW_FACTOR = 2.0;
const POOL_FEE = 0.01;
const BLOCK_MATURITY_THRESHOLD = 100;

# Event: Block Found
function OnBlockFound(foundBlockHeight, networkDifficultyAtFind):
    StoreBlock(height: foundBlockHeight, status: 'Pending', networkDifficulty: networkDifficultyAtFind);

# Background Worker: Process Matured Blocks
function ProcessBlockPayouts():
    # 1. Get pending blocks and check for maturity
    pendingBlocks = GetBlocksByStatus('Pending');
    for each block in pendingBlocks:
        if GetConfirmations(block.height) >= BLOCK_MATURITY_THRESHOLD:
            if IsBlockInMainChain(block.height):
                # 2. Confirmed: Fetch actual reward and update status
                actualReward = GetRewardFromCoinbase(block.height);
                UpdateBlock(height: block.height, status: 'Confirmed', reward: actualReward.subsidy, fees: actualReward.fees);
            else:
                # 3. Orphaned: Update status and skip
                UpdateBlockStatus(block.height, 'Orphaned');
                continue;

    # 4. Process confirmed but unpaid blocks
    matureBlocks = GetBlocksByStatus('Confirmed');
    for each block in matureBlocks:
        success = CalculateAndCreditPayouts(block);
        if success:
            UpdateBlockStatus(block.height, 'Paid');

# Payout Calculation
function CalculateAndCreditPayouts(matureBlock):
    # 5. Define window and get shares
    rewardToDistribute = (matureBlock.reward + matureBlock.fees) * (1 - POOL_FEE);
    windowScoreCapacity = matureBlock.networkDifficulty * PPLNS_WINDOW_FACTOR;
    shareWindow = GetSharesInWindow(matureBlock.height, windowScoreCapacity);

    # 6. Aggregate scores per miner
    minerScores = new map<minerId, score>;
    totalScoreInWindow = 0;
    for each share in shareWindow:
        minerScores[share.minerId] += share.difficulty; # Score is difficulty
        totalScoreInWindow += share.difficulty;

    if totalScoreInWindow <= 0: return false;

    # 7. Calculate and credit payouts atomically
    BeginDatabaseTransaction();
    try:
        for each minerId, minerScore in minerScores:
            payoutAmount = rewardToDistribute * (minerScore / totalScoreInWindow);
            CreditMinerBalance(minerId, payoutAmount, matureBlock.height);
        CommitDatabaseTransaction();
        return true;
    catch:
        RollbackDatabaseTransaction();
        return false;
```

-----

## 4. System Architecture (.NET 9.0 / C# 13)

### 4.1. Share Tracking Architecture

* **Hybrid Model**: Use a combination of in-memory and persistent storage for high throughput and durability.
* **Ingestion**: Use an in-memory, thread-safe circular buffer (e.g., `System.Collections.Concurrent.ConcurrentQueue` in C#) for real-time share ingestion with O(1) complexity. This minimizes latency on the Stratum server.
* **Persistence**: A background worker service (`IHostedService`) asynchronously dequeues shares from the buffer and writes them to a persistent database (e.g., PostgreSQL) in batches. This decouples share submission from database I/O latency.

### 4.2. Database Schema

A robust, partitioned database is critical for querying the share window efficiently.

* **`Shares` Table (Partitioned by `poolid` and `created` timestamp)**:
  * `id` (BIGINT, PK)
  * `poolid` (VARCHAR)
  * `blockheight` (BIGINT, INDEXED)
  * `difficulty` (DOUBLE) - The share's score.
  * `networkdifficulty` (DOUBLE) - Network difficulty at time of submission.
  * `miner` (VARCHAR, INDEXED)
  * `created` (TIMESTAMPTZ, INDEXED)
* **`Blocks` Table**:
  * `id` (BIGINT, PK)
  * `blockheight` (BIGINT, UNIQUE)
  * `networkdifficulty` (DOUBLE)
  * `status` (VARCHAR) - e.g., 'Pending', 'Confirmed', 'Orphaned'.
  * `reward` (DECIMAL)
  * `transactionfees` (DECIMAL)
  * `created` (TIMESTAMPTZ)
* **`Balances` Table**:
  * `address` (VARCHAR, PK)
  * `amount` (DECIMAL) - Atomically updated.
* **`Payments` Table**:
  * `id` (BIGINT, PK)
  * `address` (VARCHAR, INDEXED)
  * `amount` (DECIMAL)
  * `transactionhash` (VARCHAR)

### 4.3. Scalability & Performance

* **Asynchronous Operations**: Use `async/await` for all I/O-bound tasks (network, database).
* **Low-Allocation Networking**: Use `System.IO.Pipelines` for high-performance TCP stream processing in the Stratum server.
* **Horizontal Scaling**: Design stateless Stratum servers that can be placed behind a load balancer. State should be managed in the database and a distributed cache (e.g., Redis).
* **Database Scaling**: Use read replicas for read-heavy operations like displaying user statistics on a web dashboard.

-----

## 5. Edge Case Handling

### 5.1. Orphaned Blocks

* **Definition**: A valid block that is not accepted into the canonical blockchain due to a chain reorganization.
* **Payout Impact**: Orphaned blocks yield **zero reward** in a PPLNS system. The risk is borne by the miners.
* **Handling Procedure**:
  1. Mark newly found blocks as `Pending` in the database.
  2. Wait for a maturity threshold (e.g., 100 confirmations).
  3. Verify the block is still in the main chain.
  4. If yes, update status to `Confirmed` and queue for payout.
  5. If no, update status to `Orphaned` and do not pay out.

### 5.2. Lucky Rounds (Shares Found < N)

* If a block is found before the share window is "full" (i.e., total share score is less than the target capacity 'N'), the logic does not change.
* The total reward is distributed proportionally across the shares that *are* in the window. This results in a higher payout per unit of score, which is how miners are rewarded for the pool's good luck.

### 5.3. Network Difficulty Changes

* The system must use historical difficulty values for calculations to ensure fairness.
* **Share Score**: A share's score is calculated using the network difficulty *at the time the share was submitted*. This value must be stored with each share record.
* **Window Size**: The PPLNS window size for a given block's payout is determined by the network difficulty *at the time that block was found*.

-----

## 6. Security

### 6.1. Game-Theoretic Attacks

* **Pool Hopping**: PPLNS is inherently resistant to this attack.
* **Block Withholding (BWH)**: An attacker submits shares but discards solved blocks.
  * **PPLNS Resilience**: PPLNS is more resilient than PPS because the attacker also loses revenue from the withheld block, making the attack less profitable.
* **Share Withholding/Delay Attack**: A strategic miner withholds valid shares and submits them in a batch just before submitting a solved block, unfairly increasing their proportion in the window.
  * **Mitigation**: Requires statistical monitoring of miner performance (luck vs. hashrate) to detect anomalies. Advanced mitigation includes **Randomized PPLNS (RPPLNS)**, where new shares replace a *random* share in the window, not the oldest.
