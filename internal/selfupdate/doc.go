// Package selfupdate is the single sanctioned surface for in-binary updates in
// screpdb (issue #212). It is the only package permitted to make outbound
// network calls beyond the localhost readiness probe (it queries the GitHub
// Releases API and downloads the matching asset) and the only package permitted
// to write outside the iofacade roots (it atomically swaps the running binary in
// its own install directory).
//
// Like internal/iofacade and internal/netfacade, this package is exempt from the
// TestNoDirectIOOutsideFacades guard (issue #135) and is the documented
// chokepoint for the privileged operations it performs. Every byte it downloads
// is verified against the release's SHA256SUMS, which is itself verified against
// an embedded minisign public key before any swap touches disk; a tampered
// download fails verification regardless of which host served it.
//
// Self-update is always user-initiated: the binary checks for a newer version on
// launch (a read-only API call) and surfaces a notice, but never downloads or
// swaps anything until the user clicks Update in the dashboard. Package-manager
// installs (e.g. Scoop) and non-writable install directories are detected and
// excluded so the updater never fights another package manager.
package selfupdate
