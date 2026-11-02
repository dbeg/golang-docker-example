# golang-docker-example Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** A stdlib-only Go hello-world API, packaged by a multi-stage Dockerfile, built/scanned/signed/published to ghcr.io by GitHub Actions, with local workflows driven by mise.

**Architecture:** One Go `main` package with two handlers. One workflow (`ci.yml`) with a `test` job and a `build-scan-push` job; the image is built locally, scanned by Trivy (blocking on HIGH/CRITICAL), and only then rebuilt from cache with push, followed by cosign keyless signing by digest.

**Tech Stack:** Go (stdlib `net/http`), Docker buildx, `gcr.io/distroless/static:nonroot`, mise, GitHub Actions, Trivy, cosign, `docker/metadata-action`.

**Spec:** Design agreed in conversation (no file). Summarized under Global Constraints.

## Global Constraints

- Go stdlib only; no third-party Go dependencies.
- Endpoints: `GET /hello` returns JSON; `GET /healthz` returns 200. Listen on `:8080`; `PORT` env var overrides.
- Dockerfile: multi-stage; builder `golang:<ver>` with `CGO_ENABLED=0`, `-trimpath`, `-ldflags="-s -w"`; final `gcr.io/distroless/static:nonroot`.
- Go version is pinned in `mise.toml` and mirrored in `go.mod` and the Dockerfile builder tag. Use the latest stable Go; verify with `mise ls-remote go`.
- Image: `ghcr.io/<github-user>/golang-docker-example`; repo is public, personal account, cloned/pushed over SSH.
- Tags: `main` -> `latest`, `sha-<short>`; `v1.2.3` git tag -> `1.2.3`, `1.2`, `1` (no `v` prefix); pre-releases get only their exact tag; PRs push nothing.
- Trivy: severity `HIGH,CRITICAL`, `exit-code 1`; no override mechanism (no `.trivyignore`, no dispatch bypass).
- Push uses `GITHUB_TOKEN`; permissions `contents: read`, `packages: write`, `id-token: write`.
- Push includes `provenance: true` and `sbom: true`; cosign signs by digest, keyless.
- Action versions pinned by major tag.
- Tools run via mise; no global installs. NEVER `git push` without explicit user instruction.
- Non-goals: multi-arch, deployment, scan overrides, large test suite.

## Review Focus

- Non-GET method on `/hello` or `/healthz`: expect 405 (pinned in Task 1 test using Go 1.22+ method-pattern routing).
- Unknown path: expect 404 (Task 1 test).
- `PORT` unset vs. set: default `:8080`, override honored (Task 1 test on `addr()` helper).
- PR build: must not push or sign (Task 4 verification: inspect `if:` conditions).
- Pre-release tag `v1.3.0-rc1`: must not move `1`/`1.3` (Task 4 metadata config; verify with `docker/metadata-action` semver patterns).

---

### Task 1: Go app with handlers and tests

**Files:**
- Create: `go.mod`, `main.go`, `main_test.go`

**Interfaces:**
- Produces: `func newMux() *http.ServeMux`, `func addr() string` (returns `":" + $PORT`, default `":8080"`). Binary entrypoint `main()` serves `newMux()` on `addr()`.

- [x] **Step 1: Init module.** `go mod init github.com/<github-user>/golang-docker-example` (ask user for the username if unknown; `gh api user -q .login` works if authenticated). Set the `go` directive to the pinned version.
- [x] **Step 2: Write failing tests in `main_test.go`** using `httptest`:
  - `TestHello`: `GET /hello` -> 200, `Content-Type: application/json`, body decodes to `{"message":"hello, world"}`.
  - `TestHealthz`: `GET /healthz` -> 200.
  - `TestMethodNotAllowed`: `POST /hello` and `POST /healthz` -> 405.
  - `TestNotFound`: `GET /nope` -> 404.
  - `TestAddr`: with `PORT` unset -> `":8080"`; with `t.Setenv("PORT","9000")` -> `":9000"`.
- [x] **Step 3: Run `go test ./...`.** Expected: FAIL (undefined `newMux`, `addr`).
- [x] **Step 4: Implement in `main.go`.** Use `mux.HandleFunc("GET /hello", ...)` and `"GET /healthz"` (method patterns give the 405/404 behavior). Encode JSON with `encoding/json`. `main()` uses `http.ListenAndServe(addr(), newMux())` and logs a fatal error on failure.
- [x] **Step 5: Run `go vet ./... && go test ./...`.** Expected: PASS.
- [x] **Step 6: Commit** (`git init` first if the directory is not yet a repo): `feat: add hello and healthz http handlers`.

### Task 2: mise config and local tasks

**Files:**
- Create: `mise.toml`

**Interfaces:**
- Consumes: Task 1 module; Task 3 Dockerfile (used by the docker tasks).
- Produces: tasks `run`, `test`, `lint`, `docker-build`, `docker-run`.

