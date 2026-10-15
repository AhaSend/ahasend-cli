# Install channels for the AhaSend CLI

Users install `ahasend` with the tool they already use: Homebrew, a curl
script, AUR, .deb/.rpm files, Scoop, WinGet, Chocolatey, a PowerShell script
or npm. Every channel installs the same release binaries. Once npm and
WinGet have had a one-time manual first publish and Microsoft has merged the
first WinGet PR, one stable tag push publishes all of them; Chocolatey's
moderators then review each version before it goes live.

## Channels

| Channel | User command | Platforms | Home | Published by |
|---|---|---|---|---|
| Homebrew | `brew install ahasend/tap/ahasend` | macOS, Linux; amd64, arm64 | formula in `AhaSend/homebrew-tap` | `publish-channel.yml` (file from GoReleaser `brews`) |
| curl script | `curl -fsSL https://ahasend.com/install.sh \| sh` | macOS, Linux; amd64, arm64 | `install.sh`, a release asset | the GitHub release |
| AUR | `yay -S ahasend-bin` (or any AUR helper) | Arch x86_64, aarch64 | AUR package `ahasend-bin` | `publish-channel.yml` (files from GoReleaser `aurs`) |
| .deb / .rpm | `sudo apt install ./ahasend_<version>_amd64.deb` | Linux; amd64, arm64 | release assets | the GitHub release |
| Scoop | `scoop bucket add ahasend https://github.com/AhaSend/scoop-bucket` then `scoop install ahasend/ahasend` | Windows; x64, arm64 | manifest in `AhaSend/scoop-bucket` | `publish-channel.yml` (file from GoReleaser `scoops`) |
| WinGet | `winget install AhaSend.CLI` | Windows; x64, arm64 | `microsoft/winget-pkgs` | `publish-channel.yml` (Komac) |
| Chocolatey | `choco install ahasend` | Windows; x64 (runs under emulation on arm64) | community.chocolatey.org | `publish-channel.yml` (choco) |
| PowerShell script | `irm https://ahasend.com/install.ps1 \| iex` | Windows; x64, arm64 | `install.ps1`, a release asset | the GitHub release |
| npm | `npm install -g @ahasend/cli` | macOS, Linux, Windows; x64, arm64 | `@ahasend/cli` and six platform packages | `publish-channel.yml` (npm trusted publishing) |

The Docker image on GHCR and manual downloads from the release page are not
package channels; Release design covers how the release produces them.

Out of scope: Snap, Nix, a hosted apt/yum repository, homebrew-core, the Scoop
Main and Extras buckets, and code signing on macOS and Windows. The decision
log gives the reasons.

## Release design

### Terms

- `<tag>` is a git tag such as `v1.2.3`, and `<version>` is the same without
  the `v` (`1.2.3`). The install scripts' `--version` / `-Version` option and
  `AHASEND_VERSION` take a `<tag>`.
- `AHASEND_RELEASES_URL` is the base of every download URL. It defaults to
  `https://github.com/AhaSend/ahasend-cli/releases`; a release's assets are at
  `<AHASEND_RELEASES_URL>/download/<tag>/<asset>`, and the newest stable
  release is behind the redirect `<AHASEND_RELEASES_URL>/latest`. The install
  scripts and the scripts of the WinGet, Chocolatey and npm jobs read it
  when they run, and GoReleaser reads it when it generates the Homebrew
  formula, Scoop manifest and PKGBUILD, so a test or a mirror can point them
  elsewhere.
- A *published channel* is one that `publish-channel.yml` publishes:
  homebrew, scoop, aur, winget, chocolatey and npm. The curl script, the
  PowerShell script and the .deb/.rpm files come with the GitHub release.

### Kinds of tag

`scripts/release-kind.sh <tag>` decides a tag's kind. It prints:

- `prerelease` for a tag `vX.Y.Z-<suffix>` (for example `v0.4.0-rc.1`);
- `latest` for a stable tag `vX.Y.Z` that is not lower than any other stable
  tag in the repository (compared with `sort -V`);
- `older` for a stable tag lower than another stable tag (a fix release
  for an older line);

and fails for any other tag. Every workflow decision that depends on the
kind calls it, directly or through the other scripts named below. It reads
the repository's tags, so every job that calls it checks out with
`fetch-depth: 0`.

| | `latest` | `older` | `prerelease` |
|---|---|---|---|
| GitHub release (`scripts/make-latest.sh`) | published, marked latest | published, not marked latest | published as a pre-release |
| Docker image (`scripts/docker-tags.sh`) | `<tag>`, `<version>` and `latest` | `<tag>`, `<version>` | `<tag>`, `<version>` |
| published-channel runs started (`scripts/channels.sh`) | all six | all six | none |

`scripts/make-latest.sh <tag>` prints `true` for `latest` and `false` for
the other kinds. The `goreleaser` job passes it to `release.make_latest` as
the `MAKE_LATEST` environment variable. It always prints a value, because
GoReleaser sends no `make_latest` for an empty one, and GitHub then marks the
release latest. GoReleaser's `prerelease: auto` sets the GitHub pre-release
flag itself; it looks for the same `-` suffix, so it agrees with
`release-kind.sh`. Together they keep the install scripts, which follow
GitHub's "latest" release, and Docker's `latest` tag from moving back to an
older version.

### One release, then one run per channel

A tag push `vX.Y.Z` runs `.github/workflows/release.yml`. Every GoReleaser
run, there and in `publish-channel.yml`, uses `goreleaser/goreleaser-action`
at `GORELEASER_VERSION` on `ubuntu-latest`, and sets
`GORELEASER_CURRENT_TAG` to the tag, so a stable tag on a commit that also
carries a pre-release tag builds the stable version.

1. `test`: the Makefile's `deps-check`, `notices-check` and
   `check-versions` targets, `make test`, the snapshot check and the script
   tests (both in Testing), and `scripts/release-notes.sh <tag>` for the
   pushed tag. Nothing publishes unless this passes.
2. `goreleaser` (permissions `contents: write`, `id-token: write`,
   `attestations: write`): runs `scripts/release-notes.sh <tag>` itself for
   its notes file. GoReleaser OSS, run with
   `--skip=homebrew,scoop,aur`, builds the binaries, archives and packages
   and publishes the GitHub release with every asset. Then
   `scripts/stage-release.sh` collects the release assets (see Assets) and
   `actions/attest-build-provenance` attests them. When a step after the
   publish fails, the job can be re-run: GoReleaser finds the published
   release, and `release.replace_existing_artifacts: true` lets it upload
   the same files again, which the reproducible build leaves unchanged.
3. `docker`: runs after `goreleaser` succeeds, so an image is never pushed
   for a tag whose GitHub release failed. It builds and pushes the image
   with the tags that `scripts/docker-tags.sh <tag>` prints.
4. `channels` (permissions `contents: read` and `actions: write`): runs
   after `goreleaser` succeeds. For each published channel that
   `scripts/channels.sh <tag>` prints, it starts
   `gh workflow run publish-channel.yml --ref <tag> -f tag=<tag> -f channel=<name>`.

### Publishing a channel

`.github/workflows/publish-channel.yml` publishes one published channel for
one tag, with permission `contents: read` (plus `id-token: write` for npm).
The `channels` job starts it, and a person can start it by hand to retry a
channel. Each run is named after the tag and the channel, so a failed
channel shows as its own failed run and does not stop the others. Runs for
the same channel queue behind each other, so two tags never publish one
channel at the same time: each channel has its own `concurrency` group with
`queue: max`, because the default keeps only one waiting run and cancels
the one before it. All its jobs run on `ubuntu-latest`, except Chocolatey,
which runs on `windows-latest`.

It works in four steps.

