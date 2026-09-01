# Contributing

Keep `contracts/self-improvement-portfolio-v1.json` fixed at twelve cells.
Changes to evidence, release assets, or current status belong in the assessment
and release-lock inputs; they must not change the denominator or silently turn
an `UNKNOWN` into `CLOSED`.

All verification is performed by GitHub Actions with Go 1.27. Do not run the
repository's build, test, formatting, vet, or conformance commands on the
developer machine when preparing a change. Runtime output must remain in a
caller-owned temporary directory and the checkout must stay unchanged.
