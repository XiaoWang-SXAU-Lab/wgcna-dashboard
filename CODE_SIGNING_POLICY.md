# Code signing policy

The project has applied for SignPath Foundation signing. The policy below
describes the process planned for signed Windows releases after approval.
Those releases will carry the attribution “Free code signing provided by
SignPath.io, certificate by SignPath Foundation”.

## Scope

When a public Windows release is produced, the following artifacts are
Authenticode-signed through SignPath:

- `rsWGCNA.exe`
- `rs-wgcna-worker.exe`
- `regutrace.exe`
- `rsWGCNA-setup-<version>.exe`

The signatures identify the publisher and protect the files after packaging.
They do not by themselves guarantee that every antivirus engine will accept a
new release; false-positive reports are handled with the vendors separately.
The bundled R runtime is upstream third-party software and is not signed by
this project.

macOS builds are currently **ad-hoc signed and not notarized**. They are not
covered by this policy and should be treated as unsigned preview builds.

## Build provenance

Signing requests are submitted only from the GitHub Actions release workflow
(`.github/workflows/release.yml`) for a tag build. The workflow:

1. Builds the frontend and Go executable from this repository.
2. Builds the Rust worker and ReguTrace from their GitHub repositories.
3. Runs the ReguTrace release test suite.
4. Submits the three application executables to SignPath and waits for manual approval.
5. Packages the signed executables with Inno Setup, then submits the installer
   separately and waits for manual approval.
6. Verifies all four signatures before publishing the installer and SHA-256 checksums.

No maintainer signs a local build with the SignPath certificate.

## Roles and approval

- **Authors/maintainers**: organization members with repository write access.
- **Reviewers**: maintainers who review release changes.
- **Approvers**: SignPath users authorized to approve signing requests.

All repository maintainers and SignPath users must use multi-factor
authentication. Each SignPath signing request is manually approved according to
the project's signing policy. The release policy is restricted to tag builds.

## Verification

On Windows:

```powershell
Get-AuthenticodeSignature .\rsWGCNA.exe
Get-AuthenticodeSignature .\rs-wgcna-worker.exe
Get-AuthenticodeSignature .\regutrace.exe
Get-AuthenticodeSignature .\rsWGCNA-setup-<version>.exe
```

All four must report `Valid`. Release assets include SHA-256 checksums. The CI
workflow fails if any of the four signatures is not valid after SignPath
returns the artifacts.

## Privacy and uninstall

The core WGCNA analysis runs locally. Optional online features send requests
to external services. The AI navigator sends prompts and the context included
in the request to the configured AI provider. Literature search sends search
terms to the selected provider (PubMed, Europe PMC, Semantic Scholar, or
SerpApi). Gene Ontology resource updates request ontology data from the Gene
Ontology Consortium. The respective provider's privacy policy applies to
requests sent to that service.

Windows builds are installed and removed with the bundled uninstaller.
