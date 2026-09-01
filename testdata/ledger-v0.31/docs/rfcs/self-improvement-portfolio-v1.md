# Self-improvement portfolio v1

## Scope

The portfolio is an evidence ledger for thirty-seven specifically named capabilities.
It is not a language-wide quality score, maturity score, or completeness claim.
The only aggregate values are exact state counts and fixed bucket counts.

## Fixed denominator

The source of truth is `contracts/self-improvement-portfolio-v1.json`. Its
`cells` array is immutable during a run and contains thirty-seven entries. The v0.31
migration is append-only `ADD1/RETIRE0/SPLIT0` from the prior thirty-six-cell
profile. Every entry has a stable axis, proof bucket, indicator bucket, activity name, source path,
IR path, generated artifact path, evaluator path, and metric with denominator
one. The authoritative source activity set is
`examples/self-improvement-portfolio/main.gooo`.

The proof buckets are `FOUNDATION/COHERENCE/REGRESSION` at `4/28/5`.
The indicator buckets are `DRIVER/OUTCOME/GUARDRAIL` at `4/28/5`.
Changing a release, evaluator, or evidence artifact never changes those counts.

## Disposition

Each assessment starts with an explicit state. Release-backed `CLOSED` cells
are eligible only when the CI verifier confirms the release URL, tag, exact
target commit, immutable flag, asset URL, asset size, and asset SHA-256 digest.
`REFUTED` wins over `UNKNOWN`, and `UNKNOWN` wins over `CLOSED`.

An `UNKNOWN` record contains exactly these six fields:

`stage`, `step`, `reason`, `unknown_class`, `next_operation`, and `blocked_by`.

The current frontier closes reflexive-loop v0.2 integration, semantic merge
advice, and the OpenTofu plan path only after their immutable release evidence
passes the evaluator. It also closes release promotion only when the immutable
reflexive-loop v0.3 release carries internal lifecycle-final `PROMOTED`,
promotion, rollback-boundary, and immutable-target receipts. This does not
infer core semantic authority or independent external utility evidence.

The v0.11 frontier appends `IMPROVEMENT_FRONTIER_RELEASE`. It closes only
when the immutable `gooo-improvement-frontier@v0.1.0` release binds its
annotated tag target, source Actions run/job, downloaded receipt and source
archive bytes, and the receipt's fixed protocol result. Six failed upstream
implementation attempts remain append-only counterexample references and are
not included in the closure-gated release map.

The v0.12 frontier appends `AUTHORITY_BOOTSTRAP_RELEASE`. It closes only when
the immutable `gooo-authority-bootstrap@v0.1.0` release binds its annotated tag
target, successful required and post-main Actions runs, post-main artifact, all
six downloaded release assets, and the exact bootstrap receipt. The upstream
receipt's own `UNKNOWN` and `immutable=false` fields are preserved as source
data; the ledger cell records only the immutable release-adoption boundary.

The v0.13 frontier appends `OPENTOFU_ENVELOPE_RELEASE`. It closes only when
the immutable `gooo-opentofu-envelope@v0.1.1` release binds its annotated tag
target, successful PR and post-main Actions runs, post-main artifact, both
release assets, and the source envelope's exact 12-cell observation. The
mutable v0.1.0 release and its two failed CI runs remain append-only
`FAILED_RELEASE_IMMUTABILITY` and `FAILED_CI_VALIDATION` counterexamples.

The v0.14 frontier appends `IMPROVEMENT_PROPOSER_RELEASE`. It closes only when
the immutable `gooo-improvement-proposer@v0.1.1` release binds its annotated
tag object and target, successful release run `33397566380`, post-main
conformance run `33397372252`, post-main artifact `9759855868`, all four
release assets, the upstream 12-cell observation, and the six fixed output
artifacts per conformance case. The upstream v0.1.0 tag object/target and
failed run `33396465907`/job `99502048200` are retained as a separate
`FAILED_RELEASE_TRIGGER` with no release and cannot close the cell. Earlier
failed runs are not success evidence.

