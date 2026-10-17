# Fused query: the dense filter after the rerank

Proposed on 2026-10-17 at `1652e65`.
- **Queued.** The operator queued it after the workbook-row slice ("mettila in coda dopo le righe
  Excel"), so it lands after `2026-10-17-workbook-row-passages.md` closes.
- **The direction is approved.** This written spec is not yet.

It answers ArcadeDB's maintainers.
- On 2026-10-16 Roberto Franchini proposed three edits to Aura's fused query: drop the dense
  filter, add `LIMIT :fetch` to the full-text leg, and put `limit` in `vector.fuse`.
- He opened ArcadeData/arcadedb#10129. Its fix, PR #10181 (milestone 26.11.1, not released on
  2026-10-17), stops `vector.rerank` and `vector.fuse` reloading and copying every record.
- The engine-side filter that would make this spec unnecessary is #10169 (a `where` predicate
  on `vector.neighbors`), and it is still open.

Risk tier: **low.**
- One SQL statement changes, in `internal/arcadedb/document_retrieval.go`.
- No schema, no migration, no env var, and no change to what the agent receives on the measured
  sets.

## The problem, measured

**1. The dense filter is most of the query's cost.** `vector.neighbors` receives
`{ filter: (SELECT @rid FROM DocumentPassage WHERE <scope> AND embed_space = :space).@rid }`. That
materialises every passage RID of the space before the ANN search. Measured on 2026-10-16 on
ArcadeDB 26.10.2, over 7,798 real passages with real embeddings and 16 questions, the median of
7 runs:

| Variant | JVM | native |
|---|---|---|
| the shipped statement | 48.4 ms | 70.7 ms |
| no filter | 16.7 ms | 26.8 ms |
| `LIMIT :fetch` on the full-text leg | 44.4 ms | 63.4 ms |
| `limit` in `vector.fuse` | 52.5 ms | 69.4 ms |
| all three edits | 13.3 ms | 18.8 ms |

These numbers were reported to the operator and the maintainers, but **no verification file
holds them.** Acceptance 1 re-measures them and records them.

**2. Moving the filter after the rerank changes no answer anywhere it was measured.**
- In the variant called "post-filter" (`TestFusedQueryVariants`), the dense leg runs unfiltered.
  The full-text leg keeps its scope. The outer `WHERE` gains `embed_space = :space`.
- It gave identical answers on all 24 English and 24 Italian coding questions
  (`2026-10-16-coding-agent-questions-over-aura-sources.md`, "The filter variants").
- It gave identical answers on all 20 office and 34 complex questions
  (`2026-10-16-complex-documents-retrieval-quality.md`, "Query variants"), and on the 16 above.

**3. The other two edits change answers or gain nothing.**
- **The full-text `LIMIT`** changed 2 of the 16 answers. Aura's rerank re-scores the whole fused
  pool by cosine, so a passage past the full-text leg's limit can still win.
- **Fuse's `limit`** gains nothing on the JVM and is not in the 26.10.2 manual. Its documentation
  is arcadedb-docs#494, still open.

## Goal

The fused query costs about what the unfiltered one does, and returns the same answers on every
measured set. Embedding spaces stay isolated, and a caller's document scope keeps holding.

## Decisions (proposed)

### 1. An unscoped search filters the space after the rerank

When the caller names no documents and no sources, `fusedStatement` changes three ways:
- `vector.neighbors` gets `{ maxDistance: :max_distance }` and no `filter`;
- the full-text leg keeps `embed_space = :space`;
- the outer clause becomes `WHERE score >= :min_relevance AND embed_space = :space`.

This is the measured "post-filter" variant, clause for clause.

### 2. A scoped search keeps today's filter

When `CandidateFilter` carries `DocumentIDs`, `SourceKeys` or `SourcePrefixes`, the statement is
the one shipped today.
- **The post-filter variant was measured only unscoped.** With a scope, the 200 unfiltered
  neighbours of the whole library may hold few or none of the named documents' passages, and the
  dense leg would then contribute nothing.
- **A scoped filter subquery is small,** so its cost is not the one measured above.

### 3. What stays

The rerank, its `:candidates` (four times the limit, between 20 and the configured maximum),
`fusedDenseNeighbours` (200), the floors and the group size all stay as they are. The full-text
`LIMIT` and fuse's `limit` are not taken.

### 4. The known cost: fewer results while two spaces coexist

During a change of embedding model, a library holds two spaces until its re-embed finishes
(prd.md §11).
- The unfiltered neighbours then mix both spaces. The rows of the other space are dropped after
  the rerank, so a search can return fewer than its limit.
- **It never returns another space's passage.** The outer filter guarantees that.
- Whether the response makes that window visible to the agent is not checked here.
  Acceptance 4 records what it shows.

## Shape

| File | Change |
|---|---|
| `internal/arcadedb/document_retrieval.go` | `fusedStatement` takes whether the request is scoped and emits the two shapes; the doc comment cites this spec and #10169 |
| `internal/arcadedb/document_retrieval_test.go` | the two statements, clause by clause |
| `internal/arcadedb/document_query_variants_bench_test.go` | "shipped" becomes the new statement; the old one stays as a variant, "dense filter inside neighbours" |
| `prd.md` §11 | the amendment, before the code |

### New env vars

None.

## Acceptance

1. **Latency, recorded.** Run `TestFusedQueryVariants` on the workstation's `cb-arcade` code
   index (11,771 passages), on ArcadeDB at Aura's pin. Record the shipped and new statements'
   medians in a dated verification file. This replaces the chat-only numbers of problem 1.
2. **Answers.** `TestFusedQueryVariants` and `TestCodingAgentQuestions` on every set: English,
   Italian, find-by-name, office, complex, and the PDF corpus where it exists. Every answer must
   be identical to the shipped statement's. A difference stops the slice until it is explained.
3. **Scope.** An integration test runs a scoped search (`DocumentIDs` of two documents) and an
   unscoped one over the same library, and checks that the scoped one returns only those
   documents.
4. **Two spaces.** An integration test builds a library with passages in two spaces, and checks
   that a search in either space returns only its own passages.
5. **Unit, Go.** `go test -race` on `internal/arcadedb`, and the package keeps its coverage
   policy.

## Open questions

1. **When ArcadeDB 26.11.1 ships, re-measure.** #10181 removes the double record loads. It may
   shrink the gap between the two statements, or widen it.
2. **When #10169 lands,** a `where` on `vector.neighbors` would filter inside the ANN search. It
   would replace both shapes with one.

## What this spec does not prove

- **Latency on the appliance.** The numbers are from a workstation and a cloud container.
- **Recall for scoped searches,** which keep today's statement.
- **Anything about the full-text `LIMIT`.** It changed answers and is not taken.
