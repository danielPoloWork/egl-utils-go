# Security Policy

## Supported versions

Only the **latest released minor line of the current major** receives security fixes. The
current major is `v2`, published as [`v2.0.0`](https://github.com/danielPoloWork/egl-utils-go/releases/tag/v2.0.0);
the supported window is defined in
[`docs/workflow/maintenance.md`](docs/workflow/maintenance.md).

| Version | Supported |
|---------|-----------|
| latest released `v2.x` | ✅ |
| older `v2.x` | ❌ |
| `v1.x` and earlier | ❌ |

`v1.1.1` remains resolvable from the module proxy — Go never withdraws a published version — but
it receives no fixes. Migrating is the remedy; see the
[`v2.0.0` release notes](docs/releases/v2.0.0.md) for the import rewrite.

## Reporting a vulnerability

**Do not open a public issue or PR for a security problem.** Report it privately via
[GitHub private vulnerability reporting](https://docs.github.com/code-security/security-advisories)
on this repository (**Security** tab → *Report a vulnerability*), to `danielPoloWork`.

Please include:

- the affected version(s) and platform/toolchain;
- a minimal reproduction (a failing test is ideal);
- the observed impact and, if known, the root cause.

> **This form also receives code-of-conduct reports.** It is the only private, authenticated
> channel the repository offers, so [`CODE_OF_CONDUCT.md`](CODE_OF_CONDUCT.md) designates it for
> those as well; such a report opens with `Code of Conduct` and is handled under that document, not
> under this one. Noted here so an incoming report that is not a vulnerability is not mistaken for a
> misfiled one.

## What to expect

1. **Acknowledgement** of the report.
2. **Triage & fix under embargo** on a private branch / draft advisory; the SemVer level of
   the fix is assessed by the decision tree in
   [`docs/workflow/maintenance.md`](docs/workflow/maintenance.md).
3. **Coordinated release**: the fix ships, then the advisory is published. The fix is
   recorded in `CHANGELOG.md` under a **Security** entry with the advisory / CVE reference.
4. **Backport** to every still-supported release line.

Thank you for reporting responsibly.
