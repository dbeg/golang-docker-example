# golang-docker-example

A stdlib-only Go hello-world API, packaged as a multi-stage Docker image and
published (scanned and signed) to ghcr.io by GitHub Actions.

- `GET /hello` returns `{"message":"hello, world"}`
- `GET /healthz` returns 200
- Listens on `:8080`; set `PORT` to override.

## Local

```sh
mise run run           # go run .
mise run test
mise run lint
mise run docker-build
mise run docker-run
```

## CI

`.github/workflows/ci.yml` runs tests, builds the image, scans it with Trivy
(HIGH/CRITICAL findings fail the build), then pushes and signs it with cosign
(keyless). Pull requests build, test and scan only.

The image is published for `linux/amd64` and `linux/arm64`. The Trivy scan runs
against the amd64 build only (buildx cannot load a multi-arch image for
scanning); the arm64 build uses the same base image and the same source.

## Releases

Pushing a `v*` git tag publishes the image and creates a GitHub release
(tags containing `-` are marked pre-release) with these assets:

- `golang-docker-example_<version>_<os>_<arch>.tar.gz` for linux and darwin on
  amd64 and arm64
- `checksums.txt` (sha256), check with `sha256sum -c checksums.txt`
- `sbom_<version>_linux_<arch>.spdx.json`, the image's SBOM per platform

## Image tags

| Event | Tags |
|-------|------|
| push to `main` | `latest`, `sha-<short>` |
| git tag `v1.2.3` | `1.2.3`, `1.2`, `1` |
| pre-release tag | exact tag only |

## Verify the signature

```sh
cosign verify ghcr.io/dbeg/golang-docker-example:latest \
  --certificate-identity-regexp 'https://github.com/dbeg/golang-docker-example/' \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com
```
