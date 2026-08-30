// Package bnetfacade is the sanctioned surface for SC:R Battle.net bridge
// communication and GCS replay downloads in screpdb's Go binary (issue #317).
//
// It provides two constrained HTTP clients:
//
//   - A loopback client that talks only to 127.0.0.1 on a discovered port,
//     path-prefixed to /web-api/. This reaches SC:R's local web-api bridge.
//
//   - An outbound client allowlisted to exactly one host
//     (storage.googleapis.com) and one path prefix
//     (/starcraft-user-uploads-prod/S1-replays/). Downloaded replay bytes are
//     validated (length + seRS magic at offset 12) before being returned.
//
// Both clients reject anything outside their constraints at the facade
// boundary. This package is exempt from TestNoDirectIOOutsideFacades alongside
// internal/netfacade and internal/selfupdate.
//
// Bridge payloads may contain non-UTF-8 bytes (map titles in cp949 or latin-1),
// so DecodeBridgeJSON performs lenient decoding before JSON unmarshalling.
//
// Rate limiting (issue #319) is enforced here at the facade boundary so no
// caller can bypass it, with two separate budgets: bridge calls ride the
// user's Blizzard session (disconnection risk — conservative token bucket,
// persisted daily cap, exponential cooldown on the bridge's explicit "Rate
// Limited" signal), while GCS downloads only cost bandwidth (separate, looser
// budget). Both buckets serve PriorityUser waiters before background sweeps.
// ProbeBridge is deliberately unmetered: it is a local liveness check.
// EnableBudgetPersistence makes the daily counters survive restarts.
package bnetfacade
