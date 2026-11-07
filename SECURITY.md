# Security Policy

Nexus Chat is end-to-end encrypted software, so we take security reports
seriously and handle them privately until a fix is available.

## Supported versions

Nexus Chat has not had a release yet. Once it does, security fixes go to the
latest beta release and to `main`.

## Reporting a vulnerability

**Please do not open a public issue for security problems.**

Report privately through GitHub: open the repository's **Security** tab and
choose **Report a vulnerability**. If you cannot use GitHub, email
ali@azam.email with the subject line "Nexus Chat security".

Please include:

- what an attacker can do, and what they need first (an account, a
  malicious instance, network position, ...)
- steps to reproduce, or a proof of concept
- the version or commit you tested

## What to expect

| Step                                        | Target                  |
| ------------------------------------------- | ----------------------- |
| Acknowledgement of your report              | within 3 working days   |
| Initial assessment and severity             | within 10 working days  |
| Fix released for critical and high severity | within 90 days at most  |
| Public advisory and credit (if you want it) | when the fix is shipped |

We will keep you updated while we work on a fix, and we will not take legal
action against good-faith research that respects users' privacy and avoids
service disruption.

## Scope

In scope: the server, push relay, clients, desktop app, cryptography
design, and the default Docker deployment. Known, documented limitations are
listed in [docs/threat-model.md](docs/threat-model.md#4-known-limitations-beta).
Reports showing that one of them is worse than documented are welcome.