1. **Gates.** `scripts/channel-gates.sh <tag> <ref>` fails with a message
   naming the check when the tag is a pre-release, when the run's ref is not
   `refs/tags/<tag>` (so the templates and scripts would not be the tag's),
   or when `gh release view <tag>` finds no published release for the tag.
   The run's read-only token cannot see a draft, so a draft counts as
   missing.
2. **Prepare.**
   - homebrew, scoop, aur: one `goreleaser release --skip=publish` builds
     everything and writes the formula, the Scoop manifest, and the AUR files
     into `dist/` without publishing them. `scripts/check-rebuild.sh` compares
     `dist/checksums.txt` with the release's `checksums.txt`, and stops the
     run if they differ.
   - winget, chocolatey, npm: `scripts/fetch-release.sh <tag> <dir>`
     downloads `checksums.txt` and every asset it lists from
     `<AHASEND_RELEASES_URL>/download/<tag>/`, as the install scripts do, and
     fails if any of them does not match. These jobs never run GoReleaser;
     they use the release's own assets. The npm job packs the seven
     packages and the WinGet job fills in its manifests, and both upload the
     result as a workflow artifact, so a run that fails later still leaves
     it for a manual publish (Human setup, steps 3 and 5).
3. **Already published?** The job checks whether the channel already has
   this version, and if so, stops with success, so a run can repeat safely.
   `scripts/channel-published.sh <channel> <version>` makes this check for
   every published channel except npm, whose check is per package inside
   `scripts/npm-publish.mjs`. It prints `published`, `missing`, or, for
   winget only, `no-package` when `AhaSend.CLI` is not in `winget-pkgs` at
   all, which stops the job (see Channel details, WinGet). The environment
   variables `AHASEND_TAP_REPO`, `AHASEND_BUCKET_REPO` and
   `AHASEND_AUR_REPO` replace the git URLs of the tap, the bucket and the
   AUR repo, here and in step 4, so tests can use local repos:

   | Channel | Already published when | Not published when |
   |---|---|---|
   | homebrew | the `version` in `Formula/ahasend.rb` of `AhaSend/homebrew-tap` is the same or newer | the file is missing or older |
   | scoop | the `version` in `ahasend.json` of `AhaSend/scoop-bucket` is the same or newer | the file is missing or older |
   | aur | `pkgver` in `.SRCINFO` of the AUR repo is the same or newer | the repo has no `.SRCINFO` (cloning a package that does not exist yet gives an empty repo), or the version is older |
   | winget | `komac list-versions AhaSend.CLI` lists the version, or an open PR to `microsoft/winget-pkgs` has `AhaSend.CLI` and the version, as a whole word, in its title (`gh search prs`) | Komac lists other versions only (`missing`), or reports that the package does not exist (`no-package`) |
   | chocolatey | `https://community.chocolatey.org/packages/ahasend/<version>` exists, which is also true while the version waits in moderation | that page returns 404 |
   | npm | per package, `npm view <package>@<version> version` succeeds | per package, it fails with E404 (for the package or the version) |

   Homebrew, Scoop and AUR hold one version at a time, so a newer one counts
   as published, and a retry of an older tag never moves them back. WinGet
   and Chocolatey keep every version and serve the newest. npm keeps every
   version; Channel details says how a version older than the `latest`
   dist-tag is published.
4. **Publish.**
   - homebrew, scoop, aur: `scripts/push-channel.sh <channel>` commits the
     files GoReleaser generated in step 2, whose build was checked, to the
     tap, the bucket or the AUR repo.
   - winget, chocolatey, npm: as Channel details describes.

### Assets

The raw binaries and `checksums.txt` have the same names in every release,
so manual downloads and links in the docs work for every version.
`.goreleaser.yaml` has two builds, `unix` (linux and darwin) and `windows`,
and each archive and the nfpm packages select builds with `ids`.
`release.github` names `AhaSend/ahasend-cli`, so a run in a clone whose
`origin` is elsewhere, such as a test clone, still knows the repository. The
release assets are:

| Asset | Made by | Used by |
|---|---|---|
| `ahasend-{linux,darwin,windows}-{amd64,arm64}` (`.exe` on Windows) | archive `raw` (`formats: [binary]`, `ids: [unix, windows]`) | manual download, install scripts, npm, Chocolatey |
| `ahasend_<version>_{linux,darwin}_{amd64,arm64}.tar.gz` | archive `unix-tar` (`ids: [unix]`) | Homebrew, AUR |
| `ahasend_<version>_windows_{amd64,arm64}.zip` | archive `windows-zip` (`ids: [windows]`) | Scoop, WinGet |
| `ahasend_<version>_{amd64,arm64}.deb`, `ahasend-<version>-1.{x86_64,aarch64}.rpm` | `nfpms` (`ids: [unix]`, linux only; `file_name_template` below) | apt, dnf, rpm |
| `LICENSE`, `THIRD_PARTY_NOTICES`, `install.sh`, `install.ps1` | extra files | everyone |
| `checksums.txt` | `checksum` | everyone |

- Every package ships `LICENSE` and `THIRD_PARTY_NOTICES` with the binary.
  The tar.gz and zip archives contain both; Channel details and the .deb and
  .rpm bullet below say where each package puts them. The install scripts
  install the binary alone, as a manual download does; both files are
  release assets next to it.
- The tar.gz also has `completions/ahasend.bash`, `completions/_ahasend`
  and `completions/ahasend.fish`.
- The .deb and .rpm put the binary in `/usr/bin/ahasend` and the licence
  files in `/usr/share/doc/ahasend/`. Completions go to
  `/usr/share/bash-completion/completions/ahasend`,
  `/usr/share/fish/vendor_completions.d/ahasend.fish`, and
  `/usr/share/zsh/vendor-completions/_ahasend` in the .deb or
  `/usr/share/zsh/site-functions/_ahasend` in the .rpm (two nfpm `contents`
  entries with `packager: deb` and `packager: rpm`).
  `nfpms.maintainer` is `AhaSend <info@ahasend.com>`.
- The .deb and .rpm names follow Debian's and Fedora's conventions but keep
  `<version>` as it is: `nfpms.file_name_template` writes them, because
  nfpm's own `ConventionalFileName` turns a pre-release's `-rc.1` into
  `~rc.1`, and GitHub renames a `~` in an asset name, so the name would no
  longer match `checksums.txt`.
- `checksums.txt` has the SHA-256 of every other asset, in `sha256sum`
  format.
- `.goreleaser.yaml` lists the extra files once, under a YAML anchor that
  both `release.extra_files` and `checksum.extra_files` use. So the
  checksums cover them, including in a snapshot, which skips the release
  step.
- Every download URL in the generated formula, manifest and PKGBUILD is
  `{{ envOrDefault "AHASEND_RELEASES_URL" "<default>" }}/download/v{{ .Version }}/<asset>`.
- `scripts/stage-release.sh <dist> <out>` copies the release assets into
  `<out>` under their published names: `checksums.txt` and every name it
  lists. It finds each built asset's path in `dist/artifacts.json`, because
  GoReleaser keeps raw binaries in per-build folders; a name that
  `artifacts.json` does not list is an extra file, taken from the
  repository root. The `goreleaser` job attests `<out>`, and the
  tests serve it as a fake release.

### Version and reproducible builds

- The ldflags stamp `internal/version.Version` with `v{{ .Version }}`, so a
  release prints `AhaSend CLI vX.Y.Z`. A snapshot's version is
  `{{ envOrDefault "SNAPSHOT_VERSION" (printf "%s-snapshot+%s" (incpatch .Version) .ShortCommit) }}`,
  so by default it prints `AhaSend CLI vX.Y.(Z+1)-snapshot+<commit>`.
- `GitCommit` is the first 7 characters of the commit hash
  (`git rev-parse HEAD | cut -c1-7`), and `BuildTime` is the commit time in
  UTC, in RFC 3339
  (`TZ=UTC git log -1 --format=%cd --date=format-local:%Y-%m-%dT%H:%M:%SZ`).
  `scripts/build-info.sh` prints both. The Makefile's `build` and
  `release-snapshot`, every workflow step that runs GoReleaser (as the
  `GIT_COMMIT` and `BUILD_TIME` environment variables) and the Docker job's
  build arguments take them from it. So every channel prints the same
  `--version`.
- Publishing Homebrew, Scoop and AUR needs reproducible builds: the
  GoReleaser runs that `scripts/check-rebuild.sh` compares use the same
  GoReleaser version and Go toolchain, may run on different runner images,
  and must give the same `checksums.txt`. These settings make that so:
  - binaries: `CGO_ENABLED=0`, `-trimpath`, and
    `mod_timestamp: '{{ .CommitTimestamp }}'`;
  - every archive file: `builds_info` and each `files[].info` set
    `owner: root`, `group: root`, a fixed `mode` (0755 for the binary, 0644
    for the rest) and `mtime: '{{ .CommitDate }}'`;
  - every nfpm package: nfpm `mtime`, and each content's `file_info` with
    the same owner, group, mode and mtime;
  - the .rpm: `nfpms.rpm.buildhost` is fixed (`ahasend-release`), because
    nfpm otherwise records the machine's host name.
- `.gitattributes` gives `*.sh` `eol=lf`, because the Windows jobs and
  tests run shell scripts in Git Bash, which fails on CRLF line endings that
  a Windows checkout would otherwise write.
- A `before` hook, `scripts/completions.sh`, builds the CLI for the host into
  a temporary folder outside the work tree, and writes the three completion
  files into `completions/` from `ahasend completion`.
- Generated files either stay out of the work tree or go in folders listed
  in `.gitignore` (`dist/`, `completions/`, and `coverage.html`, which
  `make test-coverage` writes). An untracked file would make a release run
  refuse to start and would change Go's embedded `vcs.modified` flag.
  Snapshots do not check this, so `make release-snapshot` checks
  `git status --porcelain` itself: it fails when the output is not empty and
  `CI` is set, and prints a warning otherwise, so local runs work beside
  untracked notes. The release notes file and any downloaded
  `checksums.txt` go in `$RUNNER_TEMP`.

### Tool versions

- The Go toolchain is the `toolchain` line of `go.mod`. Because that line is
  only a minimum, every release build runs with `GOTOOLCHAIN=go<toolchain>`:
  `make release-snapshot` sets it the same way the Makefile's `notices`
  target does, and every workflow that builds sets it in `env`.
- The Makefile holds `GORELEASER_VERSION := v2.18.3`, and `KOMAC_VERSION`
  and `KOMAC_SHA256` (the SHA-256 of Komac's linux x64 release binary).
  `make release-snapshot` runs that GoReleaser version with `go run`.
  `scripts/goreleaser-check.sh` installs the same version into a temporary
  `GOBIN` and runs `goreleaser check` from there, because `go run` turns the
  program's exit code 2 into 1. It passes when the check exits 0, or exits 2
  with `brews` as the only deprecated property it reports. The WinGet job
  downloads that Komac binary and checks it against `KOMAC_SHA256` before
  running it.
- The Makefile's `check-go-version` target becomes `check-versions`.
  `make check-versions` checks that the Dockerfile and every workflow file
  that uses `actions/setup-go` name `go.mod`'s toolchain, that every
  `GOTOOLCHAIN` set in a workflow names it too, that every workflow file
  that uses `goreleaser/goreleaser-action` names `GORELEASER_VERSION`, and
  that the WinGet job uses `KOMAC_VERSION` and `KOMAC_SHA256`.

### Release notes

`scripts/release-notes.sh <tag>` writes the release notes, which GoReleaser
reads with `--release-notes`:

- for a stable tag `vX.Y.Z`, the section whose heading is `## [X.Y.Z]`,
  with or without a ` - YYYY-MM-DD` date, in `CHANGELOG.md`;
- for a pre-release, the `## [Unreleased]` section.

It writes the section without its heading, and appends the definitions of
the reference-style links the section uses, which `CHANGELOG.md` keeps at
its end. It rewrites links to repository files, such as
`USAGE.mdx#exit-codes`, to
`https://github.com/AhaSend/ahasend-cli/blob/<tag>/<path>`, so they work on
the release page. It fails if the section is missing or empty, and the
release then fails.

`release.footer` links to the installation docs and shows how to check a
download against `checksums.txt`. It names no channel, because it is
written at tag time, before the published channels have run.

### Attestations

The attestation covers every staged asset. Users who want more than a
checksum check can run
`gh attestation verify <file> --repo AhaSend/ahasend-cli`. The install
scripts check SHA-256 only, because attestations need `gh` on the user's
machine.

### Credentials

All channel secrets live in `publish-channel.yml`; the release itself needs
none of them.

| Secret / setting | Channel | What it is |
|---|---|---|
| `RELEASE_APP_ID`, `RELEASE_APP_PRIVATE_KEY` | Homebrew, Scoop | A GitHub App with contents write on `AhaSend/homebrew-tap` and `AhaSend/scoop-bucket`. The job gets a short-lived token with `actions/create-github-app-token`. |
| `AUR_SSH_PRIVATE_KEY` | AUR | A dedicated ed25519 key without a passphrase, registered on the AhaSend AUR account. |
| `WINGET_TOKEN` | WinGet | A classic PAT with `public_repo` from the bot account that owns the `winget-pkgs` fork. Komac cannot use fine-grained tokens. |
| `CHOCOLATEY_API_KEY` | Chocolatey | The API key of the AhaSend account on community.chocolatey.org. |
| environment `npm` | npm | Trusted publishing (OIDC), no token. Each npm package trusts `AhaSend/ahasend-cli`, workflow `publish-channel.yml`, environment `npm`. |

## Channel details

### Homebrew

- A formula, not a cask (see Decision log), at `Formula/ahasend.rb` in
  `AhaSend/homebrew-tap` (`brews.directory: Formula`). `brews.repository`
  names that repo even though GoReleaser never pushes it, because GoReleaser
  writes no formula when the repository name is empty.
- `brews.ids: [unix-tar]`, because the `raw` archive's binaries would
  otherwise also match and GoReleaser fails on two artifacts for one
  platform.
- The formula downloads the tar.gz for the platform. It installs `ahasend`,
  the three completion files, and `LICENSE` and `THIRD_PARTY_NOTICES` with
  `doc.install`. Its `test do` block checks `ahasend --version`.
- Users install with the full name `ahasend/tap/ahasend`, which since
  Homebrew 6 also trusts the formula.

### AUR

- The package is `ahasend-bin`, with `provides=(ahasend)` and
  `conflicts=(ahasend)`. Its arches are `x86_64` and `aarch64`, with the
  linux tar.gz and its `sha256sums` per arch (`aurs.ids: [unix-tar]`, for
  the same reason as `brews.ids`). The licence is `Apache-2.0`.
- `package()` (the `aurs.package` template) installs the binary, the
  completions (zsh into `/usr/share/zsh/site-functions/`), and `LICENSE` and
  `THIRD_PARTY_NOTICES` to `/usr/share/licenses/ahasend-bin/`.
- GoReleaser writes `dist/aur/ahasend-bin.pkgbuild` and
  `dist/aur/ahasend-bin.srcinfo`. `scripts/push-channel.sh aur` commits them
  as `PKGBUILD` and `.SRCINFO` to
  `ssh://aur@aur.archlinux.org/ahasend-bin.git`; the first push creates the
  package.

### Scoop

- The bucket is `AhaSend/scoop-bucket`, with one manifest, `ahasend.json`.
- The manifest has `architecture.64bit` and `architecture.arm64` with the
  zip URL and hash, and `"bin": "ahasend.exe"`. Scoop keeps the zip's
  licence files in the app folder.

### WinGet

- The identifier is `AhaSend.CLI`. Its manifest (schema 1.12.0) has one
  installer per architecture (x64, arm64), each pointing at the zip with its
  SHA-256: `InstallerType: zip`, `NestedInstallerType: portable`, and
  `NestedInstallerFiles` with `ahasend.exe` and
  `PortableCommandAlias: ahasend`. WinGet keeps the zip's licence files with
  the exe.
- `packaging/winget/` holds templates of the version, default-locale and
  installer manifests. `scripts/winget-manifest.sh <tag> <assets-dir> <out>`
  fills them in; that is what the job uploads as its artifact.
- When `scripts/channel-published.sh` prints `no-package`, the job fails,
  saying the first version must be submitted by hand from that artifact
  (Human setup, step 3).
- Once the package exists, the job runs
  `komac update AhaSend.CLI --version <version> --urls <x64 zip> <arm64 zip> --submit`,
  with `WINGET_TOKEN` passed as Komac's `GITHUB_TOKEN`. Komac copies the
  metadata of the previous version in `winget-pkgs`, sets the new URLs and
  hashes, and opens a PR from the bot's fork to `microsoft/winget-pkgs`.
- Microsoft's checks require that the installer URL can be found from the
  publisher's site, so ahasend.com must link to the GitHub releases (Human
  setup, step 7).

### Chocolatey

- The package is `ahasend`. It embeds the raw `ahasend-windows-amd64.exe`
  asset as `tools/ahasend.exe` (Chocolatey creates the `ahasend` shim), with
  `tools/LICENSE.txt`, `tools/THIRD_PARTY_NOTICES.txt` and a
  `tools/VERIFICATION.txt` that gives that asset's release URL and its
  SHA-256 from `checksums.txt`.
- No install script is needed: install and uninstall only add and remove the
  shim.
- `packaging/chocolatey/` holds the nuspec and `VERIFICATION.txt` templates.
  `scripts/choco-package.sh <tag> <assets-dir> <out>` fills them in and
  copies the exe and licence files. The job then runs `choco pack` and
  `choco push --source https://push.chocolatey.org/`.

### npm

- The wrapper is `@ahasend/cli`:
  - `bin: {"ahasend": "bin/ahasend.js"}`;
  - `optionalDependencies` pinned to the exact version of the six platform
    packages;
  - `engines.node: ">=18"`;
  - no install scripts.
- The platform packages are `@ahasend/cli-{linux,darwin,win32}-{x64,arm64}`.
  Each has `os`, `cpu` and `preferUnplugged: true`. There is no `libc`
  field, because the binaries are static.
- All seven have `license: "Apache-2.0"`,
  `repository: {"type": "git", "url": "git+https://github.com/AhaSend/ahasend-cli.git"}`,
  and `LICENSE` and `THIRD_PARTY_NOTICES` in `files`; the platform packages
  also list `bin/ahasend` (`bin/ahasend.exe` on Windows). The registry
  rejects a trusted publish whose `repository.url` does not match the
  repository that built it.
- `bin/ahasend.js`:
  - maps `process.platform` and `process.arch` to the platform package, and
    finds its binary with `require.resolve`;
  - `AHASEND_BINARY_PATH` replaces that lookup;
  - if no platform package is installed, explains that optional dependencies
    were left out and names the other install channels;
  - runs the binary with the asynchronous `child_process.spawn` and
    inherited stdio, so its signal handlers can run while the binary does;
  - ignores SIGINT and does not forward it, because a terminal Ctrl+C
    already reaches the binary, which is in the same process group;
  - forwards SIGTERM and SIGHUP to the binary;
  - when the binary exits, exits with its code; when a signal ended it,
    removes its own handlers for that signal and re-raises it, so the
    launcher ends the same way.
- This signal handling has two limits. The npm instructions in the docs
  state both, and point users who hit them to a channel that installs the
  binary directly:
  - a SIGINT sent to the launcher alone (by a supervisor that signals one
    process) does not reach the binary;
  - a supervisor that sends SIGTERM to the whole process group (such as
    `timeout`) makes the binary receive it twice, and `messages send` and
    `messages cancel` treat a second SIGTERM as "stop at once". SIGHUP has
    no such limit: the CLI does not handle it, so the first one ends the
    binary.
- Templates live in `npm/`.
  `scripts/npm-packages.mjs <tag> <assets-dir> <out>` copies each binary in
  (mode 755 for macOS and Linux; the job runs on Linux, so the mode holds)
  with the licence files, and writes the version into all seven
  `package.json` files.
- The job uses Node 24 with an exact npm 11 release no older than 11.5.1,
  pinned in the workflow, because trusted publishing needs npm 11.5.1, which
  not every runner image ships.
- `scripts/npm-publish.mjs` publishes those of the seven packages that lack
  the version, platform packages first and the wrapper last, each with
  `npm publish --access public`. A version older than the package's
  `latest` dist-tag on the registry is published with `--tag legacy`, so
  that dist-tag does not move. Provenance comes from trusted publishing.

### curl script (`install.sh`)

- POSIX `sh` with `set -eu`. All code is in functions and the last line is
  `{ main "$@"; }`, so a download that stops anywhere, even inside that
  line, runs nothing.
- It needs `curl` or `wget`, plus `uname`, `mktemp`, `mkdir`, `cp`, `mv`,
  `rm`, `chmod`, `awk` and `readlink`, and `sysctl` on macOS. It uses `dpkg`,
  `rpm` and `pacman` when they exist. It hashes with `sha256sum`, else
  `shasum -a 256`, else `openssl dgst -sha256`.
- **Platform:** `uname -s` and `uname -m`, mapped to linux/darwin and
  amd64/arm64. On macOS, `sysctl -n hw.optional.arm64` = 1 picks arm64 even
  under Rosetta. Any other system fails, naming the releases page.
- **Version:**
  - Without `--version`, it follows the redirect of
    `<AHASEND_RELEASES_URL>/latest` once and takes the tag from the final
    URL. It does not use the GitHub API, which has rate limits. That tag must
    be stable (`^v[0-9]+\.[0-9]+\.[0-9]+$`).
  - With `--version <tag>` (or `AHASEND_VERSION`), it uses that tag,
    pre-releases included.
  - It downloads from `<AHASEND_RELEASES_URL>/download/<tag>/`.
- **Install directory:** `--install-dir` or `AHASEND_INSTALL_DIR`, default
  `${XDG_BIN_HOME:-$HOME/.local/bin}`, created if missing. It never runs
  `sudo`. A directory it cannot write to fails, with a hint, and so does one
  that starts with `~`, which the shell does not expand after
  `--install-dir=`.
- **Package-manager installs:** the script never replaces a file a package
  manager owns. An existing file is owned when:
  - it is a symbolic link (the script only writes regular files, while
    Homebrew, npm and others install links);
  - it is in `/usr/bin` or `/nix/store`;
  - `dpkg -S`, `rpm -qf` or `pacman -Qo` claims it.

  The script checks ownership before it checks that it can write, on the
  target `<install-dir>/ahasend` as given and, when the directory exists, on
  its real path, because a package manager knows a file by its real path
  (`/bin/ahasend` is `/usr/bin/ahasend` where `/bin` links to `/usr/bin`).
  When the target is owned, the script exits nonzero with the
  upgrade command of the manager it recognises, or a general message:
  - `brew upgrade ahasend/tap/ahasend` for a link into a Homebrew `Cellar`;
  - `npm install -g @ahasend/cli` for a link into `node_modules`;
  - for a file `dpkg -S` claims, `sudo apt install ./<deb>`, and for one
    `rpm -qf` claims, `sudo rpm -U ./<rpm>`, each naming the release's
    download URL, because there is no hosted repository;
  - `yay -S ahasend-bin` for a file `pacman -Qo` claims.
- **Other copies:** when another `ahasend` comes earlier on `PATH`, owned or
  not (such as a manual download in `/usr/local/bin`), the script installs
  and then warns which copy runs first, naming its manager when it has one.
- **Steps:**
  1. Download the binary and `checksums.txt` into a `mktemp -d` directory.
  2. Find the binary's line in `checksums.txt`, and fail if it is missing or
     does not match.
  3. Copy the binary into the install directory under a temporary name, make
     it executable, and run `--version` on it.
  4. `mv -f` it over `ahasend`. On any earlier failure the old binary stays
     in place and works.

  A trap on EXIT, HUP, INT and TERM removes the download directory and the
  temporary copy in the install directory, so a closed terminal leaves
  nothing behind either.
- **PATH:** if the install directory is not on `PATH`, it prints the line to
  add for sh, bash and zsh, and for fish. It never edits shell profiles.
- **Flags:** `--version`, `--install-dir`, `--quiet` (prints only errors),
  `--no-color`, `--help`. Colour is also off when `NO_COLOR` is set or
  `TERM` is `dumb`, as in the CLI. A `--version` that does not match
  `^v[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z.-]+)?$` fails before any download,
  and so does an empty `--version` or `--install-dir`, so an unset variable
  in `--version "$V"` never turns a pinned install into the latest one.
  Users pass flags with
  `curl -fsSL … | sh -s -- --version v0.4.0`.
- **Exit codes:** 0 on success; nonzero for an unsupported system, a failed
  download, a missing asset, a checksum mismatch, a directory it cannot write
  to, or an owned target.

### PowerShell script (`install.ps1`)

- It runs on Windows PowerShell 5.1, which `irm … | iex` uses by default,
  and on PowerShell 7. It is ASCII only, because Windows PowerShell 5.1
  decodes a download without a charset as ISO-8859-1.
- It follows `install.sh`, with Windows tools:
  - it detects x64 or arm64 from
    `[System.Runtime.InteropServices.RuntimeInformation]::OSArchitecture`;
  - it resolves the tag the same way and checks the hash with `Get-FileHash`;
  - it installs to `%LOCALAPPDATA%\Programs\AhaSend\bin`, replacing the old
    file only after the new one passes `--version`.
- It never replaces a file a package manager owns. On Windows a path is
  owned when it is under a Scoop `shims` folder, `WinGet\Links`,
  `chocolatey\bin` or the global npm prefix (`NPM_CONFIG_PREFIX`, else
  `npm prefix -g`, else `%APPDATA%\npm`). It refuses such a folder even
  when no `ahasend.exe` is there yet, because npm and Scoop put their own
  shims there. An owned target and other copies on `PATH` are handled as in
  `install.sh`, with the matching `scoop update`, `winget upgrade`,
  `choco upgrade` or `npm install -g` command.
- It prints the command that adds the directory to the user `PATH`, and does
  not run it. The command edits `HKCU:\Environment` directly, reading `Path`
  without expanding names such as `%USERPROFILE%` and writing it back as an
  expandable value, because `[Environment]::SetEnvironmentVariable` would
  freeze those names and store a plain string.
- All code is in functions and the last line calls the main one inside an
  `if ($true) { … }` block, so a download that stops anywhere, even inside
  that line, runs nothing.
- It never calls `exit`, which would close the user's PowerShell session
  under `iex`. It reports a failure with `throw`.
- It reads the redirect of `<AHASEND_RELEASES_URL>/latest` with one HEAD
  request that does not follow it, which works the same on 5.1 and 7.
- It takes the parameters `-Version`, `-InstallDir` and `-Quiet`, plus the
  `AHASEND_VERSION`, `AHASEND_INSTALL_DIR` and `AHASEND_RELEASES_URL`
  environment variables. An empty `-Version` or `-InstallDir` fails, as in
  `install.sh`. Parameters are passed with
  `& ([scriptblock]::Create((irm https://ahasend.com/install.ps1))) -Version v0.4.0`.

### Hosting the scripts

- `https://ahasend.com/install.sh` and `https://ahasend.com/install.ps1` are
  Cloudflare redirect rules to
  `https://github.com/AhaSend/ahasend-cli/releases/latest/download/install.sh`
  (and `.ps1`).
- The served script comes from the release GitHub marks latest, which is
  always the newest stable release (see Kinds of tag), and `checksums.txt`
  covers it.
- The docs also give the GitHub URL, and show how to download the script and
  read it before running it.

## Testing

- **Fake release:** packaging tests build a GoReleaser snapshot, stage it
  with `scripts/stage-release.sh`, and serve the staged folder with
  `test/install/fake-release`, a small Go server that runs on every runner
  and in Alpine. It serves the GitHub release layout (`/latest` redirects
  to `/tag/<tag>`, `/download/<tag>/<asset>`) on 127.0.0.1, and
  `AHASEND_RELEASES_URL` points at it.
- **Test version:** a default snapshot's version is a pre-release, which npm
  and Chocolatey treat specially. Every snapshot a test builds, the snapshot
  check's included, is therefore built with `SNAPSHOT_VERSION=99.0.0` and
  served as `v99.0.0`, so the asset names, the generated URLs and the
  binaries all carry the same stable version. Tests that need a second
  version (an upgrade, a pre-release returned as latest) serve one more
  snapshot built with another `SNAPSHOT_VERSION`.
- **Script tests:** every workflow decision lives in a script, and tests
  call the scripts directly: `scripts/release-kind.sh`,
  `scripts/make-latest.sh`, `scripts/docker-tags.sh`, `scripts/channels.sh`,
  `scripts/channel-gates.sh` (with a stubbed `gh`),
  `scripts/release-notes.sh`, `scripts/check-rebuild.sh`,
  `scripts/push-channel.sh`, `scripts/channel-published.sh`,
  `scripts/fetch-release.sh`, `scripts/choco-package.sh`,
  `scripts/goreleaser-check.sh`, `scripts/npm-publish.mjs` and
  `make check-versions`. Stages 1, 3 and 4 list the cases. Each test file is
  `test/release/<name>_test.sh`, a POSIX `sh` script that uses the helpers
  in `test/release/lib.sh`, with its fixtures in
  `test/release/fixtures/<topic>/`; `make test-scripts` runs them all.
- **Where they run:**
  - `ci.yml`, on every pull request, and the release `test` job: the
    snapshot check (`scripts/goreleaser-check.sh`, `make release-snapshot`,
    `scripts/stage-release.sh`, `scripts/check-release-assets.sh`) and every
    script test that needs no network or extra tool.
  - `.github/workflows/packaging.yml` (created in Stage 2), on pull
    requests that touch `.goreleaser.yaml`, `Makefile`, `go.mod`,
    `scripts/`, `npm/`, `packaging/`, `install.sh`, `install.ps1`,
    `test/install/` or the workflows: the reproducibility check, the
    Homebrew, Scoop, AUR, WinGet, Chocolatey, npm and install-script tests,
    and the script tests that need the network or Verdaccio.
  - Locally, inside the loopback-only network namespace (`unshare -rn`),
    after the Go toolchain and GoReleaser's modules are downloaded outside
    it once: the snapshot check (which needs `dpkg-deb` and `rpm`), the
    script tests that need no network, and the `install.sh` tests.
- **Pass rule:** `namcap` and `brew audit --strict` fail a test on errors,
  and their warnings are printed in the log. The one accepted `brew audit`
  problem is "`version` is redundant with version scanned from URL", because
  GoReleaser's formula template always writes `version` and the published
  formula is the generated file unchanged.
- **Home folder:** every packaging and install-script test runs with a
  temporary `HOME` (and `USERPROFILE` and `LOCALAPPDATA` on Windows), so a
  default install directory is never the developer's own.
- **Binary runs:** packaging tests run the binary only with `--version`,
  `--help`, `completion` and an unknown command, which make no network
  calls.

## Human setup

These steps need a person with company accounts. Until a channel's step is
done, its `publish-channel.yml` run fails, without affecting the release or
the other channels; Stage 5 describes how the first release's npm and WinGet
runs fail and how these steps finish them.

1. **GitHub:**
   - create the public repos `AhaSend/homebrew-tap` and
     `AhaSend/scoop-bucket`;
   - create the GitHub App, install it on both repos, and store its ID and
     key as `RELEASE_APP_ID` / `RELEASE_APP_PRIVATE_KEY`;
   - create the `npm` environment, allowing tags matching `v*`, with required
     reviewers if wanted.
2. **AUR:** register an AhaSend account (plus a personal co-maintainer), add
   the public key, and store the private key as `AUR_SSH_PRIVATE_KEY`.
3. **WinGet:**
   - create or choose a bot GitHub account, fork `microsoft/winget-pkgs`, and
     store a classic PAT (`public_repo`) as `WINGET_TOKEN`;
   - for the first version, open a PR to `microsoft/winget-pkgs` with the
     manifests the failed WinGet run uploaded, and sign the Microsoft CLA on
     it.
4. **Chocolatey:** register the AhaSend account and store the API key as
   `CHOCOLATEY_API_KEY`. The first version waits for human moderation, which
   can take days to weeks.
5. **npm:**
   - publish the seven packages once by hand from the tarballs the npm job
     uploads as a workflow artifact (2FA, account with write access to the
     `@ahasend` scope);
   - add a trusted publisher to each package (`AhaSend/ahasend-cli`,
     `publish-channel.yml`, environment `npm`);
   - set each package to "require 2FA and disallow tokens".
6. **Cloudflare:** add redirect rules for `ahasend.com/install.sh` and
   `ahasend.com/install.ps1`.
7. **Website:** make sure ahasend.com links to the GitHub releases page,
   which the WinGet check requires.

## Stage 1: GoReleaser release pipeline
**Goal**: `release.yml` builds and publishes the GitHub release through
GoReleaser, with the raw binaries, archives and .deb/.rpm packages listed in
Assets, release notes from `CHANGELOG.md`, attestations, and the tag kinds
of Kinds of tag for the GitHub release and the Docker image. This stage
publishes no package-manager channel.
**Success Criteria**:
- `scripts/goreleaser-check.sh` passes on `.goreleaser.yaml`.
- A staged snapshot has exactly the assets in Assets except `install.sh`
  and `install.ps1`, which Stage 2 adds to the extra files list, and
  `checksums.txt` covers every asset.
- `make build` builds for development, and `make release-snapshot` is the
  Makefile's only build for every platform. The Makefile's `release-prep`
  target is `clean fmt lint test-coverage release-snapshot`.
- `make check-versions` passes and is the version check that CI, the release
  `test` job, the README and the comments in the Makefile, the Dockerfile,
  `ci.yml` and `release.yml` name.
- `.gitattributes` gives `*.sh` `eol=lf`.
- `.gitignore` lists `dist/`, `completions/` and `coverage.html`, so
  `release-prep` leaves no untracked file.
- The `--version` example in `USAGE.mdx` shows the RFC 3339 build time, and
  `CHANGELOG.md` lists the RFC 3339 build time under Changed.
**Tests**:
- The snapshot check's `scripts/check-release-assets.sh <staged>` checks:
  - the exact asset names;
  - every asset has a `checksums.txt` line and every line matches its file;
  - `ahasend-linux-amd64 --version` prints `AhaSend CLI v99.0.0`, an RFC
    3339 build time and a 7-character commit;
  - the tar.gz contents by file name, owner and mode, and the zip contents
    by file name and mode (zip entries carry no owner);
  - `dpkg-deb -c` and `rpm -qlp` list `/usr/bin/ahasend`, the completion
    files at the paths in Assets, and the licence files.
- In a scratch clone where `v1.2.3-rc.1` and `v1.2.3` point at the same
  commit, `goreleaser release --skip=publish` with
  `GORELEASER_CURRENT_TAG=v1.2.3` builds a binary that prints
  `AhaSend CLI v1.2.3`.
- In a work tree with an untracked file, `make release-snapshot` fails when
  `CI` is set and warns when it is not.
- `scripts/release-kind.sh`, in a scratch repo with tags `v1.2.3` and
  `v1.3.0`, gives `latest` for `v1.3.0`, `older` for `v1.2.3`,
  `prerelease` for `v1.3.1-rc.1`, and fails for `1.2.3`, `v1.2`,
  `v1.2-rc.1` and `v1.2.3.4`.
- In the same repo, `scripts/make-latest.sh` prints `true` for `v1.3.0` and
  `false` for `v1.2.3` and `v1.3.1-rc.1`.
- `scripts/docker-tags.sh` prints the tag and the version for every kind,
  and `latest` for a `latest` tag only.
- `scripts/release-notes.sh`, on a fixture `CHANGELOG.md`, extracts
  `[1.2.3]` (with and without a date) for `v1.2.3` and `[Unreleased]` for
  `v1.2.4-rc.1`, rewrites a `USAGE.mdx#exit-codes` link to the tag's GitHub
  URL, keeps a reference-style link working, leaves out the heading, and
  fails for a version with no section and for an empty section.
- `scripts/goreleaser-check.sh` fails on a fixture config that also uses
  another deprecated property.
- `make check-versions` fails for a workflow with another Go version, one
  with another `GOTOOLCHAIN`, and one with another GoReleaser version.
**Status**: Complete

## Stage 2: Install scripts
**Goal**: `install.sh` and `install.ps1` behave as Channel details describe,
ship as release assets, and are tested offline on every platform they
support.
**Success Criteria**:
- Both scripts are in the staged snapshot and in `checksums.txt`.
- `shellcheck -s sh` and `checkbashisms` pass on `install.sh`.
  `PSScriptAnalyzer` reports no error or warning on `install.ps1` except
  `PSAvoidUsingWriteHost` (the script writes its messages for the user), and
  the file holds only ASCII bytes.
- `.github/workflows/packaging.yml` exists and runs these tests.
**Tests**:
- `test/install/run.sh` (sh) and `test/install/install.Tests.ps1` (Pester)
  run against the fake release. They cover:
  - a fresh install, by option and by environment variable
    (`AHASEND_VERSION`, `AHASEND_INSTALL_DIR`), and the default install
    directory (including `XDG_BIN_HOME` for `install.sh`);
  - an upgrade over an older binary;
  - pinning a version, including a pre-release tag, and an empty version or
    install directory, which fails;
  - `--quiet`, `--no-color` and `--help` for `install.sh`, and `-Quiet` for
    `install.ps1`;
  - the PATH hint when the directory is not on `PATH`;
  - a checksum mismatch, a missing asset and a 404 on latest, each leaving
    the old binary in place and failing;
  - a pre-release tag returned as latest, which is rejected;
  - a directory the script cannot write to;
  - an owned target, which is refused with the matching upgrade command:
    - for `install.sh`: a symbolic link into a fake Homebrew `Cellar`, one
      into a fake npm `node_modules`, files that fake `dpkg -S`, `rpm -qf`
      and `pacman -Qo` commands (first on `PATH`) claim, a file that a fake
      `dpkg -S` claims by its real path only, reached through a linked
      directory and through a path with `..`, and, in CI,
      `--install-dir /usr/bin` over an existing `/usr/bin/ahasend`, which
      gets the general message;
    - for `install.ps1`: a fake Scoop `shims` folder, a fake `WinGet\Links`
      folder, a fake `chocolatey\bin` folder, and a fake npm prefix
      (`NPM_CONFIG_PREFIX`);
  - another copy earlier on `PATH`, owned and not owned, which installs and
    warns;
  - a script cut off mid-file, which runs nothing;
  - no leftover temporary files, in the download directory or the install
    directory, after a success, after each failure, and after an INT, a
    TERM and a HUP sent during the download (`install.sh`).
- `install.sh` also runs with `wget` and no `curl` on `PATH`, and with each
  hashing tool missing in turn (`sha256sum`, then `shasum`) so it falls back
  to the next.
- On macos-latest (arm64), `install.sh` run under Rosetta
  (`arch -x86_64 /bin/sh install.sh`) installs the arm64 binary.
- A failing `install.ps1` run through `iex` in a child session leaves that
  session running, and the `[scriptblock]::Create(...) -Version` form passes
  its parameter.
- On Windows, the printed PATH command, run against a saved and restored
  user `Path`, keeps a `%SystemRoot%` entry unexpanded and the value an
  expandable string.
- `install.sh` runs on ubuntu-latest, ubuntu-24.04-arm, macos-latest
  (arm64), macos-15-intel, and under `dash` and Alpine's `busybox sh`.
- `install.ps1` runs on windows-latest (x64) and windows-11-arm, under both
  Windows PowerShell 5.1 and PowerShell 7.
**Status**: Complete

## Stage 3: Channel publishing, with Homebrew, Scoop and AUR
**Goal**: `publish-channel.yml` publishes Homebrew, Scoop and AUR for a
stable tag, and the `channels` job of `release.yml` starts it for those three.
**Success Criteria**:
- `goreleaser release --skip=publish` on a snapshot writes
  `dist/homebrew/Formula/ahasend.rb`, the Scoop `ahasend.json`, and
  `dist/aur/ahasend-bin.pkgbuild` and `dist/aur/ahasend-bin.srcinfo`,
  matching Channel details.
- `publish-channel.yml` runs the four steps of Publishing a channel in
  order.
**Tests**:
- **Homebrew** (macos-latest, macos-15-intel, ubuntu-latest): install the
  snapshot formula by its full name from a local tap (`<tap>/ahasend`). Then
  run `brew test` and `brew audit --strict` on it, check `ahasend --version`,
  the completion files and the licence files, and `brew uninstall`.
- **Scoop** (windows-latest): `scoop install` the snapshot manifest, check
  `ahasend --version` in a new shell and the licence files, then
  `scoop uninstall`.
- **AUR** (`archlinux:base-devel` container, x86_64): `makepkg -si` as a
  non-root user, `namcap` on the PKGBUILD and the package,
  `ahasend --version`, the completion and licence files exist, then
  `pacman -R`.
- **Gates:** `scripts/channel-gates.sh` refuses a pre-release tag, a ref
  that is not `refs/tags/<tag>`, and a tag with no published release.
- **Already published:** `scripts/channel-published.sh`, against a fixture
  tap, bucket and AUR repo, prints `missing` for a missing file, an empty
  repo and an older version, and `published` for the same version and a
  newer one.
- **Reproducibility:** in `packaging.yml`, two jobs on separate runners run
  the two builds that `scripts/check-rebuild.sh` compares, for a test tag
  `v99.0.0` added to the commit, each in a full clone: one runs the
  `goreleaser` job's command with `publish` added to its `--skip` list and
  with a release notes file; the other runs the GoReleaser command of
  Prepare in Publishing a channel. A third job runs
  `scripts/check-rebuild.sh` on their `checksums.txt` files.
- **Rebuild check and push:** `scripts/check-rebuild.sh` passes a matching
  `checksums.txt` and stops on a differing one; `scripts/push-channel.sh`
  commits the generated files, under their published names, to a local bare
  repo standing in for the tap, the bucket and the AUR repo, including an
  empty one.
- **Channels job:** `scripts/channels.sh` prints the three channels for a
  `latest` and an `older` tag, and none for a pre-release.
**Status**: Not Started

## Stage 4: WinGet, Chocolatey and npm
**Goal**: `publish-channel.yml` also publishes WinGet, Chocolatey and npm
from a release's assets, and `scripts/channels.sh` lists them.
**Success Criteria**:
- `npm/`, `packaging/winget/` and `packaging/chocolatey/` hold the templates,
  and `scripts/npm-packages.mjs`, `scripts/winget-manifest.sh` and
  `scripts/choco-package.sh` fill them in.
- Each of the three jobs gets the assets with `scripts/fetch-release.sh`
  before it packs, fills in or publishes anything.
**Tests**:
- **npm** (ubuntu-latest, ubuntu-24.04-arm, macos-latest, macos-15-intel,
  windows-latest on Node 24, and ubuntu-latest on Node 18), with a local
  Verdaccio registry that has no uplinks. One ubuntu-latest job packs the
  seven packages, as the npm job does, and the other jobs install those
  tarballs:
  - all seven filled-in `package.json` files have the `repository.url`,
    `license` and `files` in Channel details, and each packed tarball holds
    the licence files and, for macOS and Linux, a binary with mode 755;
  - publish the seven packed packages to Verdaccio, then
    `npm install -g @ahasend/cli` from it and check `ahasend --version`;
  - check that `npx @ahasend/cli --version` works, and that pnpm and bun
    install it the same way;
  - check that the launcher passes the binary's exit code through (`ahasend`
    with an unknown command exits 4), and that `AHASEND_BINARY_PATH` works;
  - on Linux and macOS, with `AHASEND_BINARY_PATH` pointing at a test
    script that records the signals it gets:
    - SIGINT sent to the process group reaches a script that handles it
      exactly once, and the launcher exits with the script's code;
    - SIGINT sent to the process group ends a script that does not handle
      it, and the launcher then ends by SIGINT;
    - SIGTERM and SIGHUP, each sent to the launcher alone, reach the script
      once, and the launcher ends the same way as the script;
    - SIGTERM sent to the process group reaches the script twice, the limit
      that Channel details states;
  - check that the missing-package message appears when optional
    dependencies are left out.
