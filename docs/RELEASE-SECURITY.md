# Windows release security: sidecars, signing and AV false positives

## Rule: never release an EXE to a temp directory at runtime

`rsWGCNA.exe` is a Go/WebView2 application. The Rust worker and ReguTrace are
separate native processes. They are shipped as **sidecar executables** next to
the application:

```text
rsWGCNA.exe
regutrace.exe
rs-wgcna-worker.exe
R\...
```

The Go backend resolves them in this order:

1. `WGCNA_REGUTRACE` / `WGCNA_WORKER` environment override (development)
2. sibling executable in the application directory
3. `PATH`

It must **not** `go:embed` a native EXE, write it to `%TEMP%`, and execute it.
That is a dropper pattern and triggers endpoint protection. The sidecar
binaries are installed by the signed installer. Ship all three executables
together; never publish a main EXE without matching sidecars.

`WGCNA_REGUTRACE`/`WGCNA_WORKER` remain useful for development and tests.

## Why AV false positives happen

- A brand-new unsigned PE has no reputation.
- Debug builds are large, contain debug sections and static SQLite, and are more
  likely to hit heuristics.
- Embedding an EXE in another EXE and extracting it at runtime looks like a
  dropper.
- AV definitions change over time; a file that was clean yesterday can be
  flagged today.
- Bundled third-party runtimes (R) are unsigned and cannot be re-signed unless
  their license permits it.

## Release requirements

### 1. Sidecars only

Build scripts must copy `rs-wgcna-worker.exe` and `regutrace.exe` next to
`rsWGCNA.exe`. The installer must include all three. Do not copy them into
`backend/internal/*-bin` for embedding.

### 2. Code signing (required for public releases)

Sign with a trusted Authenticode certificate:

- `rsWGCNA.exe`
- `regutrace.exe`
- `rs-wgcna-worker.exe`
- the Inno Setup installer (`rsWGCNA-setup-*.exe`)

Options:

- **SignPath Foundation**: free for eligible open-source projects (this repo is
  MIT/public); the certificate is issued to SignPath Foundation, so Windows
  shows SignPath Foundation as the publisher. Recommended for this project.
- **OV code signing**: paid; exportable PFX is usually possible; publisher is
  the lab/organization; reputation builds over time.
- **EV code signing**: paid; hardware token/HSM; stronger identity, but EV does
  not guarantee immediate SmartScreen reputation.
- **Azure Trusted Signing**: paid cloud signing; verify organization eligibility
  (commonly limited to the US/Canada/EU/UK).
- Self-signed certificates do **not** solve SmartScreen/AV warnings.

Do not re-sign third-party R binaries unless their license allows it.

### 3. Windows version resources

The local Windows build and CI stamp the three application executables before
signing. They use `rsWGCNA` as ProductName and the `AppVersion` from
`packaging/setup.iss` as ProductVersion and FileVersion. Inno Setup supplies
the installer's product metadata. CI checks it against the application before
requesting the installer signature.

When changing `AppVersion`, update the installer filename references and the
product-version restrictions in both SignPath artifact configurations. Apply
the reviewed XML to both configurations in SignPath before signing that release.

### 4. Vendor false-positive submission

Before publishing a release, submit the signed installer and EXEs:

- Huorong (火绒) false-positive report
- Microsoft WDSI file submission (select "incorrect detection")
- 360 false-positive report

Include: product name, download page, SHA-256, what the binary does, and the
exact detection name. This is vendor-side allowlisting, not a user instruction.

### 5. CI release gate (SignPath Foundation)

Windows tag builds use the Rust GNU toolchain with locked dependencies and run
the ReguTrace workspace tests in release mode. The workflow submits the three
unsigned EXEs to SignPath, rebuilds the installer from the signed payload,
submits the installer separately, then verifies all four signatures.

SignPath onboarding (manual, once):

1. Apply at <https://signpath.org/> with the GitHub repository URL.
2. Create a SignPath project (`project-slug`, e.g. `rswgcna`), link the GitHub
   trusted build system, and install the SignPath GitHub App.
3. Create two artifact configurations and paste the XML from
   `.signpath/artifact-configurations/`:
   - `rsWGCNA-payload` (`rsWGCNA-payload.xml`)
   - `rsWGCNA-installer` (`rsWGCNA-installer.xml`)