The v0.15 frontier appends `TEST_FRONTIER_RELEASE`. It closes only when the
immutable `gooo-test-frontier@v0.1.1` release binds tag object
`398577621c42eb7450416bdf086b9304c8c1e42a` to target
`f8e1f8aebb67abbda237073893a4a855a8659df5`, successful release/audit run
`33398545885`/job `99508911340`, post-main run `33398482775`/job
`99508698139`, artifact `9760281954`, and all three release assets. The
upstream protocol has 12 activities/cells, `4/4/4` proof and indicator totals,
10 cases (`3 CLOSED / 3 UNKNOWN / 4 REFUTED`), exact test totals
`40/9/16/10/5`, invalidated frontier 20, and zero product-authority writes,
local test executions, and cross-project gates. Its v0.1.0 release ID
`379770450` remains an append-only `REFUTED`
`SELF_ASSERTED_IMMUTABILITY_CONTRADICTED_BY_PLATFORM` counterexample because
the GitHub API reports `immutable=false`; the direct-main `7281ead` observation
is workflow-only with no pull request and is not confused with product state.

The v0.16 frontier appends `CHANGE_BUNDLE_RELEASE`. It closes only when the
immutable `gooo-change-bundle@v0.1.1` release binds tag object
`09885ac7480d1ee2e350e907f5dc408b35188f47` to target
`a93c41a28b5718f110b8679556b169f2b11c75b5`, successful release run
`33398653367`/job `99509268842`, audit job `99509422788`, and audit artifact
`9760351466` with size `735` and digest
`sha256:30b54d122e4e32f47fecc74f93345e7f9a04a15c1c757f4f83cad36e2ba5f762`.
The source protocol has 12 cells/activities, `4/4/4` proof and indicator
totals, `3 CLOSED / 3 UNKNOWN / 6 REFUTED` cases, exact replay/rollback
comparisons, and zero authority. Release v0.1.0 remains a platform-immutability
`REFUTED` counterexample, while the three post-PR #1 direct-main commits are
preserved as `DEVELOPMENT_PROCESS_DIRECT_MAIN` REFUTED process observations.

The v0.17 frontier appends `UTILITY_TRIAL_PROTOCOL_RELEASE`. It closes only
when the immutable `kimjooyoon/gooo-utility-trial@v0.1.1` release binds release
ID `379863199`, annotated tag object `5a42a68fb1f9a54eaa33097fb6eeca4db421bf05`
to target `5500f00ec67b75fadf450110acefca713c5b5733`, merged upstream PR #3,
successful pre-merge run `33409087319`/job `99543871814`, post-main run
`33409165999`/job `99544131261`, release/audit run `33409188187`/job
`99544202999`, audit artifact `9764422074` (`2764537` bytes,
`sha256:a690b16b4ec7f6271eee23bffa52f1209ab238cfd21ff15f75f7f61a5e93adee`),
and both release assets. Its source protocol remains 12 cells/activities with
`4/4/4` proof and indicator buckets, `protocol_ready=CLOSED`,
`utility=UNKNOWN`, zero external evidence and eligible pairs,
`process=REFUTED`, `score=NOT_COMBINED`, and denominator migration `NONE`.
The failed initial v0.1.0 release `379848683` (`immutable=false`), failed run
`33407273856`, audit artifact `9763659711`, and assets `538154567/538154571`
are preserved as an append-only `RELEASE_HISTORY_REWRITE_PROCESS=REFUTED`
counterexample. Current historical release `379850805` and assets
`538157619/538157605` remain replacement evidence only and never closure evidence.

