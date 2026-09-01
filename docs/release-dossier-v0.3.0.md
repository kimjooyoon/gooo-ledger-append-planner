# Release dossier v0.3.0

This release adds the v3 transaction manifest and the immutable upstream v0.33.0 target binding. It separates the five semantic append-only sources from the existing report/history derived projections. The initial `NO_OVERWRITE` counterexample remains preserved in the v2 behavior and historical evidence.

The v0.33.0 target is commit `0723f7cdcc149a74a9a7a7aee03117f6b108cbd3`, release `380165562`, annotated tag object `f1150846ca5b660b7c19321f6845ff35fe8affeb`, source archive digest `sha256:fe518bb651c4842218f91fd65e3cdd7ffe98c1f6957305697f78e0b2e9cdd6ef`, source-tree digest `sha256:b0ba210450cba861c38cf9c44eb69a9c6986cc85233ce3a350aa796671666cb1`, and release asset `538796911` digest `sha256:9c9b893b341f9e7bffe26620909c9e20fa178c56530e47924834a9fda7fb9256`.

The adopted explanation-carrying compiler lock is immutable release `kimjooyoon/gooo-reflexive-compiler-slice@v0.3.0`, release `380150043`, target commit `0cf44db8b0d6cd96d190e9f902312d0be9394029`, tag object `d7e2bd301f5d1634e92b0de90d54798a35db424a`, with the six exact release assets recorded in `examples/transactions/valid-append-v3-v0.33.json`.

GitHub Actions must observe v0.31 `CLOSED`, v0.32 `CLOSED`, v0.33 projection regeneration `CLOSED`, projection before/source digest mismatches `REFUTED`, and missing projection authority `UNKNOWN`, with replay mismatches `0`, exact rollback, caller-owned temporary output only, and repository writes `0`. Only the exact v0.2→v0.3 pair supports `supported_projection_transition_cardinality=0→1`; remainder claims stay `UNKNOWN`.

The release ID, merge/tag SHA, workflow run/job, artifact IDs, release asset sizes, and SHA-256 values are appended only from the GitHub Actions REST audit after publication. Existing tags and releases must never be rewritten. The upstream v0.34.0 tag/release was not observed and is not asserted.