4. Create a signing policy (e.g. `release-signing`) restricted to tag builds,
   and grant the CI user submitter permission.
5. Add the GitHub Actions secret and variables:

| Kind | Name | Value |
|---|---|---|
| Secret | `SIGNPATH_API_TOKEN` | SignPath API token |
| Variable | `SIGNPATH_ORGANIZATION_ID` | SignPath organization UUID |
| Variable | `SIGNPATH_PROJECT_SLUG` | e.g. `rswgcna` |
| Variable | `SIGNPATH_SIGNING_POLICY_SLUG` | e.g. `release-signing` |
| Variable | `SIGNPATH_ARTIFACT_CONFIGURATION_SLUG_PAYLOAD` | optional; default `rsWGCNA-payload` |
| Variable | `SIGNPATH_ARTIFACT_CONFIGURATION_SLUG_INSTALLER` | optional; default `rsWGCNA-installer` |

6. Test with a tag while `RSWGCNA_SIGNING_ENABLED` is still off: SignPath signs
   the artifacts, but the release job is blocked, so nothing is published.
   Set `RSWGCNA_SIGNING_ENABLED=true` only after those signatures verify. When
   it is true and SignPath credentials are missing, the Windows job fails
   instead of publishing unsigned files.

Protect `v*` tags and restrict the SignPath API token to release maintainers.
The token can submit signing requests even though it cannot export the signing
key.

Non-tag builds still produce unsigned test artifacts. macOS builds are ad-hoc
signed and **not notarized**; they are not part of the Windows signing gate.

### 6. Clean-machine test

Test the signed installer on a clean Windows VM with:

- Windows Defender
- Huorong
- 360

Also scan with VirusTotal and record the detection names in the release notes.
Do not publish if a major engine flags the installer or a sidecar.

## Local development exception

Adding a local AV exclusion for the build directories is acceptable **only on
development machines**:

```text
%USERPROFILE%\Bio-info-PRJ\regutrace\target
%USERPROFILE%\Bio-info-PRJ\wgcna-dashboard
%USERPROFILE%\Bio-info-PRJ\rs_WGCNA\target
%LOCALAPPDATA%\rsWGCNA-build
%USERPROFILE%\Bio-info-PRJ\_rsWGCNA-local
```

This must not appear in user-facing documentation as the fix for a release.

## Release checklist

1. Build release sidecars; verify no `go:embed` native binaries remain.
2. Verify the stamped Windows VERSIONINFO and add/verify the icon.
3. Sign all three EXEs and the installer with a timestamp.
4. Build the installer; verify it includes all sidecars.
5. Test install/uninstall/launch on a clean VM with Defender/Huorong/360.
6. VirusTotal scan; submit false positives to vendors; wait for confirmation.
7. Publish with SHA-256 and signing information.

## Current status

- Sidecar resolution is implemented in `internal/worker` and
  `internal/regutrace`; the build scripts no longer copy binaries into
  `*-bin` embed locations.
- The release workflow builds sidecars into the app bundle/installer directory
  and blocks tag releases until `RSWGCNA_SIGNING_ENABLED=true`. The Windows
  release upload includes the three payload executables alongside the installer,
  so every entry in `SHA256SUMS.txt` is a published asset.
- Windows SignPath wiring is implemented. SignPath approval, API credentials,
  production signing and end-to-end installer verification are **not yet
  configured**. Until then, Windows release artifacts are unsigned and the
  release gate must stay off.
- macOS remains ad-hoc signed and not notarized. This is intentional for now;
  Developer ID and notarization require a paid Apple Developer Program account.
- Windows VERSIONINFO is stamped for the main executable and both Rust
  sidecars. Icon resources for those three EXEs are not yet added.
- The Inno Setup uninstaller is not signed by the current external signing
  flow. If publisher identity is required for uninstall, configure Inno Setup's
  `SignTool`/`SignedUninstaller` or add a separate SignPath request.
- Verified on Windows (2026-11-11): the built app starts from its app directory
  with both sidecars, a missing worker is fatal, a missing ReguTrace binary is
  non-fatal, and the full ReguTrace release test suite passes 61/61. Huorong
  still deleted the unsigned **debug** test binary during `cargo test`; this is
  the remaining endpoint-reputation problem, not a code-path failure.