The v0.18 frontier appends `REFLEXIVE_MODERN_CYCLE_RELEASE`. It closes only
when immutable upstream `gooo-reflexive-loop@v0.3.1` release `379879740` binds
annotated tag object `e54e08feacb3ea4da67b5aa5e404a4ce0b605895` to target
`ed8ff02c7d8f56d8d9474b68036ea80cdc105261`, successful post-main conformance
run `33410813438`/job `99549616696`, Actions artifact `9765064827`, and all
four release assets. Its source observation is fixed at 12 activities/cells,
`4/4/4` proof and indicator buckets, `3 normal / 3 UNKNOWN / 4 REFUTED`
scenarios, and precedence `REFUTED > UNKNOWN > CLOSED`. The normal candidate
receipt records oracle failures `1 -> 0`, tests `4/2/1/1/0`, replay `19/0`,
rollback `1/0`, build `7436 ms / 270260 KiB`, conformance
`8844 ms / 14524 KiB`, and `93` Gooo lines across `44` files and `16`
directories, with all eight authority fields zero. Upstream v0.3.0 release
`379458203` and its target remain only as a separate historical fact; legacy
`REFLEXIVE_LOOP` v0.2 is unchanged and is not retired or replaced.

The v0.19 frontier appends `EXPERIENCE_MEMORY_RELEASE`. It adopts immutable
`gooo-experience-memory@v0.1.0` release `379896833` with its annotated tag,
main CI evidence, release recheck, source/evidence assets, release manifest,
and checksum bindings. The upstream observation has fixed denominator `12`,
`4/4/4` proof and indicator totals, `4 CLOSED / 4 UNKNOWN / 4 REFUTED` cases,
and precedence `REFUTED > UNKNOWN > CLOSED`. Its authoritative main-CI
metrics include recurrence `1 -> 0`, avoided `1`, new unknown `2`, replay
`2/0`, attempts `2`, memory `1`, candidates `5`, peak RSS `7256 KiB`, `1578`
Go lines across `9` files, `16` Gooo lines in `1` file, and `13` descendant
directories. Because the Go implementation remains larger than the Gooo source,
`CORE_SEMANTIC_AUTHORITY` stays `REFUTED`; utility remains `UNKNOWN`, and prior
refutations and upstream release-process observations remain append-only.

The v0.20 frontier appends `SEMANTIC_DRIFT_GUARD_RELEASE`. It adopts
immutable `gooo-semantic-drift-guard@v0.1.1` release `379915376`, binding
annotated tag object `1e1cf4882347ccd69c14c4aa96e63c096709d512` to target
`15b6c1dcce26feb5f64d562140708f7cb27390aa`, successful PR #2 conformance
run `33416441475`/job `99568101328`, artifact `9767194212`, and successful
release run `33416657453`/job `99568816492`. The independent protocol has
denominator `12`, ten cases (`1 CLOSED / 4 UNKNOWN / 5 REFUTED`), precedence
`REFUTED > UNKNOWN > CLOSED`, and canonical `source -> IR -> generated Go`
binding. Its normal metrics are releases `2`, source files `2`, IR nodes `24`,
generated files `2`, relations `12 -> 12`, equivalent `1`, drift `0`, unknown
bindings `0`, replay `1/0`, RSS `12246 KiB`, wall `1 ms`, build `4654 ms`, test
`1993 ms`, tests `12/12/0/0/0`, Go `1772` lines/`13` files, Gooo `85`
lines/`6` files, `24` directories, and `45` files; all three authority counts
are zero. The v0.1.0 release `379905110` remains an append-only non-score
process observation with `immutable=false` and the literal
`tag_object=v0.1.0^{tag}` defect, together with its faulty assets and
annotated correction assets. This independent release does not close or alter
`SEMANTIC_DRIFT_DEVELOPMENT_PROCESS=REFUTED`; Go `1772` versus Gooo `85`
keeps `CORE_SEMANTIC_AUTHORITY=REFUTED`. Utility remains `UNKNOWN`, yielding
`CLOSED23/UNKNOWN1/REFUTED2`.

