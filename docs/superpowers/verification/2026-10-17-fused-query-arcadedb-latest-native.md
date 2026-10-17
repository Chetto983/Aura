# The fused query on ArcadeDB's latest native build

Date: 2026-10-17. Same bench and indexes as `2026-10-17-fused-query-maintainers-edits.md`, on the
operator's request ("Misura con l'ultima native di arcade Latest").

The image `arcadedata/arcadedb:latest-native` today is `26.11.1-SNAPSHOT` at build `15958b48f` on
ArcadeDB's main, pushed at 06:37 UTC. The JVM image `latest` carries the same commit. That commit
holds two changes this spec waited for:
- **#10181,** where `vector.fuse` and `vector.rerank` stop loading every match twice;
- **#10169,** a `where` option on `vector.neighbors`. It tests the predicate on each candidate the
  ANN search meets, instead of materialising every RID of the space up front.

**Verdict:**
- **The native build is slower than the JVM on every statement.** On the 7,798-passage library the
  shipped statement takes 68 to 74 ms on native and 44 to 51 ms on the JVM of the same commit.
  The 16 October measurement on 26.10.2 had the same gap (70.7 against 48.4 ms).
- **#10181 does not move this query.** The JVM snapshot gives the same medians as 26.10.1, within
  the noise of one run. The cost is still the dense filter subquery.
- **`where: { embed_space: :space }` keeps every answer, at the cost of the post-filter.**
  - It gives 16/16 identical answers on the large library, and identical metrics with identical
    answers on all 108 gold questions.
  - It costs 14.5 ms on the JVM snapshot, against 14.3 ms for post-filter (P) and 43.7 ms for
    the shipped statement.
  - Unlike P and R it filters inside the search, so a library holding two embedding spaces
    still gets its full `k` neighbours from its own space. That is the cost spec decision 4
    accepted for P and R.
- **The full-text `LIMIT` still changes the same 2 of 16 answers,** with or without `where`.

## Large library, 7,798 passages (median over 16 questions)

| Statement | JVM 26.10.1 (pin) | JVM snapshot | native snapshot | identical |
|---|---|---|---|---|
| shipped | 44.3 | 48.8 / 51.3 / 43.7 | 73.8 / 67.9 / 74.3 / 71.5 | 16/16 |
| no filter | 15.5 | 18.0 / 16.9 | 23.5 / 24.7 / 21.1 | 16/16 |
| post-filter (P) | 15.3 | 15.1 / 15.2 / 14.3 | 30.7 / 24.8 / 25.6 / 23.5 | 16/16 |
| maintainers' three edits (R) | 12.3 | 12.0 / 9.8 / 11.5 | 20.9 / 15.9 / 18.6 / 15.0 | 14/16 |
| no filter, rerank top 80 | 13.9 | 13.8 / 15.7 | 26.6 / 22.8 / 20.5 | 16/16 |
| `where: { embed_space: :space }` | refused | 14.5 | 24.9 / 24.6 | 16/16 |
| `where: 'embed_space = :space'` | refused | 16.7 | 24.8 / 24.9 | 16/16 |
| `where` map, rerank top 80 | refused | 15.1 | 21.7 / 20.4 | 16/16 |
| `where` map, three edits | refused | 13.0 | 18.2 / 17.3 | 14/16 |

All figures are in ms. Each cell is one run; a slash separates repeated runs.

## Gold sets on the native snapshot

| Set | Questions | r@1 / r@3 / r@10 / MRR, every statement | identical: P / `where` map / R |
|---|---|---|---|
| office | 20 | 0.700 / 0.850 / 1.000 / 0.804 | 20 / 20 / 20 |
| complex | 34 | 0.882 / 1.000 / 1.000 / 0.941 | 34 / 34 / 34 |
| code, English | 24 | 0.917 / 0.958 / 0.958 / 0.938 | 24 / 24 / 24 |
| code, Italian | 24 | 0.792 / 0.875 / 0.917 / 0.840 | 24 / 24 / 23 |
| code, find-by-name | 6 | 0.833 / 1.000 / 1.000 / 0.889 | 6 / 6 / 6 |

- **On these small indexes every variant takes 7 to 23 ms,** and the shipped statement is always
  the slowest. They rank no arm by speed.
- **The metrics match 26.10.1's** (`2026-10-17-fused-query-maintainers-edits.md`) on every set.

## `where` applies, and 26.10.1 refuses it

- **The predicate is applied.** On the code index, which holds 634 passages in one space,
  `vector.neighbors` with `k` 10 returns 10 rows for the real space and 0 for a space that does
  not exist, in both the map and the condition form.
- **26.10.1 refuses the option loudly:** `CommandSQLParsingException: Unknown option 'where' for
  function 'vector.neighbors'. Allowed: [efSearch, filter, groupBy, groupSize, maxDistance]`. So a
  statement using it can never silently ignore it, but it cannot ship before Aura's pin moves to
  a release that carries #10169.
- **The documented semantics** are in `RecordPredicateRidSet.java` at `15958b48f`.
  - A predicate that keeps few or none leaves the walk short of `k`. The index then tests every
    indexed vector, and the call pays the full load the `filter` subquery pays on every call
    anyway.
  - A named parameter is required, and a positional `?` is refused.
  - A `filter` given as well is checked first, so a scoped search can keep its small `filter`
    and add `where`.

## How the `where` variants were run

`TestFusedQueryVariants` with these lines added in a scratch worktree, not committed, because on
the pinned 26.10.1 they fail the bench:

```go
whereMap := withFilter("{ where: { embed_space: :space }, maxDistance: :max_distance }")
{name: "where: space map", statement: whereMap},
{name: "where: condition", statement: withFilter("{ where: '" + where + "', maxDistance: :max_distance }")},
{name: "where, rerank top 80", statement: rerankTop(whereMap)},
{name: "where, three edits", statement: rerankTop(textLimit(whereMap))},
```

Every database was a copy, so the snapshot never opened an index that 26.10.1 reads.

## What this does not prove

- **Two spaces at once.** Every index measured holds one space, so `where` dropped nothing, and its
  real advantage over P and R stays reasoned. Acceptance 4 of the spec is still the test that shows
  it.
- **A space with few passages beside a large one.** By the documented semantics, `where` then
  degrades to an exhaustive scan. That was not measured.
- **Scoped searches** with `filter` and `where` together.
- **The full code index** (11,771 passages) and the appliance.
- **The release.** `26.11.1-SNAPSHOT` is a build of main, not 26.11.1. Numbers may move before the
  release.
- **One run per cell, on one 4-core host.** The native column varies by up to 7 ms between runs
  of the same statement.
