# Stratum V1 Protocol Specification

This document provides a technical overview of the Stratum V1 protocol for Bitcoin pooled mining. It is intended for developers implementing mining pool servers or client-side mining software.

## 1. Protocol Fundamentals

### 1.1. Transport and Framing

- **Transport**: Communication occurs over a persistent, stateful TCP socket. The standard URI scheme is `stratum+tcp://`. Some pools offer an optional TLS-encrypted connection, often using `stratum+ssl://` or `stratum+tls://`. By default, the connection is unencrypted.
- **Framing**: All messages are JSON objects serialized into strings. Each message MUST be terminated by a single newline character (`\n`). Receiving endpoints must buffer data until a newline is received to parse a complete message.

### 1.2. JSON-RPC 2.0 Structure

The protocol uses a JSON-RPC 2.0 message structure. Messages are bidirectional and asynchronous.

- **Request**: A message from client to server that expects a response.
  - `id`: A unique identifier (number or string) for matching responses.
  - `method`: The name of the method to be invoked (e.g., `mining.authorize`).
  - `params`: An array of parameters for the method.
- **Response**: A message from server to client replying to a request.
  - `id`: The `id` from the corresponding request.
  - `result`: The data returned on success. Mutually exclusive with `error`.
  - `error`: An error object on failure. Mutually exclusive with `result`.
- **Notification**: A message (typically server-to-client) that does not require a response.
  - `id`: Must be `null`.
  - `method`: The name of the method (e.g., `mining.notify`).
  - `params`: An array of parameters.

## 2. Core Concepts

### 2.1. Distributed Nonce Space

Stratum expands the 32-bit nonce found in the block header by utilizing part of the coinbase transaction. This creates a larger, distributed nonce space.

- **ExtraNonce1 (Per-Connection Nonce)**: A hex-encoded string provided by the pool during the `mining.subscribe` handshake. It is unique to the miner's connection and prevents work collision between miners.
- **ExtraNonce2 (Miner-Controlled Nonce)**: A counter generated and incremented by the miner. The required byte-size is specified by the pool in the `mining.subscribe` response. When the 32-bit hardware nonce is exhausted, the miner increments `ExtraNonce2`, rebuilds the coinbase transaction and Merkle root, and restarts hashing.
- **nOnce (Hardware-Iterated Nonce)**: The standard 32-bit nonce field in the block header, which is iterated by the mining hardware (ASIC).

### 2.2. Share Difficulty

Pools assign a lower difficulty target for "shares" to track miner contributions without requiring them to find a full block.

- **mining.set_difficulty**: A notification from the pool to the miner setting the share difficulty. The share target is calculated as `(2^256 - 1) / difficulty`.
- **Vardiff (Variable Difficulty)**: A common pool-side implementation where the share difficulty is dynamically adjusted based on a miner's hashrate to maintain a stable share submission rate, optimizing network overhead.

## 3. Typical Session Flow

1. **Connect**: Miner establishes a TCP connection to the pool server.
2. **Subscribe**: Miner sends a `mining.subscribe` request.
3. **Acknowledge Subscription**: Pool responds with `ExtraNonce1` and `ExtraNonce2_size`.
4. **Authorize**: Miner sends a `mining.authorize` request with worker credentials.
5. **Confirm Authorization**: Pool responds with authorization status (`true`/`false`).
6. **Set Difficulty**: Pool sends a `mining.set_difficulty` notification.
7. **Notify Job**: Pool sends a `mining.notify` notification with the first block template.
8. **Mine**: Miner constructs the block header and begins hashing, iterating `nOnce`. If the `nOnce` space is exhausted, it increments `ExtraNonce2`, rebuilds the header, and continues.
9. **Find Share**: Miner finds a hash below the pool's share target.
10. **Submit Share**: Miner sends a `mining.submit` request with the solution.
11. **Validate Share**: Pool validates the share and responds with `true` (accepted) or `false` (rejected).
12. **New Block on Network**: When a new block is found elsewhere, the pool sends a new `mining.notify` job, usually with `clean_jobs` set to `true`, instructing the miner to abort all previous work.

## 4. Message Specification

### 4.1. Client-to-Server Methods

#### `mining.subscribe`

Initial message sent by the miner to register for work.

**Request**:

```json
{
  "id": 1,
  "method": "mining.subscribe",
  "params": ["my-miner/1.0.0", null]
}
```

**Response**:

```json
{
  "id": 1,
  "result": [["mining.notify", "subscription_id"], "ab010203", 4],
  "error": null
}
```

- `result` (Array): Subscription details. Can often be ignored.
- `result[1]` (String): Hex-encoded `ExtraNonce1`.
- `result[2]` (Integer): Required size of `ExtraNonce2` in bytes.

#### `mining.authorize`

Authenticates a worker. Must be successful before submitting shares.

**Request**:

```json
{
  "id": 2,
  "method": "mining.authorize",
  "params": ["wallet_address.worker_name", "x"]
}
```

**Response**:

```json
{
  "id": 2,
  "result": true,
  "error": null
}
```

- `result` (Boolean): `true` on success, `false` on failure.

#### `mining.submit`

Submits a valid share to the pool.

**Request**:

```json
{
  "id": 4,
  "method": "mining.submit",
  "params": ["wallet_address.worker_name", "bf", "00000001", "5a54a978", "1a2b3c4d"]
}
```

- `params[0]` (String): Worker name.
- `params[1]` (String): `job_id` from the `mining.notify` message.
- `params[2]` (String): Hex-encoded `ExtraNonce2`.
- `params[3]` (String): Hex-encoded `nTime` (timestamp).
- `params[4]` (String): Hex-encoded `nOnce`.

**Response**:

```json
{
  "id": 4,
  "result": true,
  "error": null
}
```

- `result` (Boolean): `true` if share is accepted, `false` if rejected.

### 4.2. Server-to-Client Notifications

#### `mining.notify`

Pushes a new work template to the miner.

**Notification**:

```json
{
  "id": null,
  "method": "mining.notify",
  "params": [
    "bf",
    "4128bf63...",
    "01000000...",
    "...000000",
    [],
    "20000000",
    "1800c29f",
    "5a54a978",
    true
  ]
}
```

**Parameters**:

| Index | Parameter       | Type    | Description                                                                |
| ----- | --------------- | ------- | -------------------------------------------------------------------------- |
| 0     | `job_id`        | String  | ID for this job, to be used in `mining.submit`.                            |
| 1     | `prev_hash`     | String  | Hex-encoded hash of the previous block. Often requires byte-reversal.      |
| 2     | `coinb1`        | String  | First part of the coinbase transaction.                                    |
| 3     | `coinb2`        | String  | Second part of the coinbase transaction.                                   |
| 4     | `merkle_branch` | Array   | Array of hex-encoded Merkle branch hashes for calculating the Merkle root. |
| 5     | `version`       | String  | Hex-encoded block version.                                                 |
| 6     | `nbits`         | String  | Hex-encoded packed network difficulty.                                     |
| 7     | `ntime`         | String  | Hex-encoded Unix timestamp.                                                |
| 8     | `clean_jobs`    | Boolean | If `true`, miner must abort all prior jobs.                                |

#### `mining.set_difficulty`

Adjusts the share difficulty for the miner.

**Notification**:

```json
{
  "id": null,
  "method": "mining.set_difficulty",
  "params": [difficulty]
}
```

- `params[0]` (Number): The new difficulty value.

## 5. Miner-Side Block Construction Algorithm

1. **Assemble Coinbase Transaction**: Concatenate the following hex strings: `coinb1` + `ExtraNonce1` + `ExtraNonce2` + `coinb2`. The `ExtraNonce2` must be padded with leading zeros to the length specified by `ExtraNonce2_size`.
2. **Calculate Coinbase Hash**: Decode the assembled coinbase string from hex to raw bytes and perform a double-SHA256 hash (SHA256d) on it.
3. **Calculate Merkle Root**: Start with the coinbase hash from the previous step. For each hash in the `merkle_branch` array, concatenate it with the current hash and perform a SHA256d on the result. The final hash is the Merkle root. If `merkle_branch` is empty, the coinbase hash is the Merkle root.
4. **Assemble and Hash Block Header**: Construct the 80-byte block header by concatenating the following fields (multi-byte fields must be little-endian):
   - `version` (4 bytes)
   - `prevhash` (32 bytes)
   - `merkle_root` (32 bytes)
   - `ntime` (4 bytes)
   - `nbits` (4 bytes)
   - `nOnce` (4 bytes)
5. **Proof-of-Work**: Perform a SHA256d on the 80-byte header. If the resulting hash is less than or equal to the share target, a valid share has been found.

## 6. Implementation Conventions & Security

### 6.1. Password Field Usage

The password field in `mining.authorize` is not used for security. It has been repurposed as a side-channel for configuration.

- **Placeholder**: Often, a simple placeholder like `x` is used and accepted by pools.
- **Configuration**: It is commonly used to pass key-value parameters to the pool, such as `d=8192` to suggest an initial difficulty.

### 6.2. Transport Security

By default, Stratum V1 operates over unencrypted TCP, which exposes it to several attacks.

- **Eavesdropping**: An attacker can monitor traffic to infer a miner's hashrate and earnings.
- **Hashrate Hijacking**: A man-in-the-middle (MITM) attacker can intercept `mining.submit` messages and replace the worker's username to steal shares and rewards.
- **Mitigation**: Some pools offer a TLS-encrypted endpoint (`stratum+ssl://`) to protect against these attacks, but this is an extension and not universally supported.

## 7. Appendix

### A.1.randum Error Codes

The `error` field in a response is an array `[code, message, data]`.