The v0.21 frontier appends `SEMANTIC_AUTHORITY_CENSUS_RELEASE`. It adopts
immutable `gooo-semantic-authority-census@v0.1.0` release `379947813`, binding
annotated tag object `c81ff9b843dce716c57fe2ab542bde52e922ab2b` to target
`0451a1f5813e51a2d09145d7516170c7802f9fd5`, successful PR #1 merge, main run
`33421788389`/job `99585671364` with artifact `9769198151`, and release run
`33421919840`/job `99586108117` with artifact `9769259042`. Its independent
protocol has denominator `12`, proof and indicator buckets `4/4/4`, cases
`CLOSED3/UNKNOWN3/REFUTED3`, and score `NOT_COMBINED`; the exact pair is
`obligations=3`, `generated_bound=2 -> 3`, `handwritten_go=1 -> 0`. The main-run
timings are compile `5184 ms`, build `751 ms`, test `2209 ms`, conformance
`8306 ms`, and peak RSS `270708 KiB`; tests are `4/4/0/0/0`, replay is `9/0`,
and all authority counts are zero. The census is observation-only: it does not
close core semantic authority or the semantic-drift development-process cell,
and external utility remains `UNKNOWN`, yielding `CLOSED24/UNKNOWN1/REFUTED2`.

The v0.22 frontier appends `REFLEXIVE_LEARNING_DRIFT_CYCLE_RELEASE`. It adopts
the immutable upstream `kimjooyoon/gooo-reflexive-loop@v0.4.0` release
`379940049`, binding annotated tag object
`89f6d283791f917c2fe789fa05016a0f33df21d2` to target
`134d9043e8808147ed2f7252527e809d3eafad44`, successful main Actions run
`33420406673`/job `99581097777`, and artifact `9768699219` of `3624947` bytes
with digest
`sha256:11f3fffb4c6ee93307e46b5c1fdb8013fe5829983e069d823d896dc77e84a6c2`.
Its learning-drift-gated protocol has fixed denominator `12`, cases
`CLOSED3/UNKNOWN4/REFUTED5`, precedence `REFUTED > UNKNOWN > CLOSED`, and a
normal `CLOSED` decision with external utility `UNKNOWN`. The normal evidence
records cycles `2`, candidates `5`, known-refuted recurrence `1 -> 0`, attempts
`2`, avoided/refuted/unknown candidates `1/1/2`, replay `16/0`, rollback `1/0`,
tests `4/2/1/1/0`, build `520 ms / 91448 KiB`, test `0 ms / 7116 KiB`, and
conformance `4291 ms / 14300 KiB`; repository writes, local test executions,
and cross-project required gates are zero. The release manifest and all four
release assets are pinned by API identity, size, and SHA-256 digest. The
adoption is append-only and does not close core semantic authority or semantic-
drift development process; utility remains `UNKNOWN`, yielding
`CLOSED25/UNKNOWN1/REFUTED2`.

The v0.23 frontier appends `UNKNOWN_RESOLUTION_LATTICE_RELEASE`. It adopts
immutable upstream `kimjooyoon/gooo-resolution-lattice@v0.2.0`, release
`379967493`, binding annotated tag object
`2f452efe6b05b50760500da1a4bea7d323e9c11d` to target
`fac2f5c0688c62fd31912a310e0fae77bc198258`. Source conformance, post-main
conformance, and release Actions are locked to runs/jobs
`33424634161/99595118419`, `33425091977/99596614819`, and
`33425271313/99597213464`; both conformance artifacts are pinned by ID, size,
and digest. The adopted protocol is the immutable five-stage
`PROJECT -> ARTIFACT -> ACTIVITY -> PREDICATE -> FIELD` ladder with a fixed
12-cell denominator, `4/4/4` proof and indicator buckets, cases
`CLOSED1/UNKNOWN4/REFUTED5`, four UNKNOWN classes, six verified receipts, and
16 identity comparisons with zero mismatches. Fixed-point evidence is the only
accepted closure; the top unknown decision is `FAIL_CLOSED`, contradictions have
`REFUTED` precedence, and utility inference is false. The exact normal pairs
are `4 -> 2` and `5 -> 3`. The upstream release closes only the new ledger cell;
the portfolio becomes `CLOSED26/UNKNOWN1/REFUTED2`.