- **Chocolatey** (windows-latest): `scripts/choco-package.sh` on the fake
  release writes a `VERIFICATION.txt` with the exe's release URL and its
  `checksums.txt` hash; then `choco pack`, then
  `choco install ahasend --source <dir>`, check `ahasend --version` in a new
  shell and the licence files, then `choco uninstall ahasend`.
- **WinGet** (windows-latest): `scripts/winget-manifest.sh` on the fake
  release, then `winget validate`, then `winget install --manifest` (after
  `winget settings --enable LocalManifestFiles`, and after installing WinGet
  if the runner image lacks it) and `ahasend --version`.
- **Already published:**
  - the WinGet check prints `published` for a version of `jqlang.jq` that
    exists and, with `gh` stubbed, for a version that only an open PR adds;
    `missing` for a version that does not exist, including `1.2.3` while
    only a `1.2.30` PR is open; and `no-package` for a package that does not
    exist;
  - the Chocolatey check reports an approved `jq` version as published and a
    missing version as not published;
  - against Verdaccio, the npm check reports a package that does not exist
    at all (E404) as missing; given a version that three of the seven
    packages already have, `scripts/npm-publish.mjs` publishes only the other
    four, platform packages first and the wrapper last; and it publishes a
    version older than the `latest` dist-tag with `--tag legacy`, leaving
    that dist-tag unchanged.