- [x] **Step 1: Write `mise.toml`.** `[tools] go = "<pinned>"`. Tasks: `run` = `go run .`; `test` = `go test ./...`; `lint` = `go vet ./...`; `docker-build` = `docker build -t golang-docker-example:dev .`; `docker-run` = `docker run --rm -p 8080:8080 golang-docker-example:dev`. (`lint` is included per the design's "only if wanted" note; CI's `go vet` step can call `mise run lint`, or drop the task if the user declines.)
- [x] **Step 2: Verify.** `mise install && mise run test`. Expected: PASS. `mise tasks` lists all five.
- [x] **Step 3: Commit:** `chore: add mise config and tasks`.

### Task 3: Dockerfile

**Files:**
- Create: `Dockerfile`, `.dockerignore`

**Interfaces:**
- Produces: image exposing 8080, running as nonroot, entrypoint `/app`.

- [x] **Step 1: Write `Dockerfile`.** Stage `build` from `golang:<pinned>`; `WORKDIR /src`; copy `go.mod` then `go mod download` (cache layer; harmless with no deps); copy source; `CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /app .`. Final stage `gcr.io/distroless/static:nonroot`; `COPY --from=build /app /app`; `EXPOSE 8080`; `ENTRYPOINT ["/app"]`.
- [x] **Step 2: Write `.dockerignore`:** `.git`, `docs`, `.github`, `mise.toml`, `*.md`.
- [x] **Step 3: Verify.** `mise run docker-build`, then `mise run docker-run` in the background and check `curl -s localhost:8080/hello` returns the JSON and `curl -si localhost:8080/healthz` returns 200. Stop the container. Check `docker image ls golang-docker-example:dev` size is small (roughly 5-15 MB).
- [x] **Step 4: Commit:** `feat: add multi-stage dockerfile`.

### Task 4: GitHub Actions workflow

**Files:**
- Create: `.github/workflows/ci.yml`

**Interfaces:**
- Consumes: Task 3 Dockerfile; Task 1 tests.

- [x] **Step 1: Triggers and permissions.** `on:` `pull_request`; `push` with `branches: [main]` and `tags: ['v*']`. Workflow-level `permissions: contents: read`; the `build-scan-push` job adds `packages: write` and `id-token: write`.
- [x] **Step 2: `test` job.** `actions/checkout@v4`, `actions/setup-go@v5` with `go-version-file: go.mod`, run `go vet ./...` and `go test ./...`.
- [x] **Step 3: `build-scan-push` job** (`needs: test`), in order:
  1. checkout; `docker/setup-buildx-action@v3`.
  2. `docker/metadata-action@v5` with `images: ghcr.io/${{ github.repository }}` and tags: `type=raw,value=latest,enable={{is_default_branch}}`; `type=sha`; `type=semver,pattern={{version}}`; `type=semver,pattern={{major}}.{{minor}}`; `type=semver,pattern={{major}}`. (Metadata-action omits the major/minor patterns for pre-releases. Confirm in step 6.)
  3. `docker/build-push-action@v6` with `load: true`, `push: false`, a single local tag `scan:local`, and `cache-to`/`cache-from: type=gha`.
  4. `aquasecurity/trivy-action@<current major tag>` with `image-ref: scan:local`, `severity: HIGH,CRITICAL`, `exit-code: '1'`, `ignore-unfixed: false`.
  5. `if: github.event_name != 'pull_request'`: `docker/login-action@v3` to `ghcr.io` with `${{ github.actor }}` / `GITHUB_TOKEN`.
  6. Same `if:`: `docker/build-push-action@v6` with `id: push`, `push: true`, `tags: ${{ steps.meta.outputs.tags }}`, `labels: ${{ steps.meta.outputs.labels }}`, `provenance: true`, `sbom: true`, `cache-from: type=gha`.
  7. Same `if:`: `sigstore/cosign-installer@v3`, then `cosign sign --yes ghcr.io/${{ github.repository }}@${{ steps.push.outputs.digest }}`. (Repository must be lowercase; owner names are lowercase here, otherwise lowercase it in a step.)
- [x] **Step 4: Lint the workflow** if `actionlint` is available via `mise exec actionlint@latest -- actionlint`; expected: no errors. Otherwise re-read the YAML for indentation and `if:` correctness.
- [x] **Step 5: Commit:** `ci: add test, scan, publish and sign workflow`.

### Task 5: README, repo creation, first run

**Files:**
- Create: `README.md`