The v0.24 frontier appends `SELF_REPAIR_INTEGRATION_RELEASE`. It adopts
immutable `kimjooyoon/gooo-self-repair-example@v0.2.1`, release `379971030`,
binding annotated tag object
`b8318c1645bc76286eb5c404b771118b6ce1e07b` to target
`28f3589d69796b4630b2e066c6a5c45ac8468096`. PRs #3 and #4 are merged; the
historical direct-main workflow commit
`5dca56d238751739beba3fafe9a9018c0bb18ce4` is preserved as
`DEVELOPMENT_PROCESS_DIRECT_MAIN` `REFUTED`, while the current guard is
`CLOSED`. Post-main Actions run/job `33425759488/99598796427` and release
run/job `33425908089/99599283424` are successful; artifact `9770678796` is
locked at `14701` bytes and
`sha256:870a731cf484535e2b1218e1d7eee37a0ccdd9c7ad194ff19030ab31e42c7514`.
The 12-activity protocol records claims `3/3/3`, proof and indicator buckets
`4/4/4`, cycles `attempts=2/candidates=5/recurrence=1->0/avoided=1/unknown=2/replay=2/0`,
tests `3/3/0/0/0`, build/test/conformance `250/240/13757 ms`, peak RSS
`90856 KiB`, inventory `Go 8/1547`, `Gooo 2/16`, `15` directories, `25`
files, outputs `12/38440`, and zero repository writes, local test executions,
and cross-project required gates. Core semantic authority is `CLOSED`,
external utility is `UNKNOWN` because the exact pair axes cross, and the new
cell is `CLOSED`; the portfolio becomes `CLOSED27/UNKNOWN1/REFUTED2`.

The v0.25 frontier appends `OPENTOFU_DURABLE_SEMANTIC_ENVELOPE_RELEASE`. It
adopts immutable `kimjooyoon/gooo-opentofu-envelope@v0.1.9` release
`380009987`, binding annotated tag object
`8f913ac3bcef39a5105280a6a05114b7abc3ac87` to target
`b482afd68a864400a209cb4f439e727cfdfe2eda`. Upstream PR #10, main CI
`33432375475`/job `99620555197`, and release CI
`33432449551`/job `99620801430` are successful; main artifact `9773097414`
is `99611` bytes with digest
`sha256:f04619dbd77314bdf84ba2d5c1b9edd4b9a09b533a8a26c2185ec3b786804157`.
All four immutable release assets are pinned by API identity, size, and
download digest: evidence `538450808` (`13344`), manifest `538450812`
(`55115`), checksums `538450816` (`263`), and source `538450823` (`30885`).
The upstream envelope is a fixed 12-cell protocol with 5 path steps, 14
binding edges, `4/4/4` proof and indicator buckets, cases `3/3/3`, replay
`2/0`, tests `9/9/0/0/0`, build/test/conformance `214/45/324 ms`, peak RSS
`76084 KiB`, 15 files/2401 lines/7 directories, 3 outputs/8118 bytes, and
zero repository, remote, direct-main, tag, and local-test mutations. The
semantic graph is `CLOSED` only within `GOOO_SEMANTIC_GRAPH_ONLY`; upstream
utility is `UNKNOWN` and upstream global core is `NOT_MADE`, so the ledger's
existing core and development-process `REFUTED` states remain unchanged.
The manifest preserves v0.1.3 as immutable with zero assets, v0.1.4-v0.1.7
as failed no-release triggers, and v0.1.8 as a draft with zero assets; none
are deleted or hidden. The new cell is `CLOSED`, yielding
`CLOSED28/UNKNOWN1/REFUTED2`.

