# REST-based block/header download spec (port 30304)

## Overview

A simpler block/header download path that uses REST over port 30304 alongside the existing P2P downloader. Clients use **connected P2P peers (30303)** and assume each has REST on **30304**, with round-robin to query and download headers and blocks.

---

## 1. REST API (peer-server side)

### 1.1 Headers

| Endpoint | Method | Query params | Response |
|----------|--------|--------------|----------|
| Single header | GET | `number={blockNumber}` | RLP-encoded single header |
| Header range | GET | `from={blockNumber}&count={count}` | RLP-encoded list of headers (same as `BlockHeadersPacket`) |

- No reverse, skip, or hash-based origin. Only block number and optional count.
- Enforce existing limits: `maxHeadersServe`, `softResponseLimit`.

### 1.2 Blocks

| Endpoint | Method | Query params | Response |
|----------|--------|--------------|----------|
| By number | GET | `number={blockNumber}` | RLP-encoded block body (same as `GetBodyRLP`) |
| By hash | GET | `hash=0x{blockHash}` | RLP-encoded block body |

- Optional: `/blocks?number=n1,n2,...` or `?hash=0x1,0x2,...` for batch; response = RLP list of bodies.
- Enforce: `maxBodiesServe`, `softResponseLimit`.

### 1.3 Status (optional)

- **GET** `/status` or `/height` → chain height and/or head hash so the client knows what to request.

### 1.4 Wire format

- Response body = raw RLP bytes (no RLPX; RLP only). Same encoding as current `BlockHeadersMsg` / `BlockBodiesMsg` payloads.
- Port: **30304** (configurable).

---

## 2. Peer-server implementation (reuse existing logic)

- **Location:** `eth/protocols/eth/handlers.go` (and/or new `eth/protocols/eth/serve.go`).
- **Shared helpers:** e.g. `HeadersByNumber(chain, from, count)`, `BodyByNumber(chain, number)`, `BodyByHash(chain, hash)` using `backend.Chain()` and existing limits.
- **REST server:** New package (e.g. `eth/restsync`); HTTP server on 30304; routes call shared helpers and write RLP. Use same `Backend` / `BlockChain` as P2P handlers.
- **Lifecycle:** Start REST server only when config enables it (e.g. `--restsync.listen`), alongside existing node services.

---

## 3. Client: automatic round-robin over a list of REST peers

### 3.1 Configuration

- **Peer list:** Derived from connected P2P peers: client uses `p2p.Server.Peers()`, builds `http://<host>:30304` per peer. No RestSyncPeers flag or config.
- **No single “primary” URL:** The client always uses the **list** and selects peers via round-robin (and optionally failover).

### 3.2 Round-robin behavior

- **State:** Client maintains an ordered list of peer URLs and an index (or counter) for “next peer to use.”
- **Per request:** For each logical request (e.g. “headers from N, count M” or “block by hash H”):
  1. Pick the next peer in round-robin order (e.g. `peers[index % len(peers)]`; then increment index).
  2. Perform the HTTP GET to that peer’s URL (e.g. `http://peer:30304/headers?from=N&count=M`).
  3. On success: decode RLP, return data to the downloader/syncer; consider this peer “healthy” for the next cycle.
  4. On failure (connection error, 4xx/5xx, timeout, invalid RLP): optionally mark this peer as temporarily failed and **immediately try the next peer in the list** for the **same** request (retry same request on next peer). After exhausting the list once, return error or optionally do one more full round.
- **Result:** Load is spread across all configured peers; if one peer is down or slow, the next request automatically uses another peer, and the next request after that rotates again.

### 3.3 Optional refinements

- **Per-peer failure tracking:** Track consecutive failures or failure rate per URL; temporarily skip a peer in the round-robin until a cooldown expires (e.g. try again after 30s).
- **Sticky session (optional):** For a given “sync session” or “origin”, optionally stick to one peer until it fails, then switch; or keep strict round-robin. Spec recommends **strict round-robin** for simplicity and even load.
- **Discovery:** No automatic discovery of REST peers in this spec; the list is explicitly configured. Future extension could add a “discovery” endpoint that returns other REST peer URLs.