- [x] **Step 1: Write a short `README.md`:** what it is; `mise run run/test/docker-build/docker-run`; the tag scheme; how to verify the signature: `cosign verify ghcr.io/<user>/golang-docker-example:latest --certificate-identity-regexp 'https://github.com/<user>/golang-docker-example/' --certificate-oidc-issuer https://token.actions.githubusercontent.com`.
- [x] **Step 2: Commit:** `docs: add readme`.
- [x] **Step 3: STOP and ask the user** before creating or pushing the remote. On approval: `gh repo create golang-docker-example --public --source=. --remote=origin` (confirm the remote URL is the SSH form `git@github.com:...`), then push `main` only when the user says so.
- [x] **Step 4 (after the user-approved push): verify.** The Actions run on `main` goes green; the package appears under the user's ghcr packages; set it to public in the package settings (manual, one-time); `docker pull` it anonymously; run the cosign verify command from the README. Expected: verification succeeds.
- [x] **Step 5 (optional, user-approved): test the rolling tags.** Push tag `v0.1.0` and confirm that `0.1.0`, `0.1` and (per metadata-action's behavior for 0.x) the major tag appear. Then optionally tag `v1.0.0` and `v1.0.1`, and confirm `1` and `1.0` move.

### Task 6: Multi-arch image, release assets, automated release

**Files:**
- Modify: `Dockerfile`, `.github/workflows/ci.yml`, `README.md`

**Constraints:**
- Platforms: `linux/amd64`, `linux/arm64`.
- The scan step stays single-platform (amd64): buildx cannot `load` a multi-arch image. Document this in the README.
- Release assets for `v*` tags:
  - Binaries `golang-docker-example_<version>_<os>_<arch>.tar.gz` for `linux` and `darwin` on `amd64` and `arm64` (four archives, each containing the binary; `<version>` has no `v` prefix).
  - `checksums.txt` (sha256 of every archive).
  - One SBOM per image platform: `sbom_<version>_linux_<arch>.spdx.json`.
- Tags containing `-` create a GitHub pre-release; others create a full release.
- The existing `v1.0.0` release is left untouched.

**Interfaces:**
- Consumes: the `push` step's `digest` output (Task 4).
- Produces: a `digest` output on the `build-scan-push` job, consumed by the new `release` job.

- [x] **Step 1: Dockerfile cross-compile.** Make the builder stage `FROM --platform=$BUILDPLATFORM golang:1.26.10 AS build`; add `ARG TARGETOS TARGETARCH`; build with `GOOS=$TARGETOS GOARCH=$TARGETARCH CGO_ENABLED=0`. Verify: `docker buildx build --platform linux/amd64,linux/arm64 .` succeeds and `mise run docker-build && mise run docker-run` still serves `/hello` on the arm64 host.
- [x] **Step 2: Workflow push step.** Add `platforms: linux/amd64,linux/arm64` to the "Push image" step only. Expose `digest: ${{ steps.push.outputs.digest }}` as a `build-scan-push` job output.
- [x] **Step 3: Add a `release` job.** `needs: build-scan-push`, `if: startsWith(github.ref, 'refs/tags/v')`, `permissions: contents: write, packages: read`. Steps: checkout; `setup-go` (`go-version-file: go.mod`); cross-compile the four binaries and package them as `.tar.gz`; write `checksums.txt`; log in to ghcr.io and extract each platform's attested SBOM from `${IMAGE}@<digest>` with `docker buildx imagetools inspect --format '{{json .SBOM}}'` (fallback if that does not yield clean per-platform SPDX: `trivy image --format spdx-json`); run `gh release create "$GITHUB_REF_NAME" --generate-notes` with all files, a notes header containing the `docker pull ghcr.io/<repo>:<version>` and `cosign verify` commands, and `--prerelease` when the tag contains `-`.
- [x] **Step 4: Lint.** `mise exec actionlint@latest -- actionlint`. Expected: no errors.
- [x] **Step 5: README.** Document the platforms, the release assets, how to verify checksums, and the single-arch scan caveat.
- [x] **Step 6: Commit locally:** `feat: publish multi-arch images and release assets`. Do not push until the user approves.
- [ ] **Step 7: Verify (after the user-approved push).** Tag `v1.0.1-rc.1`; expect a pre-release with 4 archives, `checksums.txt` and 2 SBOMs, `docker buildx imagetools inspect` showing both platforms, and the image running natively on the arm64 Mac. Then tag `v1.0.1`; expect a full release with the notes header, `1.0.1`, `1.0`, `1` and `latest` moved, and `cosign verify` succeeding.

---

## Execution notes

Completed 2026-11-02. Deviations from the plan as written:

- Go 1.26.10 was installed manually from a tarball: Zscaler blocked mise's download. Pinned in `mise.toml`, `go.mod` and the Dockerfile.
- Action versions are current majors (`checkout@v7`, `setup-go@v7`, `build-push-action@v7`, etc.), not the v4/v5 in Task 4. `sigstore/cosign-installer` and `aquasecurity/trivy-action` are pinned to exact tags (no floating major).
- Added a `scan` mise task (trivy, `--db-repository ghcr.io/aquasecurity/trivy-db` to get past Zscaler) and `trivy` in `mise.toml`.
- Rolling-tag test used `v0.0.1` and `v0.0.2-rc.1`; `metadata-action` does publish the bare `0` tag for 0.x. The RC published only its exact tag.