The v0.26 frontier appends `LANGUAGE_DELTA_FORGE_DURABLE_RELEASE`. It adopts
immutable `kimjooyoon/gooo-language-delta-forge@v0.1.2` release `380033725`,
binding annotated tag object `5d68c5f2f699f9d73bcf2e87121204512dfd64fc` to
target `30ad7a736d5d354a9e0cd998a8a1bd4dd5e11b45`. The main verification run
`33436391757`/job `99633759904` and immutable release run
`33436456556`/job `99633967202` are successful; their locked artifacts are
`9774550869` (`27490` bytes, digest
`sha256:a2f9a55ebb3870f2093e0f3b11439a523c899fac968efb0c449b6c5c6dc486cd`)
and `9774576485` (`58715` bytes, digest
`sha256:bcf1519c02234b44b69490378723c82fa9e9f83d64b65c2c24138d9ce341013b`).
The four immutable release assets are pinned by API identity, size, and
SHA-256 digest. The fixed upstream denominator is `18`, with cases
`CLOSED3/UNKNOWN3/REFUTED3`, program proof and indicator totals `6/6/6`,
case proof and indicator totals `3/3/3`, candidate bundles `10`, generated JSON
outputs `11`, representative delta `2/1/1`, rollback `2/1/1`, and zero
repository writes or protected-core adoption. The upstream utility is
`NOT_CLAIMED` and upstream global core is `NOT_MADE`; the new ledger cell is
`CLOSED`, while the ledger's global core and development-process states remain
`REFUTED`, yielding `CLOSED29/UNKNOWN1/REFUTED2`.

The v0.27 frontier appends `OPENTOFU_GENERATED_SERVICE_PROJECT_DURABLE_RELEASE`.
It adopts immutable `kimjooyoon/gooo-opentofu-envelope@v0.2.1` release
`380037012`, binding annotated tag object
`06ab6ccc2f75cf0602715811f51a7a3097d23277` to target
`bdc5c2cdacb5865f59efd2eb496d58eeb0bd2787`. Main CI
`33436975864`/job `99635653831` and release CI `33437056751`/job
`99635914331` are successful. The pinned main artifact is `9774763580`,
`16278` bytes, with digest
`sha256:8e2bd75365b7e0e92ee8276cadbfc0d03842145a7bf5fd52efd1a33e2973de06`.
The release assets are pinned by API identity, size, and SHA-256. The upstream
generated service-project observation has fixed denominator `12`, `6` user-path
steps, `14` edges, `4/4/4` proof and indicator buckets, cases
`NORMAL2/UNKNOWN2/REFUTED2`, `5` generated outputs/`9019` bytes, project
resources/capabilities/endpoints `3/1/2`, relations `2/2/2`, runtime
`106/56/56/46/30 ms`, peak RSS `75304 KiB`, tests `6/6/0/2/2`, and one
successful OpenTofu validation with no repository, local, direct-main, or tag
mutations. The scoped semantic graph is `CLOSED`, utility and improvement are
`UNKNOWN`, global core is `NOT_MADE`, and the new ledger cell is `CLOSED`.
Historical v0.2.0 remains `REFUTED` with release API `404`, tag object
`9dfdee84d61f3acbe899b5ad57fd8f35f8159210`, target
`c9f5de0b33fee1ca8546a627a8a94242b99c0733`, failed run/job
`33435908822/99632154067`, and the malformed historical target preserved as
the failure reason. During CI diagnosis, the ledger consumer performed one
local artifact-schema assertion replay; this was not a local Go
test/build/vet/conformance execution, and the ledger development process
remains `REFUTED`. The portfolio is `CLOSED30/UNKNOWN1/REFUTED2`.

