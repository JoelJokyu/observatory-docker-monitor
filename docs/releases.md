# Preparing a public release

Repository: https://github.com/JoelJokyu/observatory-docker-monitor. [Observatory 0.1.0](https://github.com/JoelJokyu/observatory-docker-monitor/releases/tag/v0.1.0) was published November 2, 2026. Published artifacts are listed on [GitHub Releases](https://github.com/JoelJokyu/observatory-docker-monitor/releases). MIT license; copyright Observatory contributors. Private vulnerability reporting is enabled.

Native amd64/arm64 Checks passed for the initial source commit in [run 41083633644](https://github.com/JoelJokyu/observatory-docker-monitor/actions/runs/41083633644). The numbered release workflow rechecks its exact tagged source before publishing images. See docs/release-notes/v0.1.0.md for deployment limits and the Linux kernel advisory.

## What users receive

An allowlisted ZIP with compose.yaml, .env.example, INSTALL.md, LICENSE, dependency notices and release.json. Users need Docker/Compose, not Go/Node/Git/PostgreSQL tools. They copy/edit .env, pull images, start Compose and open localhost. The app image is pinned to an immutable registry digest by the release workflow. The source checkout retains its separate build-based development Compose file.

The current local ZIP references `observatory-release-test:0.1.0`, an image built only on this computer for verification. It is not a publicly installable release and is ignored by Git under dist/. The actual workflow replaces that with this repository's real GHCR digest.

## Build locally

```
docker buildx build --platform linux/amd64,linux/arm64 --build-arg VERSION=0.1.0 --build-arg REVISION=local-preparation --load -t observatory-release-test:0.1.0 .
python scripts/package_release.py --version 0.1.0 --image observatory-release-test:0.1.0
```

Loading both architectures needs a Docker image store that supports multi-platform images (Docker Desktop does here). A classic store can load/build one platform at a time; the publishing workflow pushes the multi-platform manifest directly. Node/Go build stages run on the builder's platform; Go cross-compiles the static runtime binary for each target. No emulated Node build is necessary. The app supports `--version` without needing Docker or a database connection.

`python scripts/smoke_release.py --image observatory-release-test:0.1.0 --platform linux/amd64` creates a random disposable Compose project and database, checks readiness/UI/version/migrations, performs a binary-safe backup and restore to a separate database, recreates the app and checks readiness, then removes only that test project/volume. Repeat for linux/arm64 if your Engine can execute it. ARM execution under emulation is not native Apple Silicon/Linux ARM verification.

Regenerate dependency notices with `python scripts/collect_licenses.py` before packaging after toolchain/dependency changes. The collector requires installed Go modules and frontend node_modules. The package tool uses a fixed file allowlist, copies the selected notice manifest, rejects unsafe/non-versioned image references and outputs a ZIP checksum. It never copies local .env, databases, node_modules or the work folder. It creates a bundle, not a source export; clone/download source from the repository separately.

## When the repository is ready

1. Review the source, MIT wording, dependency notices and private-reporting contact in this repository.
2. Run the Checks workflow. Its native amd64/arm64 jobs include frontend checks, all Go/database checks and disposable release smoke tests. Confirm both pass for the intended source revision.
3. Confirm supported hosts using the matrix below. Create an immutable reviewed source tag such as v0.1.0. Do not reuse release version tags for different code.
4. Manually run **Prepare numbered release**, entering 0.1.0 without v. The selected source tag is checked out and checked again before image publication. The job derives `ghcr.io/<owner>/<repo>`, publishes both architectures, adds version/revision/source/license metadata and BuildKit provenance/SBOM, then uploads a digest-pinned ZIP and SHA256SUMS as a workflow artifact. It does not create/publish a GitHub Release or upload arbitrary local files.
5. GHCR packages may start private. Set this package public when ready. Verify an anonymous pull and a clean user installation from the digest-pinned bundle before announcing it. Source repository visibility and package visibility are separate settings.
6. Download the workflow artifact, verify its checksum and contents, then attach the inner ZIP and SHA256SUMS to a GitHub Release for that exact tag with reviewed release notes. Keep the artifact retention window in mind (14 days). This final public announcement/release remains a deliberate maintainer action.

Publication requires normal GitHub workflow/package permissions but no hardcoded personal access token. The workflow only publishes on explicit workflow_dispatch. Pull-request checks have read-only repository permissions and do not publish images. Action major versions should be reviewed and pinned to audited commit SHAs before a stricter supply-chain policy is claimed; no such claim is made here. Build-stage base tags currently follow their minor/major line, so rebuilding a version can change output; retain published digests and do not overwrite version tags.

## Verification/support matrix

| Target | Current evidence | Claim for release |
| --- | --- | --- |
| Windows Docker Desktop, Linux containers, amd64 | Existing live project; release smoke tested locally | Locally verified after checks recorded in docs/verification.md |
| Linux arm64 container on this Windows Engine | Cross-build and emulated smoke test | Architecture check, not native OS support proof |
| Native Linux amd64/arm64 | Native GitHub CI checks passed | CI-verified image startup, collection and backup/restore |
| macOS Intel / Apple Silicon | No local host available | Community testing needed; do not claim verified |
| Windows containers / remote Docker / rootless / ECI | Outside the standard verified path | Document limitations; rootless socket override is best effort |

## First release notes draft

Observatory 0.1.0 is an early local-first Docker monitoring release: container resources/I/O/PIDs, Compose project filtering, lifecycle events, bounded Docker log snapshots, named test sessions, per-project local alerts and completed-session comparison. It runs as one Go app with an embedded React/TypeScript dashboard and PostgreSQL. Data stays in the local installation; no hosted account is needed.

Monitoring is sampled, event replay is bounded and logs are not archived. Missing data is explicit. Authentication, remote hosting, Windows-container monitoring and production capacity guarantees are outside this release. See INSTALL.md for Docker socket access, seven-day history, backup/restore, upgrades and removal.

## References checked during preparation

- [Docker multi-platform builds in GitHub Actions](https://docs.docker.com/build/ci/github-actions/multi-platform/)
- [GitHub container publishing](https://docs.github.com/en/actions/tutorials/publish-packages/publish-docker-images)
- [GHCR public/anonymous access](https://docs.github.com/en/packages/working-with-a-github-packages-registry/working-with-the-container-registry)
- [MIT license text](https://opensource.org/license/mit)


## Version 0.1.0 delivery evidence

The release workflow and [fresh anonymous-image installations](https://github.com/JoelJokyu/observatory-docker-monitor/actions/runs/41460009960) passed on native Linux amd64/arm64. Public ZIP/checksum downloads were independently verified after publication. The source tag stays on its reviewed commit; the extra verification workflow and this delivery record are later main-branch maintenance commits.

The maintainer’s Windows Docker Desktop still rejected the GHCR pull despite successful anonymous registry reads and native Linux installs; its cause is unresolved. This limitation is in the public release notes. See docs/verification.md for the exact image digest, checksum and evidence. Do not treat earlier local builds as proof of a successful public-image pull on Windows.
