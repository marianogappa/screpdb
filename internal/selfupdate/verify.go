package selfupdate

import (
	"encoding/hex"
	"fmt"
	"strings"

	"aead.dev/minisign"
)

// minisignPublicKey is the embedded public half of the key whose secret half
// signs every release's SHA256SUMS (see README "Verifying downloads"). Pinning
// it here is what makes self-update trustworthy: the binary only applies an
// update whose checksums carry a signature from this exact key.
const minisignPublicKey = "RWS9gPPOydPD/tR8JBOelXKhif526NoAKY18dau7QHR4dqg84QMhJ5L/"

func verifySignature(sumsData, sigData []byte) error {
	var pk minisign.PublicKey
	if err := pk.UnmarshalText([]byte(minisignPublicKey)); err != nil {
		return fmt.Errorf("parse embedded minisign public key: %w", err)
	}
	if !minisign.Verify(pk, sumsData, sigData) {
		return fmt.Errorf("minisign signature verification failed")
	}
	return nil
}

// checksumFor returns the SHA-256 digest recorded for the named asset in a
// SHA256SUMS file (the `sha256sum`-style "<hex>  <name>" format).
func checksumFor(sumsData []byte, asset string) ([]byte, error) {
	for _, line := range strings.Split(string(sumsData), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) != 2 {
			continue
		}
		// sha256sum prefixes binary-mode entries with "*"; tolerate both.
		name := strings.TrimPrefix(fields[1], "*")
		if name != asset {
			continue
		}
		sum, err := hex.DecodeString(fields[0])
		if err != nil {
			return nil, fmt.Errorf("malformed checksum for %s: %w", asset, err)
		}
		return sum, nil
	}
	return nil, fmt.Errorf("no checksum entry for %s in SHA256SUMS", asset)
}
