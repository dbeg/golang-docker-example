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

- [ ] **Step 1: Init module.** `go mod init github.com/<github-user>/golang-docker-example` (ask user for the username if unknown; `gh api user -q .login` works if authenticated). Set the `go` directive to the pinned version.
- [ ] **Step 2: Write failing tests in `main_test.go`** using `httptest`:
  - `TestHello`: `GET /hello` -> 200, `Content-Type: application/json`, body decodes to `{"message":"hello, world"}`.
  - `TestHealthz`: `GET /healthz` -> 200.
  - `TestMethodNotAllowed`: `POST /hello` and `POST /healthz` -> 405.
  - `TestNotFound`: `GET /nope` -> 404.
  - `TestAddr`: with `PORT` unset -> `":8080"`; with `t.Setenv("PORT","9000")` -> `":9000"`.
- [ ] **Step 3: Run `go test ./...`.** Expected: FAIL (undefined `newMux`, `addr`).
- [ ] **Step 4: Implement in `main.go`.** Use `mux.HandleFunc("GET /hello", ...)` and `"GET /healthz"` (method patterns give the 405/404 behavior). Encode JSON with `encoding/json`. `main()` uses `http.ListenAndServe(addr(), newMux())` and logs a fatal error on failure.
- [ ] **Step 5: Run `go vet ./... && go test ./...`.** Expected: PASS.
- [ ] **Step 6: Commit** (`git init` first if the directory is not yet a repo): `feat: add hello and healthz http handlers`.

### Task 2: mise config and local tasks

**Files:**
- Create: `mise.toml`

**Interfaces:**
- Consumes: Task 1 module; Task 3 Dockerfile (used by the docker tasks).
- Produces: tasks `run`, `test`, `lint`, `docker-build`, `docker-run`.

- [ ] **Step 1: Write `mise.toml`.** `[tools] go = "<pinned>"`. Tasks: `run` = `go run .`; `test` = `go test ./...`; `lint` = `go vet ./...`; `docker-build` = `docker build -t golang-docker-example:dev .`; `docker-run` = `docker run --rm -p 8080:8080 golang-docker-example:dev`. (`lint` is included per the design's "only if wanted" note; CI's `go vet` step can call `mise run lint`, or drop the task if the user declines.)
- [ ] **Step 2: Verify.** `mise install && mise run test`. Expected: PASS. `mise tasks` lists all five.
- [ ] **Step 3: Commit:** `chore: add mise config and tasks`.

### Task 3: Dockerfile

**Files:**
- Create: `Dockerfile`, `.dockerignore`

**Interfaces:**
- Produces: image exposing 8080, running as nonroot, entrypoint `/app`.

- [ ] **Step 1: Write `Dockerfile`.** Stage `build` from `golang:<pinned>`; `WORKDIR /src`; copy `go.mod` then `go mod download` (cache layer; harmless with no deps); copy source; `CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /app .`. Final stage `gcr.io/distroless/static:nonroot`; `COPY --from=build /app /app`; `EXPOSE 8080`; `ENTRYPOINT ["/app"]`.
- [ ] **Step 2: Write `.dockerignore`:** `.git`, `docs`, `.github`, `mise.toml`, `*.md`.
- [ ] **Step 3: Verify.** `mise run docker-build`, then `mise run docker-run` in the background and check `curl -s localhost:8080/hello` returns the JSON and `curl -si localhost:8080/healthz` returns 200. Stop the container. Check `docker image ls golang-docker-example:dev` size is small (roughly 5-15 MB).
- [ ] **Step 4: Commit:** `feat: add multi-stage dockerfile`.

### Task 4: GitHub Actions workflow

**Files:**
- Create: `.github/workflows/ci.yml`

**Interfaces:**
- Consumes: Task 3 Dockerfile; Task 1 tests.

- [ ] **Step 1: Triggers and permissions.** `on:` `pull_request`; `push` with `branches: [main]` and `tags: ['v*']`. Workflow-level `permissions: contents: read`; the `build-scan-push` job adds `packages: write` and `id-token: write`.
- [ ] **Step 2: `test` job.** `actions/checkout@v4`, `actions/setup-go@v5` with `go-version-file: go.mod`, run `go vet ./...` and `go test ./...`.
- [ ] **Step 3: `build-scan-push` job** (`needs: test`), in order:
  1. checkout; `docker/setup-buildx-action@v3`.
  2. `docker/metadata-action@v5` with `images: ghcr.io/${{ github.repository }}` and tags: `type=raw,value=latest,enable={{is_default_branch}}`; `type=sha`; `type=semver,pattern={{version}}`; `type=semver,pattern={{major}}.{{minor}}`; `type=semver,pattern={{major}}`. (Metadata-action omits the major/minor patterns for pre-releases. Confirm in step 6.)
  3. `docker/build-push-action@v6` with `load: true`, `push: false`, a single local tag `scan:local`, and `cache-to`/`cache-from: type=gha`.
  4. `aquasecurity/trivy-action@<current major tag>` with `image-ref: scan:local`, `severity: HIGH,CRITICAL`, `exit-code: '1'`, `ignore-unfixed: false`.
  5. `if: github.event_name != 'pull_request'`: `docker/login-action@v3` to `ghcr.io` with `${{ github.actor }}` / `GITHUB_TOKEN`.
  6. Same `if:`: `docker/build-push-action@v6` with `id: push`, `push: true`, `tags: ${{ steps.meta.outputs.tags }}`, `labels: ${{ steps.meta.outputs.labels }}`, `provenance: true`, `sbom: true`, `cache-from: type=gha`.
  7. Same `if:`: `sigstore/cosign-installer@v3`, then `cosign sign --yes ghcr.io/${{ github.repository }}@${{ steps.push.outputs.digest }}`. (Repository must be lowercase; owner names are lowercase here, otherwise lowercase it in a step.)
- [ ] **Step 4: Lint the workflow** if `actionlint` is available via `mise exec actionlint@latest -- actionlint`; expected: no errors. Otherwise re-read the YAML for indentation and `if:` correctness.
- [ ] **Step 5: Commit:** `ci: add test, scan, publish and sign workflow`.

### Task 5: README, repo creation, first run

**Files:**
- Create: `README.md`

- [ ] **Step 1: Write a short `README.md`:** what it is; `mise run run/test/docker-build/docker-run`; the tag scheme; how to verify the signature: `cosign verify ghcr.io/<user>/golang-docker-example:latest --certificate-identity-regexp 'https://github.com/<user>/golang-docker-example/' --certificate-oidc-issuer https://token.actions.githubusercontent.com`.
- [ ] **Step 2: Commit:** `docs: add readme`.
- [ ] **Step 3: STOP and ask the user** before creating or pushing the remote. On approval: `gh repo create golang-docker-example --public --source=. --remote=origin` (confirm the remote URL is the SSH form `git@github.com:...`), then push `main` only when the user says so.
- [ ] **Step 4 (after the user-approved push): verify.** The Actions run on `main` goes green; the package appears under the user's ghcr packages; set it to public in the package settings (manual, one-time); `docker pull` it anonymously; run the cosign verify command from the README. Expected: verification succeeds.
- [ ] **Step 5 (optional, user-approved): test the rolling tags.** Push tag `v0.1.0` and confirm that `0.1.0`, `0.1` and (per metadata-action's behavior for 0.x) the major tag appear. Then optionally tag `v1.0.0` and `v1.0.1`, and confirm `1` and `1.0` move.
