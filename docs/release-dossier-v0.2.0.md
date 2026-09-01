# Release dossier v0.2.0

This release adds the v2 transaction manifest and the immutable v0.32.0 target binding. The v0.1.0 source-tree guard remains strict: a v0.32 binding applied to the v0.31 fixture is an explicit `SOURCE_TREE_DIGEST_MISMATCH` refutation.

The immutable v0.32.0 upstream target is `5486b346aa193875383cdafa0763f3bd371c2fc1`, release `380137101`, annotated tag object `607cf838605a6f91d881559fedaf848385d0d10c`, source archive digest `sha256:3bfc8585963c88858a22308b48a78d8a212d63677b68ca11946df62b23d2b611`, source-tree digest `sha256:3db0cc6bf08e6944fa8d2e3fe8e2e546a9de0a3a536f91e147d1e488d57079fe`, and release asset `538702344` digest `sha256:79a291b446510cccf5ea32686b13e1dcad926e3b6166867f1e761a36f2df36b1`.

The manifest declares target before digests, expected denominator/state counts, structural anchors, exact planned paths, and after invariants. GitHub Actions must observe the corpus as v0.31 `CLOSED`, v0.32 `CLOSED`, wrong source tree `REFUTED`, and missing binding `UNKNOWN`, with replay mismatches `0`, caller-owned temporary output only, and repository writes `0`.

The matched v0.1→v0.2 pair supports the narrow metric `supported_immutable_target_bindings=1→2`. Whole-language improvement, external utility, and any unmatched improvement claim remain `UNKNOWN`. Process facts remain `development_process=REFUTED` and `bootstrap_scope_violation=BOOTSTRAP_SCOPE_VIOLATION`.

The final release ID, annotated tag SHA, workflow run/job, artifact IDs, asset IDs, and SHA-256 values are appended only from the GitHub Actions REST audit after publication. Existing tags and releases must never be rewritten.