The v0.28 frontier appends `REFLEXIVE_COMPILER_PHASE_DURABLE_RELEASE`. It adopts
immutable `kimjooyoon/gooo-reflexive-compiler-slice@v0.1.1` release `380040917`,
binding annotated tag object `8db85557f66d4bb61a4fc1816b3a20dab2c40f0c` to target
`dabbe38badebefdf2979d8862c26a647b0dd15c0`. Upstream main CI
`33437644781`/job `99637878450`, release CI `33437664492`/job `99637944818`, and
main artifact `9775010906` (`19269921` bytes,
`sha256:99934e633fc823b236077fb02f2dee2e0447c40686243cd6e647ca9e30be874c`) are
locked. The six release assets are pinned by API identity, size, and SHA-256.
The scope is `ONE_COMPILER_PHASE_ONLY`, denominator schema
`gooo/reflexive-compiler-denominator/v1`, and phase `reflexive.normalize.v1`, with
exact operations `NormalizeSource→SemanticIR`, `EmitBackend→GeneratedBackend`, and
`VerifyReplay→Evidence`. Cases are `CLOSED1/UNKNOWN1/REFUTED1` under
`REFUTED>UNKNOWN>CLOSED`; UNKNOWN is `DIRECT_MISSING` with all six fields and the
duplicate stable ID is REFUTED. Decision matches are `3`, digest/IR/generated
mismatches are `0/0/0`, rollback is `3/3/3`, outputs are `21` files/`32273` bytes,
runtime is compile/build/test/conformance `58/5388/2093/2614 ms` with peak RSS
`7221248` bytes, tests are `3/3/0/0/1`, and inventory is Go `8/1096`, Gooo
`4/33`, `24` regular files, and `14` directories. Local tests are zero; repository
writes and external mutations are not observed; proof and indicator are not observed;
global self-hosting, external utility, and whole-language improvement remain
`UNKNOWN`. Upstream v0.1.0 remains untouched and `NON_DURABLE` (`immutable=false`).
The new ledger cell is `CLOSED`, while ledger global core and development process
remain `REFUTED`, yielding `CLOSED31/UNKNOWN1/REFUTED2`. The prior local diagnostic
replay count remains one, with no additional local schema/conformance replay.

The v0.29 frontier appends `CAUSAL_VERIFICATION_RUNNER_DURABLE_RELEASE`. It
adopts immutable `kimjooyoon/gooo-causal-verification-runner@v0.1.1` release
`380048457`, binding annotated tag object
`82bb99006232a064725df29a53af5405e222cd42` to target
`0c16428762d1d1da1b28fe05c4e051d2cc41967b`. Upstream PR #2 head
`65295c74603e1e8ac418f20ef66b12f2ae935979` and merge/main
`0c16428762d1d1da1b28fe05c4e051d2cc41967b` are retained, with PR run
`33438798441`, main run `33438900833`, and release audit
`33439000856`/job `99642343892` successful. The main artifact is `9775474098`,
named `gooo-causal-verification-runner-33438900833`, `3466163` bytes, digest
`sha256:b3b9b89c820e9aa2f2d48c6686fb4a51bd52ac0b58c2c9ef15bc531191966183`.
The three immutable assets are pinned by API identity, size, and SHA-256. The
upstream denominator is `gooo/causal-verification-runner/denominator/v1` with
12 causal activities and `4/4/4` proof and indicator buckets. Cases are
`CLOSED2/UNKNOWN1/REFUTED3` under `REFUTED>UNKNOWN>CLOSED`; UNKNOWN is
`CAUSAL_EDGE_UNKNOWN` with all six fields. Safe reuse is `2/1/1/1/2/0/0`,
transitive impact `3/2/2/1/3/0/0`, unknown edge `2/2/2/0/2/0/1`, stale proof
`2/2/2/0/2/0/0`, hidden counterexample `2/1/1/1/2/1/0`, and cache-hit-only
`2/2/2/0/2/0/0`; avoided executions are `1/1/0/0/1/0`. Outputs are `52`
files/`6140304` bytes, runtime compile/build/test/conformance is
`7220/6880/2020/12020 ms` with peak RSS `268496 KiB`, tests are `13/10/3/1/1`,
and inventory is Go `8/2466`, Gooo `1/59`, `52` files, and `15` directories
with the root README excluded. The current process guard is `CLOSED` with
bootstrap direct-main `1`, historical post-bootstrap direct-main `2`, and
post-guard direct-main `0`; overall upstream development process remains
`REFUTED`. Historical v0.1.0 remains untouched and immutable `true`, including
the two preserved post-bootstrap direct-main counterexamples. Utility is
`UNKNOWN` and global core is `NOT_MADE`; the new ledger cell is `CLOSED`,
yielding `CLOSED32/UNKNOWN1/REFUTED2`. The prior local diagnostic replay count
remains one, with no additional local schema/conformance replay.

