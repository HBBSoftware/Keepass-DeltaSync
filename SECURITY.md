# Security Policy

DeltaSync is a password-sync system: security is the product. Reports of
vulnerabilities are taken seriously and welcomed.

## Reporting a vulnerability

**Please do not open a public issue for security problems.** Public issues are
world-readable and would disclose the flaw before a fix exists.

Instead, use one of these private channels:

1. **GitLab — confidential issue or security advisory** (preferred):
   open a [confidential issue](https://gitlab.com/Star95/keepass-deltasync/-/issues/new)
   (tick *"This issue is confidential"*), or a private security advisory under
   the project's *Secure → Security advisories* if available.
2. **Email** the maintainer if you cannot use GitLab. The contact address is
   published in [`/.well-known/security.txt`](https://deltasync.bjoerck-braun.dk/.well-known/security.txt).

Please include:

- a description of the issue and its impact,
- steps to reproduce (a proof of concept if you have one),
- the affected component and version (server / desktop client / desktop GUI /
  Android / Firefox extension),
- any suggested remediation.

## What to expect

This is a small, volunteer-run project, so responses are best-effort rather
than bound by an SLA. You can expect:

- an acknowledgement of your report,
- an honest assessment of whether it is in scope and how severe it is,
- coordination on a fix and a disclosure timeline,
- credit in the changelog / advisory if you would like it (or anonymity if you
  prefer).

Please give a reasonable window to fix the issue before any public disclosure.

## Scope

In scope — anything that breaks the system's core promises:

- **Confidentiality of vault contents.** The server must never be able to read
  entry titles, usernames, passwords, notes, attachments, or master keys.
  Anything that lets the server (or a network attacker) recover plaintext is a
  high-severity bug.
- **Authentication & authorization.** Token forgery, privilege escalation
  (device ↔ admin), accessing another user's database, or bypassing sharing
  boundaries.
- **Cryptographic weaknesses** in the entry encryption (Argon2id → HKDF →
  XChaCha20-Poly1305) or the sharing key-wrap (X25519 sealed-box).
- **Server-side flaws**: SQL injection, auth bypass, RCE, SSRF, etc.
- **Supply chain**: the published Docker image or release binaries not matching
  the source.

Generally out of scope: missing rate limiting beyond what already exists,
issues that require a fully compromised client device, social engineering, and
denial of service from an authenticated user.

## The trust model

Before reporting, it helps to understand what the server is *designed* to see
and not see. The full threat model is in
[`docs/threat-model.md`](docs/threat-model.md). In short: the server stores only
client-encrypted blobs plus metadata (mtimes, deletions, user/device records,
an audit log); it never sees plaintext or master keys.

## Supported versions

Only the latest release of each component receives security fixes. There are no
long-term-support branches.
