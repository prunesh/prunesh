# Security Policy

## Supported versions

| Version | Supported |
|---|---|
| Latest release on [Releases](https://github.com/prunesh/prunesh/releases) | Yes |
| Older releases | Best effort only |

Security fixes land on `main` and ship in the next patch or minor release.

## Reporting a vulnerability

**Do not** open a public GitHub issue for security reports.

Report privately through GitHub Security Advisories:

https://github.com/prunesh/prunesh/security/advisories/new

Include:

- Affected version(s) or commit
- Description of the issue and impact
- Steps to reproduce or a proof of concept when possible
- Any suggested fix

We will acknowledge the report as soon as practical, assess impact, and
coordinate a fix and disclosure timeline with you.

## Scope

In scope for this repository:

- The `prunesh` binary (proxy, hooks, plugin install/registry)
- Official agent integrations under `integrations/`
- `install.sh` and release artifacts published from this repo

Out of scope (report upstream or to the marketplace publisher when relevant):

- Third-party marketplace plugins and their binaries
- Vulnerabilities only in dependencies with no realistic impact on prunesh
- Social engineering, physical attacks, or DoS against GitHub/infrastructure
  we do not control

## Safe harbor

We welcome good-faith research. As long as you:

- Avoid privacy violations, destruction of data, and interruption of services
- Do not exploit a vulnerability beyond what is needed to demonstrate it
- Report findings promptly through the private channel above

we will not pursue legal action related to that research.

## Prefer private disclosure

If a fix requires a coordinated release, please keep details private until a
patched version is available or we agree on a public disclosure date.