### 3.4 Integration with downloader

- **REST peer set:** The client presents itself to the downloader as one or more logical “peers” (e.g. a single `RestSyncPeerSet` that internally round-robins over the URL list).
  - When the downloader asks for “headers from N, count M”, the REST client:
    - Picks next URL from the list.
    - GET `{baseURL}/headers?from=N&count=M`.
    - On success, injects `headerPack{peerID: restPeerId, headers}` into `headerCh`.
    - On failure, tries next URL for the same request; after full round with no success, reports error.
  - Same for bodies: `RequestBodies(hashes)` → for each hash (or batched), pick next URL, GET `/block?hash=...`, inject `bodyPack` or equivalent.
- **Peer ID:** Use a stable logical ID for the “REST peer set” (e.g. `"restsync"` or `"rest-0"`) so the downloader can attribute deliveries and optionally enforce single-origin rules if desired.

### 3.5 Coexistence with P2P

- When `RestSyncPeers` is non-empty, the REST round-robin client is registered as an additional source (e.g. additional “peer(s)”) alongside existing P2P peers. Existing P2P discovery and downloader logic remain unchanged; the downloader may use both P2P peers and the REST peer set.

---

## 4. Summary of updates (round-robin)

| Item | Detail |
|------|--------|
| **Config** | No peer list config. REST URLs are derived from connected P2P peers: `http://<peer_host>:30304`. |
| **Selection** | Round-robin: next request uses `peers[index % len(peers)]`; advance index after each request (or after each successful request, depending on chosen policy). |
| **Failover** | On request failure (network, HTTP error, bad RLP), retry same request on next peer in list; optionally skip unhealthy peers for a cooldown period. |
| **Automatic connection** | No explicit “connect” step; “connection” is implicit: when a request is needed, pick next peer from the list and perform HTTP GET. No long-lived connection; each request is independent. |
| **Query and download** | All header and block queries and downloads go through this round-robin list; the downloader (or standalone syncer) calls the REST client, which hides the multi-peer selection. |

---

## 5. Files / packages (unchanged from base plan, with round-robin in client)

| Area | Location | Action |
|------|----------|--------|
| Server – shared logic | `eth/protocols/eth/handlers.go` or new `serve.go` | Add `HeadersByNumber`, `BodyByNumber`, `BodyByHash`; reuse in REST. |
| Server – REST | New package (e.g. `eth/restsync/server.go`) | HTTP server on 30304; routes `/headers`, `/block`, optional `/status`. |
| Config | `node/config.go` or `eth/ethconfig` | Add `RestSyncPeers []string`, `RestSyncListen` (enable server). |
| **Client – round-robin** | **e.g. `eth/downloader/restpeer.go` or `eth/restsync/client.go`** | **Maintain ordered list of base URLs and round-robin index; implement `Peer` (or equivalent) that, on each `RequestHeadersByNumber` / `RequestBodies`, selects next peer, performs GET, on failure tries next peer(s), then injects result into downloader channels.** |
| Node lifecycle | Where HTTP/stack starts | Start REST server if `RestSyncListen`; if `RestSyncPeers` non-empty, register REST peer set (round-robin client) with downloader. |

---

## 6. Request/response RLP format

- **Headers:** RLP-encode `[]*types.Header` as `BlockHeadersPacket`.
- **Block body:** One RLP value = `GetBodyRLP(hash)` (one element of `BlockBodiesRLPPacket`).
- **Multiple blocks:** RLP list of such body values.
- No message code or RLPX frame on HTTP; body = raw RLP only.

---

## 7. Security and limits

- Apply `maxHeadersServe`, `maxBodiesServe`, `softResponseLimit` on server.
- Optional: bind 30304 to internal/VPN IP; or simple token/auth for REST.
- Rate limiting per client IP (or per token) to avoid abuse.