- **Fetch:** `scripts/fetch-release.sh`, against the fake release, passes
  when every asset matches `checksums.txt` and fails when one asset is
  changed.
- **Channels job:** `scripts/channels.sh` prints all six published channels
  for a `latest` and an `older` tag.
- `make check-versions` fails for a WinGet job with another Komac version or
  hash.
**Status**: Not Started

## Stage 5: Accounts, first releases and docs
**Goal**: the accounts and secrets exist; the first stable release made
with this setup reaches every channel (npm and WinGet through their one-time
manual first publish, Chocolatey through moderation); the second stable
release publishes on every channel without manual steps; the docs list the
channels that are live. The stage closes after the second release.
**Success Criteria**:
- Human setup is done before the first release, except the hand publishes
  in steps 3 and 5, which use that release's artifacts.
- The first stable release publishes the GitHub release, the Docker image,
  Homebrew, Scoop, AUR, the deb/rpm assets and both install scripts, and
  pushes the Chocolatey package into moderation.
- Its npm run fails at the publish step, and its WinGet run fails when the
  check prints `no-package`, after each uploads its artifact. Human setup
  steps 3 and 5 publish both by hand from those artifacts.
- After that, `ahasend.com/install.sh` and `/install.ps1` install that
  release.
- The second stable release, made after Microsoft has merged the first
  WinGet PR, publishes every channel by itself: npm by trusted publishing,
  WinGet by a Komac PR, Chocolatey by a pushed update.