The v0.30 frontier appends `EXECUTABLE_EVOLUTION_TRIAL_COUNTEREXAMPLE_DURABLE_RELEASE`.
It adopts immutable `kimjooyoon/gooo-evolution-trial@v0.1.0` release `380086557`,
binding annotated tag object `c3a87bd320a24e6c4961afc532fd4df6b5d165c3` to target
`d917eec6344f2eaa9a6fb0069f7fe0aaafeb6982`. Its successful main run is
`33445305000`/job `99662940416`, with artifact `9777794326`, `130085` bytes,
digest `sha256:07c81d12ecf003907678cbbea15d104effd44bdb658893ee426820fdb5b9a13a`.
The successful release audit is `33445379243`/job `99663170908`, with artifact
`9777817257`, `33589` bytes, digest
`sha256:1cf113aaf5a6e629ce2da1f358a5502dba3600ec9f8155b785d72acfd5f20001`.
All six release assets are pinned by API identity, size, URL, and SHA-256 in
the release lock, and CI also rechecks the three immutable upstream tool inputs.
The released experiment is explicitly `REFUTED`: candidate evolution is
`CLOSED` with `ADD2/RETIRE1/SPLIT1` and exact rollback, but the released
compiler rejects the four-activity candidate with
`phase graph must declare exactly three executable activities`; semantic IR and
backend are `NOT_OBSERVED`, and the causal result is `REFUTED` with
`2/1/1/1/2/1/0` total/selected/executed/reused/oracle/fail/unknown/avoided
metrics. This is preserved counterexample evidence, not an experiment success.
The ledger adoption cell is `CLOSED`, yielding `CLOSED33/UNKNOWN1/REFUTED2`.
The prior `local_validation_executions=1`, artifact-schema replay count `1`,
zero local Go executions, and process state `REFUTED` remain unchanged.

The v0.31 frontier appends `REFLEXIVE_COMPILER_GRAPH_TOPOLOGY_SELF_IMPROVEMENT_DURABLE_RELEASE`.
It adopts immutable `kimjooyoon/gooo-reflexive-compiler-slice@v0.2.0` release
`380102097`, binding annotated tag object `5852cc52f4ecec7fc835fdb6ed7adc1108459d6a`
to target `7bdba0c353a73a40111747dbf55512939f6841a0`. The source PR validation,
post-merge main validation, and release audit are pinned in the release lock,
including all six release assets and their SHA-256 digests. The compiler now
accepts both the prior three-role/two-edge/stage-one topology and the
four-role/three-typed-edge/stage-two split based on role, edge, and stage
semantics rather than raw activity count. The direct corpus remains
`CLOSED1/UNKNOWN1/REFUTED1` before and after, while the trial candidate cases
are accepted `0 -> 3`; the historical trial refutation remains `REFUTED`.
The ledger adoption cell is `CLOSED`, yielding `CLOSED34/UNKNOWN1/REFUTED2`.
The prior `local_validation_executions=1`, process state `REFUTED`, and zero
additional local schema/conformance replays remain preserved.

The non-completeness capability evidence registry is a separate seventeen-entry
external-input ledger. Its count and dispositions never alter this fixed
thirty-seven-cell denominator; unavailable inputs remain `UNKNOWN`, while known
release/API or digest contradictions remain `REFUTED`.

## Authority and measurements

Runtime writes are limited to caller-owned temporary output. The source
checkout is snapshotted before and after CI work and must remain unchanged.
The report records `wall_ms` and raw `duration_ns` separately so sub-millisecond
runs do not disappear through rounding. Inventory lines are physical lines;
the root README is excluded from line accounting. CI also records exact build,
test, and peak-RSS observations in `ci-observations.json`. Developer-local
verification counts remain zero because all build, test, formatting, vet, and
conformance checks are performed by GitHub Actions.
