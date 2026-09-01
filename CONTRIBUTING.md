# Contributing

The `.gooo` file is the authority for the operation graph, required transaction fields, append-only invariants, status precedence, migration, rollback states, process boundary, and metric names. Go is the AST executor; changes that bypass the metacode contract are not accepted.

Keep changes in one open pull request at a time. Use GitHub Actions with Go 1.27 for verification. Do not edit or test by mutating the v0.31.0 ledger repository. The planner's operation must always receive an input path and produce a separate caller-owned temporary copy.

Release tags must be annotated and immutable. Never delete or rewrite a tag or release, and record exact REST release/tag/asset/run/job/artifact IDs and digests in release evidence after CI observes them. Do not turn local timing into improvement evidence without matched manual/tool before-after inputs under the same source digest.