- The docs:
  - `cli/installation.mdx` in the docs repo, the README's Installation
    section and the Installation section of `USAGE.mdx` give each live
    channel's install, upgrade and uninstall commands;
  - WinGet and Chocolatey are added once Microsoft and Chocolatey accept
    them;
  - the README and `USAGE.mdx` keep their binary download and
    build-from-source instructions, and `cli/installation.mdx` its download,
    build-from-source and `go install` ones;
  - for browser downloads on macOS, the docs show
    `xattr -d com.apple.quarantine`;
  - the npm instructions state the signal limits in Channel details.
- `CHANGELOG.md` lists the new channels under Added for the first release.
**Tests**:
- On a clean machine of each kind (a GitHub runner is enough), install the
  first release from every live channel with the real URLs, check
  `ahasend --version` and `ahasend --help` in a new shell, upgrade to the
  second release, and uninstall. Use a temporary home folder (`HOME`, and
  `USERPROFILE` on Windows) with a dummy profile, and check the profile is
  unchanged after each step.
**Status**: Not Started

## Decision log

- **GoReleaser OSS for build and release.** It produces the assets,
  checksums, deb/rpm, Homebrew formula, Scoop manifest and AUR package files
  from one config, and keeps the asset names.
  - Rejected: Makefile plus a script per channel (each channel would need its
    own templating code).
  - Rejected: GoReleaser Pro (none of its features are needed).
