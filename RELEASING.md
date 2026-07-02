# Releasing

Releases are cut by pushing a `vX.Y.Z` tag (or publishing a GitHub Release);
[`.github/workflows/release.yml`](.github/workflows/release.yml) builds the
cross-platform binaries, signs `SHA256SUMS`, uploads the assets, and bumps both
the Scoop manifest and the Homebrew formula.

## One-time setup

### Homebrew tap (`HOMEBREW_TAP_TOKEN`)

The Homebrew formula lives in a **separate, screpdb-dedicated tap repo** because
Homebrew requires the `homebrew-<name>` naming convention. The release workflow
pushes the bumped formula there on each tagged release.

1. **Create the tap repo:** a new public repo named **`marianogappa/homebrew-screpdb`**.
   Homebrew maps the install command `brew install marianogappa/screpdb/screpdb`
   to this repo, reading the formula from `Formula/screpdb.rb`. The workflow
   creates `Formula/` and the file on first run, so the repo can start empty
   (a README is fine).

2. **Create a token** with write access to *only* that repo — a
   [fine-grained personal access token](https://github.com/settings/tokens?type=beta):
   - **Resource owner:** `marianogappa`
   - **Repository access:** *Only select repositories* → `homebrew-screpdb`
   - **Permissions:** *Repository permissions → Contents → Read and write*
   - Set an expiry and calendar a rotation reminder.

   (A classic PAT with the `repo` scope also works but grants access to every
   repo — prefer the fine-grained, single-repo token above.)

3. **Add it as a secret** on the **screpdb** repo:
   *Settings → Secrets and variables → Actions → New repository secret*
   - **Name:** `HOMEBREW_TAP_TOKEN`
   - **Value:** the token from step 2.

The default `GITHUB_TOKEN` can't be used here because it only grants access to
the repo running the workflow, and the tap is a different repo. If the secret is
absent the release still succeeds — the formula-bump step logs a warning and
skips.

### Minisign signing key (`MINISIGN_SECRET_KEY`)

`SHA256SUMS` is signed with [minisign](https://jedisct1.github.io/minisign/) so
downloads can be verified (see the "Verifying downloads" section of the README).
Store the minisign **secret key** as a repository secret named
`MINISIGN_SECRET_KEY`. If absent, the release still succeeds but publishes no
`SHA256SUMS.minisig`. The matching public key is embedded in the binary and
printed in the README.

## Cutting a release

```bash
git tag vX.Y.Z
git push origin vX.Y.Z
```

The workflow does the rest. Verify afterward that the release assets, the Scoop
manifest bump (`bucket/screpdb.json`), and the Homebrew formula
(`marianogappa/homebrew-screpdb`) all updated.
