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

### Scoop-bump GitHub App (`SCOOP_APP_ID`, `SCOOP_APP_PRIVATE_KEY`)

The release workflow bumps [`bucket/screpdb.json`](bucket/screpdb.json) by opening
a PR that auto-merges once the required `test` check passes. A PR opened by the
default `GITHUB_TOKEN` has its checks gated behind a manual *"Approve workflows to
run"* click, which stalls auto-merge — so the PR is opened by a **GitHub App**
instead (a trusted actor whose PR checks run automatically). If these secrets are
absent the release still succeeds; the bump PR is opened with the default token
and simply needs one manual approval.

1. **Create a GitHub App** — *[Settings → Developer settings → GitHub Apps](https://github.com/settings/apps)
   → New GitHub App*:
   - **Name:** anything unique, e.g. `screpdb-scoop-bump`.
   - **Homepage URL:** the repo URL (any valid URL works).
   - **Webhook:** untick **Active** (none needed).
   - **Repository permissions:** *Contents → Read and write* and
     *Pull requests → Read and write*. Nothing else.
   - **Where can this GitHub App be installed?** *Only on this account.*
   - Click **Create GitHub App**, then note the **App ID** near the top.

2. **Generate a private key** — on the App's page, scroll to *Private keys →
   Generate a private key*. A `.pem` file downloads.

3. **Install the App** — App page → *Install App* → install on `marianogappa` →
   *Only select repositories* → **screpdb**.

4. **Add two secrets** on the **screpdb** repo (*Settings → Secrets and variables
   → Actions → New repository secret*):
   - `SCOOP_APP_ID` — the App ID from step 1.
   - `SCOOP_APP_PRIVATE_KEY` — the full contents of the `.pem` from step 2.

The App only has write access to this one repo and can only touch contents and
pull requests, so it cannot bypass the branch ruleset — the bump PR still merges
through the normal `test` gate.

## Cutting a release

```bash
git tag vX.Y.Z
git push origin vX.Y.Z
```

The workflow does the rest. Verify afterward that the release assets, the Scoop
manifest bump (`bucket/screpdb.json`), and the Homebrew formula
(`marianogappa/homebrew-screpdb`) all updated.