- **The `goreleaser` job publishes only the GitHub release; every channel
  publishes in its own `publish-channel.yml` run.** One path serves the
  first publish and every retry, a channel's failure or missing secret never
  touches the release or the other channels, and the `goreleaser` job writes
  its `artifacts.json` and attests on every published release.
  - Rejected: GoReleaser pushing Homebrew, Scoop and AUR inside the
    `goreleaser` job (when one push fails, GoReleaser stops before writing
    `artifacts.json`, so staging and attesting fail too, and a missing App
    key would block the release itself).
  - Rejected: a reusable-workflow call from `release.yml` (npm would see
    `release.yml` for releases and `publish-channel.yml` for retries, so
    each package would need two trusted publishers).
- **Homebrew, Scoop and AUR: our script pushes the files GoReleaser
  generated in the checked build.** The published formula, manifest and
  PKGBUILD then always come from a build whose checksums match the release.
  - Rejected: a second GoReleaser run that publishes (it would rebuild, and
    the published files would come from a build nobody compared).
- **Only the newest stable tag becomes "latest"** (see Kinds of tag). A fix
  release for an older line still gets its GitHub release, its Docker
  `<tag>` and `<version>` tags, and the channels that keep every version
  (WinGet, Chocolatey, npm); Homebrew, Scoop and AUR keep the newer
  version.
