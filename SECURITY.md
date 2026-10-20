# Security policy

## Reporting a vulnerability

Report a vulnerability privately through [GitHub private vulnerability reporting](https://github.com/panyam/agni/security/advisories/new)
on this repository. If that is unavailable, email sri.panyam@gmail.com with "agni security" in the
subject. Please do not open a public issue for a suspected vulnerability.

We acknowledge a report within 2 business days. A critical issue (CVSS 7.0 or higher) is fixed or
mitigated within 7 days. The fix ships as a patch release on the current release line, and the
advisory is published once that release is available.

## Supported versions

The latest tagged minor release line receives security fixes.

## Scope

The root module (`github.com/panyam/agni`, the `agni` binary and the `ghcr.io/panyam/agni` image) and
the `datasheet` module (`agnids` and the `ghcr.io/panyam/agnids` image). The example modules under
`examples/` are not shipped and are out of scope.

`agni serve` is built to serve designs to the people who can already read them. Exposing it to the
public internet is not yet supported (agni issue 855). A report that depends on an untrusted
client reaching a server is still welcome, and is triaged with that in mind.

## What runs

`make audit` runs every scanner: staticcheck (`make lint`), gosec (`make seccheck`), gitleaks over
the whole history (`make secrets`) and govulncheck over each shipped module (`make vulncheck`). The
first three run in the gate on every pull request. govulncheck runs weekly and on each release tag,
because the advisory database moves independently of the code. OpenSSF Scorecard runs weekly and on
each push to main.
