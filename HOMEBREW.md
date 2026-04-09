# Homebrew Distribution Steps

## 1. Push code to GitHub

The module is already named `github.com/webbcam/git-autocommit`. Ensure the repo exists, is public, and has the current code pushed.

## 2. Create a Homebrew tap repo

Create a new public GitHub repo named `webbcam/homebrew-tap`.

## 3. Add GoReleaser

[GoReleaser](https://goreleaser.com/) handles cross-platform builds, GitHub Releases, and Homebrew formula updates automatically.

Add a `.goreleaser.yaml` to the root of this repo. It should:

- Build binaries for `darwin/amd64`, `darwin/arm64`, and `linux/amd64`
- Archive each binary as a `.tar.gz`
- Create a GitHub Release with the archives attached
- Generate and commit a Homebrew formula to `webbcam/homebrew-tap` under `Formula/git-autocommit.rb`

The formula should call `bin.install "git-autocommit"` in its install block.

## 4. Set up GitHub Actions

Add a workflow (e.g. `.github/workflows/release.yml`) that:

- Triggers on pushed tags matching `v*`
- Runs `goreleaser release`
- Has a `GITHUB_TOKEN` with write access to both this repo and `webbcam/homebrew-tap`

## 5. Tag and release

```sh
git tag v0.1.0
git push origin v0.1.0
```

The Actions workflow will build the binaries, publish the GitHub Release, and update the tap formula automatically.

## 6. Verify installation

Once the release is live:

```sh
brew tap webbcam/tap
brew install git-autocommit
```