- **Homebrew formula, not a cask; no macOS signing.** Homebrew quarantines
  cask downloads, and macOS blocks an unsigned quarantined binary. Formula
  installs are not quarantined.
  - GoReleaser marks `brews` as deprecated but still supports it in v2.18.3.
    The exact version pin keeps it; a GoReleaser upgrade that drops `brews`
    has to bring a template that fills in the formula instead.
  - Rejected: Apple Developer ID signing with a cask (yearly cost and a
    D-U-N-S number, and the formula avoids the problem).
  - Rejected: homebrew-core. It requires at least 75 GitHub stars (225 when
    the authors submit), which the repo does not have.
- **No Windows code signing.** WinGet, Chocolatey and Scoop all accept
  unsigned binaries.
  - Rejected: Azure Artifact Signing (a monthly cost and a company identity
    check). It reduces SmartScreen warnings and antivirus false alarms, so it
    is the remedy if a false alarm blocks a WinGet or Chocolatey review.
- **WinGet: our templates for the first version, Komac after that.** Komac
  only updates a package that is already in `winget-pkgs`, and carries the
  previous version's metadata forward. Its binary is pinned by a hash kept
  in the repository, because it runs with `WINGET_TOKEN`.
  - Rejected: GoReleaser's `winget` pipe (it would write a second set of
    manifests next to the ones in `winget-pkgs`, and a failed PR does not
    fail its run).