| Code   | Message              | Meaning                                              |
| ------ | -------------------- | ---------------------------------------------------- |
| 20     | Other/Unknown        | Generic server-side error.                           |
| 21     | Job not found        | Submitted share is for a stale/expired job.          |
| 22     | Duplicate share      | The exact same share has already been submitted.     |
| 23     | Low difficulty share | Share does not meet the current difficulty target.   |
| 24     | Unauthorized worker  | The worker is not authorized.                        |
| 25     | Not subscribed       | The client sent a message before `mining.subscribe`. |
| -32600 | Invalid Request      | The JSON sent is not a valid Request object.         |
| -32601 | Method not found     | The requested RPC method does not exist.             |
| -32602 | Invalid params       | Invalid parameters were sent for the RPC method.     |
| -32700 | Parse error          | The server received invalid/malformed JSON.          |

## Works Cited

- Stratum mining protocol - Bitcoin Wiki, accessed June 12, 2025, <https://en.bitcoin.it/wiki/Stratum_mining_protocol>
- Stratum V2: The New Era of Bitcoin Mining, accessed June 12, 2025, <https://blog.areabitcoin.co/stratum-v2/>
- Stratum: The Bitcoin Mining Protocol - Cointribune, accessed June 12, 2025, <https://www.cointribune.com/en/stratum-the-bitcoin-mining-protocol/>
- Features | Stratum V2 The next-gen protocol for pooled mining, accessed June 12, 2025, <https://stratumprotocol.org/features/>
- Message flows involved in stratum protocol. Upper diagram runs over a... - ResearchGate, accessed June 12, 2025, <https://www.researchgate.net/figure/Message-flows-involved-in-stratum-protocol-Upper-diagram-runs-over-a-TCP-connection_fig1_343895376>
- Stratum Protocol - Warthog Network, accessed June 12, 2025, <https://docs.warthog.network/developers/integrations/pools/stratum/>
- Trick or treat? Unveil the “Stratum” of the mining pools - Forum of Incident Response and Security Teams, accessed June 12, 2025, <https://www.first.org/resources/papers/amsterdam2019/FIRST-TC-pres-v1.1.pdf>
- All miners connect to pools using a protocol called stratum. This is ..., accessed June 12, 2025, <https://news.ycombinator.com/item?id=22161675>
- EthereumStratum-2.0.0/README.md at master · AndreaLanfranchi/EthereumStratum-2.0.0 · GitHub, accessed June 12, 2025, <https://github.com/AndreaLanfranchi/EthereumStratum-2.0.0/blob/master/README.md>
- Stratum | æternity Documentation Hub, accessed June 12, 2025, <https://docs.aeternity.com/developer-documentation/protocol/stratum>
- Stratum Protocol · bonesoul/CoiniumServ Wiki · GitHub, accessed June 12, 2025, <https://github.com/bonesoul/CoiniumServ/wiki/Stratum-Protocol>
- IsServer in sv1_api - Rust - Docs.rs, accessed June 12, 2025, <https://docs.rs/sv1_api/latest/sv1_api/trait.IsServer.html>
- STRATUM.md - aeternity/protocol - GitHub, accessed June 12, 2025, <https://github.com/aeternity/protocol/blob/master/STRATUM.md>
- Stratum Protocol, accessed June 12, 2025, <https://reference.cash/mining/stratum-protocol>
- Specifications/EthereumStratum_NiceHash_v1.0.0.txt at master - GitHub, accessed June 12, 2025, <https://github.com/nicehash/Specifications/blob/master/EthereumStratum_NiceHash_v1.0.0.txt>
- zone117x/node-stratum-pool: High performance Stratum ... - GitHub, accessed June 12, 2025, <https://github.com/zone117x/node-stratum-pool>
- Hardening Stratum, the Bitcoin Pool Mining Protocol, accessed June 12, 2025, <https://users.cs.fiu.edu/~carbunar/bedrock.pdf>
- Hardening Stratum, the Bitcoin Pool Mining Protocol - ResearchGate, accessed June 12, 2025, <https://www.researchgate.net/publication/315456462_Hardening_Stratum_the_Bitcoin_Pool_Mining_Protocol>
- Questions on Merkle root hashing for Stratum pools - Bitcoin Stack Exchange, accessed June 12, 2025, <https://bitcoin.stackexchange.com/questions/109820/questions-on-merkle-root-hashing-for-stratum-pools>
- Stratum protocol - problem with implementation in python : r/Bitcoin - Reddit, accessed June 12, 2025, <https://www.reddit.com/r/Bitcoin/comments/el8a2x/stratum_protocol_problem_with_implementation_in/>
- stratum password : r/BitAxe - Reddit, accessed June 12, 2025, <https://www.reddit.com/r/BitAxe/comments/1b3udew/stratum_password/>
- Question about AxeOS stratum password : r/BitAxe - Reddit, accessed June 12, 2025, <https://www.reddit.com/r/BitAxe/comments/1ifcsl6/question_about_axeos_stratum_password/>
- How can the miner access the host like stratum+tcp:// directly - Stack Overflow, accessed June 12, 2025, <https://stackoverflow.com/questions/23126284/how-can-the-miner-access-the-host-like-stratumtcp-directly>
