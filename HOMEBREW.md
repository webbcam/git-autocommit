# Homebrew Distribution Steps

## 1. Push code to GitHub

The module is already named `github.com/webbcam/git-autocommit`. Ensure the repo exists, is public, and has the current code pushed.

## 2. Create a Homebrew tap repo

Create a new public GitHub repo named `webbcam/homebrew-tap`.

## 3. Create a Personal Access Token (PAT)

GoReleaser needs write access to `webbcam/homebrew-tap` to push the formula. The default `GITHUB_TOKEN` only covers the current repo.

1. Go to GitHub → Settings → Developer settings → Personal access tokens → Fine-grained tokens
2. Create a token with **Contents: Read and Write** access scoped to the `webbcam/homebrew-tap` repo
3. In the `webbcam/git-autocommit` repo, go to Settings → Secrets and variables → Actions
4. Add a secret named `HOMEBREW_TAP_GITHUB_TOKEN` with the token value

## 4. GoReleaser config

A `.goreleaser.yaml` is included at the root of this repo. It:

- Builds binaries for `darwin/amd64` and `darwin/arm64`
- Archives each binary as a `.tar.gz`
- Creates a GitHub Release with the archives attached
- Generates and commits a Homebrew formula to `webbcam/homebrew-tap` under `Formula/git-autocommit.rb`

## 5. GitHub Actions workflow

A workflow at `.github/workflows/release.yml`:

- Triggers on pushed tags matching `v*`
- Runs `goreleaser release`
- Uses `GITHUB_TOKEN` (auto-provided) for the release and `HOMEBREW_TAP_GITHUB_TOKEN` (from step 3) for the tap

## 6. Tag and release

```sh
git tag v0.1.0
git push origin v0.1.0
```

The Actions workflow will build the binaries, publish the GitHub Release, and update the tap formula automatically.

## 7. Verify installation

Once the release is live:

```sh
brew tap webbcam/tap
brew install git-autocommit
```