- **Chocolatey: embed the raw amd64 exe, pack on a Windows runner.**
  - Embedding avoids broken download URLs, and Chocolatey allows it for
    software we distribute. The raw exe has its own `checksums.txt` line,
    which a moderator can check directly.
  - One exe covers both architectures, because arm64 Windows runs it under
    emulation.
  - Rejected: GoReleaser's `chocolateys` pipe (it needs `choco` on the
    runner that runs GoReleaser).
- **Scoop through our own bucket.** Main and Extras require far more GitHub
  stars than the repo has.
- **AUR `-bin` package only, from a tar.gz archive.** It uses the tested
  release binary, and the archive carries the notices and completions, which
  GoReleaser's `aurs` cannot add as separate sources.
  - Rejected: a source package `ahasend` (it would build with Arch's Go and
    needs the `go.mod` toolchain pinned).
- **deb/rpm as release assets, no hosted repository.** Hosting a signed
  apt/yum repo costs money or key management, and Linux demand is low (the
  v0.2.0 Linux binary has a dozen downloads).
- **No Snap.** Strict confinement cannot read `~/.ahasend` or files outside
  the home directory without manual `snap connect` steps.
- **No Nix package.** nixpkgs prefers packages submitted by users.
- **npm with per-platform packages in `optionalDependencies` and a Node
  launcher.**
  - Rejected: a postinstall download (npm 12 and pnpm block install scripts
    by default).
  - Rejected: GoReleaser's `npms` (Pro only, and it uses a postinstall
    download).
  - Rejected: one package with every binary (each user would download all
    six).
  - Rejected: an unscoped `ahasend` wrapper (a second package to maintain;
    `@ahasend/cli` sits next to `@ahasend/sdk`).
- **The npm launcher ignores SIGINT and forwards SIGTERM and SIGHUP.** A
  terminal Ctrl+C reaches the binary without the launcher, and a SIGTERM
  sent to the launcher alone still stops the binary. The two limits this
  leaves (see Channel details, npm) are accepted.
  - Rejected: forwarding SIGINT (a terminal Ctrl+C would reach the binary
    twice).
  - Rejected: not forwarding SIGTERM (a SIGTERM to the launcher alone would
    leave the binary running).
  - Rejected: starting the binary in its own process group (it would lose
    the terminal, so prompts and Ctrl+C would break).
- **Install scripts as release assets behind ahasend.com redirects** (see
  Hosting the scripts).
  - Rejected: a copy on the website (it needs a sync step).
  - Rejected: a redirect to the `main` branch (any merge would go live
    untested).
- **The install scripts never use sudo, never edit shell profiles, and never
  replace a file a package manager owns.** The user stays in control of
  system directories, their shell setup and their package managers.
- **SHA-256 in the scripts, attestations for users who want more** (see
  Attestations).
