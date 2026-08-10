# GooseEngine
Chess engine written in Golang. Written with a grand plan in mind, but is for now a ~2400 ELO classic evaluation engine.

Using bitboards and strong core evaluation features.

## Documentation

- Reviewer backend docs: `docs/reviewer/`
- Live review web backend docs: `docs/livereview/`
- Planning / working notes: `docs/planning/`

## Search algorithm
- MultiPV line
- Iterative deepening
- Aspiration windows
- Alpha-beta negamax
- Transposition Table
- Principal Variation Search (PVS)
- Quiescence search
- Check extension
- Singular extension
- Internal iterative deepening (IID)
- PV line tracking

### Search pruning techniques
- Transposition table cutoffs
- Static Null Move Pruning (also known as Reverse futility pruning, RFP)
- Null-move pruning (with verification search)
- Razoring
- Late Move Pruning (LMP)
- Futility pruning
- Late Move Reductions (LMR)
- Quiescence stand-pat pruning
- Quiescence SEE pruning
- Quiescence delta pruning
- ProbCut pruning

### Transposition table implementation type
- Bucketed hash table
- Generation-based aging/replacement

### Move ordering optimizations
- TT/PV move
- Promotion
- MVV-LVA
- SEE-based scoring
- Killer moves
- Counter-moves
- History heuristic

## Evaluation features
- Generic: Material, PSQT, Mobility Tables
- Pawn: Isolated, Doubled, Connected, Phalanx, Passed Pawns, Candidate Passed, Backward, Blocked, Weak Lever, Pawn Storm
- Knight: Outposts, King Tropism
- Bishop: Outposts, Bishop Pair (scaled by center openness), Bad Bishop
- Rook: Open File, Semi-Open File, Stacked/Connected Rooks, Seventh Rank
- Queen: Centralization
- King: Attack Units (inner/outer ring), Open/Semi-Open File Penalty, Minor Piece Defense, Pawn Shield Defense, Weak King Squares, King Passer Proximity, Endgame King Centralization Penalty (Manhattan distance), Mop-up bonus (king distance + defender edge distance)
- Positional features: Space Evaluation, Material Imbalance (knight/bishop imbalance vs pawn count), Center State (knight/bishop mobility + bishop-pair scaling by locked/open center), Theoretical Draw Detection & draw Score Divider, Tempo Bonus, Tapered Evaluation

## Position Metadata Trace

`engine/position_trace.go` exports static explanatory metadata without adding
evaluation terms or entering the search path. The levels are additive:

- `basic`: exact evaluation trace, regional and square control, per-piece
  attacks/mobility/contacts, pawn relations, king topology, pins, and legal
  choices with SEE on captures. Per-piece legal destinations are emitted only
  for the side to move; opponent pseudo-mobility remains available separately.
- `extended`: all basic data plus latent slider lines, static piece-route reach,
  and dependency/criticality records.
- `moves`: all extended data plus one-ply static deltas for every legal move.

For one position in the UCI engine, use `positiontrace basic`,
`positiontrace extended`, or `positiontrace moves` after setting the position.
`explainjson` is an alias. Both commands emit immediately and do not start a
search.

For JSONL batch extraction, send `id<TAB>fen` records on standard input:

```sh
go run ./cmd/positiontrace -level extended < positions.tsv > traces.jsonl
```

Routes, latent scope, criticality, and move deltas are static measurements.
They are candidate explanatory signals to correlate with Stockfish lines, not
claims that a route or transformation is tactically sound.

## Live Review Web UI

The primary interactive review entry point is now `cmd/livereview-web`.

- It serves a browser UI for direct position review and PGN-based game replay.
- PGN text is imported into server memory and exposed as in-memory game IDs.
- Replay review is computed lazily per visited ply and cached in memory.
- Full-game analysis is available through `POST /api/pgn/analyze` with optional `depth`, `timeMs`, and `forceRecompute`.
- Optional opening-book integration supports `-book-epd <path.epd>` and `-book-polyglot <path.bin>` (you can use both together). `-book` remains as a legacy alias for EPD.
- The UI/backend flow is documented under `docs/livereview/`.

Current scope:

- direct move review and eval from the browser
- PGN import, navigation, single-ply review, and full-game analysis
- in-memory caching and progress polling for analysis

Not core yet:

- Certabo support exists, but it is still treated as an auxiliary integration rather than the main livereview flow

## Book Builder
- Command: `cmd/bookbuild`
- Purpose: build a polyglot `.bin` opening book from PGN (SAN parsed against legal moves).
- Filters:
  - minimum `(position, move)` sample count via `--min-games` (default `25`)
  - maximum indexed depth via `--max-ply` (default `20`)
  - minimum mover score `(wins + 0.5 * draws) / total` via `--min-score` (default `0.35`)
- Example:
  - `go run ./cmd/bookbuild --pgn lichess.pgn --polyglot out.bin --max-ply 20 --min-games 25 --min-score 0.35`

## Game Analysis & Segmentation Pipeline

Offline batch analysis runs through `cmd/sequence-analyze` and turns a PGN into
canonical search evidence, a macro game timeline, research-only candidate
beats and episode candidates, and the detector-neutral episode contract.
Forward-only file artifacts:

```
                    ┌─▶ moves.jsonl (legacy-compatible move records)
PGN ──analyze───────┼─▶ search_evidence.jsonl (full search protocol)
                    ├─▶ games.jsonl
                    └─▶ manifest.json

moves.jsonl ──segment──▶ timeline.jsonl + segments.jsonl

moves + search evidence + timeline + segments
  ──episodes──▶ candidate_beats.jsonl + episode_candidates.jsonl
                + episodes.jsonl + episode_games.jsonl + episode_manifest.json
```

- **analyze** — Stockfish search per ply → eval, PV-verified motif/tag
  classification, and the multi-PV best-vs-second gap (`gap12`). The unchanged
  move stream is accompanied by full ranked PV lines for the root, played move,
  and after-position search, including actual depth, selective depth, nodes,
  mate distance, WDL when supplied, bounds, and optional reactive/preparatory
  null probes. If the played move is absent from stored MultiPV, a constrained
  `searchmoves` search records it. The manifest hashes the PGN, engine binary,
  and opening-book inputs. `-multipv N` defaults to 3 and depth defaults to 12.
- **segment** — replays the analyzed game once to emit the versioned macro
  regime timeline (`timeline.jsonl`), then runs the existing **combined
  segmenter** (`review/segment/combiner.go`). The timeline records
  A/book, B/chapter, C/transition, D/endgame, and unknown spans plus structure
  overlays and transitions. A versioned 37-family lifecycle registry decides
  which geometric matches can own B; uncertain, setup, handoff, masked, and
  material-degraded matches remain diagnostic overlays. Its transitions are
  mandatory segment boundaries;
  A/book is kept as one segment, while the combiner's existing local cuts are
  preserved inside the other regimes. `breaks.jsonl` records whether each cut
  came from local logic, the macro timeline, or both.
- **episodes** — joins moves, search evidence, the A/B/C/D timeline, and the
  existing segments. It exports each old segment as an `uninterpreted`
  `candidate_beats.jsonl` record, then emits overlapping, `researchOnly`
  windows in `episode_candidates.jsonl`. Local one-to-three-beat windows,
  complete regime contexts, and immediate macro-transition contexts are
  included; B→D and C→D are treated like every other transition. Candidate
  records have observations and source annotations but no episode kind, owner,
  purpose, lifecycle, value, judgment, or outcome. Optional detector sidecars
  remain separately validated against the semantic episode contract. With none
  enabled, `episodes.jsonl` is empty. Neither candidate stream is consumed by
  the explain layer.
- **explain** — the current narration layer. It does not consume episodes yet;
  the four-level renderer remains a later integration step.

Example:

```
sequence-analyze -engine ./stockfish-... -pgn games.pgn -out run/moves.jsonl -depth 12 -multipv 3
sequence-analyze segment -in run/moves.jsonl -out-dir run/segmented/
sequence-analyze episodes -moves run/moves.jsonl \
  -search-evidence run/search_evidence.jsonl \
  -timeline run/segmented/timeline.jsonl \
  -segments run/segmented/segments.jsonl \
  -out-dir run/pre_explain/
```

The research-only D-pawn `v2.2.3` detector can be integrated at the last stage
with `-d-pawn-objectives`, `-d-pawn-transactions`, and
`-d-pawn-relations`. These inputs produce descriptive technique/transaction
episodes only; they do not authorize move quality, timing, purpose, or rendered
review claims.

The macro classifier is version-pinned and parity-tested against the frozen
research classifier. The local segmenter is grounded in a 460-game study of GM
play (build → tension → resolution cycles, phase-dependent chapter lengths) and
validated bit-exact against its research prototype plus blind A/B review. Current
design: `docs/reviewer/SEGMENTATION.md`; consolidated evidence:
`research/evidence/EVIDENCE_LEDGER.md`.

## Live Review Detectors

Live Review builds each view in this order:

1. tactical motif tagging and tactical/forcing classification
2. preparatory override for played moves still tagged `strategic:unknown`
3. reactive override for played moves still tagged `strategic:unknown`
4. strategic detector runner
5. `strategic:conversion` fallback for neutral moves in already winning positions
6. eval-bucket fallback (`eval_explain_fallback`) with capped continuation context

`followup` is not a detector in this stack. It is assigned later from sequence context, not by Live Review.

### Preparatory
| Order | Detector | Uses PV line? | Description |
|---:|---|---|---|
| 1 | `preparatory:*` | Yes (probe, depth 6) | Played-move-only null-turn probe that upgrades `strategic:unknown` to `preparatory:fork`, `preparatory:pin`, `preparatory:skewer`, `preparatory:discovered`, `preparatory:deflection`, `preparatory:trapped`, or `preparatory:promotion` when the move clearly prepares the next tactical shot. |

### Reactive
| Order | Detector | Uses PV line? | Description |
|---:|---|---|---|
| 1 | `reactive:*` | Yes (probe, depth 6) | Played-move-only null-turn tactical probe that upgrades `strategic:unknown` to `reactive:prevent_tactic`, `reactive:defend`, or `reactive:ignore_threat`. Explanatory motifs may include `prevention`, `piece_retreat`, `defense`, `displacement`, `counter_threat`, or `contest`. |

### Tactical
| Order | Detector | Uses PV line? | Description |
|---:|---|---|---|
| 1 | `freewin` | No | Detects immediate favorable captures using SEE. |
| 2 | `fork` | Yes (optional) | Detects fork geometry; PV can confirm follow-up conversion. |
| 3 | `pin` | Yes (required) | Tags pin only when PV confirms restriction or conversion outcome. |
| 4 | `skewer` | Yes (optional) | Detects skewer geometry; PV can confirm profitable back-piece win. |
| 5 | `discovered` | Yes (optional) | Detects discovered attacks or checks; PV confirms non-check dual-threat lines. |
| 6 | `backrank` | No | Detects back-rank mating or decisive check patterns. |
| 7 | `zwischenzug` | No | Detects forcing in-between moves over immediate recapture. |
| 8 | `trapped` | No | Detects moves that trap or nearly trap an enemy piece. |
| 9 | `remove_defender` | Yes (required) | Requires PV-confirmed exploitation after defender removal. |
| 10 | `overloaded` | Yes (optional) | Detects overloaded defenders; PV confirms concrete conversion when available. |
| 11 | `deflection` | Yes (required) | Requires geometry plus PV confirmation that a forced piece leaves key defensive duty. |
| 12 | `promotion` | Mixed | Direct promotions are static; promotion threats require PV confirmation. |

### Forcing Classification
| Order | Detector | Uses PV line? | Description |
|---:|---|---|---|
| 1 | `forcing:mate_threat` | No | Detects a move that creates a direct mate-in-1 threat after the reply. |
| 2 | `forcing:trade` | Yes (required) | Detects an immediate same-square recapture sequence when PV confirms the trade. |
| 3 | `forcing:winning_capture` | No | Detects favorable captures via SEE when no higher-priority tactical motif applies. |
| 4 | `forcing:check` | No | Detects immediate checking moves when no higher-priority tactical or forcing category applies. |

### Strategic
| Order | Detector | Uses PV line? | Description |
|---:|---|---|---|
| 1 | `passed_pawns` | Yes (optional) | Detects creation or upgrade of passed-pawn structures with PV persistence checks. |
| 2 | `bad_bishop` | Yes (required) | Requires PV evidence that the bishop stays constrained. |
| 3 | `outpost` | Yes (optional) | Detects new outpost occupation; PV can confirm durability or springboard value. |
| 4 | `king_attack` | No | Detects immediate increase in pressure or coverage around the enemy king zone. |
| 5 | `pawn_break` | No | Detects pawn advances that create immediate pawn-chain tension. |
| 6 | `piece_trap` | No | Detects sharp mobility collapse of an enemy piece due to the move. |
| 7 | `blockade` | No | Detects occupying the square in front of an enemy passed pawn. |
| 8 | `weak_square` | No | Detects occupation of enemy-territory squares enemy pawns cannot challenge. |
| 9 | `seventh_rank` | No | Detects rook or queen reaching the opponent's seventh rank. |
| 10 | `space_control` | Yes (required) | Quiet pawn-space gains requiring PV survival confirmation. |
| 11 | `open_file` | Yes (required) | Requires PV-confirmed rook or queen exploitation of newly opened or semi-open files. |
| 12 | `rook_coordination` | No | Detects new connected or stacked rook formations caused by the move. |
| 13 | `bishop_pair` | Yes (required) | Requires PV persistence of bishop-pair imbalance after elimination. |
| 14 | `simplification` | Yes (required) | Requires PV recapture sequence and preserved advantage after the trade. |
| 15 | `piece_activation` | Yes (optional) | Detects explicit passive-to-active piece activation before generic fallback. |
| 16 | `pawn_structure` | No | Detects direct structural pawn damage or improvement from the move itself. |
| 17 | `prophylaxis` | No | Detects quiet moves that materially reduce the opponent's immediate threat score. |
| 18 | `psqt_fallback` | Yes (optional) | Strict strategic fallback inside the detector runner that returns `strategic:activity` when no earlier strategic detector hits. |

### Strategic Fallbacks
| Order | Fallback | Uses PV line? | Description |
|---:|---|---|---|
| 1 | `strategic:conversion` | No | Applied after the strategic detector runner when the move is roughly neutral (`lossCp <= 20`) but the side was already winning (`|eval| >= 150cp`). |
| 2 | `eval_explain_fallback` | Yes (capped context) | Final wide-net fallback using split eval buckets with at most 6 continuation plies. |

Notes:
- Tactical motifs are tagged first; forcing categories are assigned only if no higher-priority tactical motif wins classification.
- Preparatory and reactive are played-view-only upgrades and only run while the move is still `strategic:unknown`.
- Strategic detectors run only when the move is not already classified as `reactive:*`.
- `psqt_fallback` is part of the strategic detector runner, not the final fallback.
- If the strategic runner still returns `strategic:unknown`, review tries `strategic:conversion` before the eval-bucket fallback.
- Eval-bucket fallback applies at most 6 continuation plies, never the full PV endpoint.

### For Debug

$env:LIVEREVIEW_DEBUG_BEST_PV="1"

## ToDo

| Feature | Trigger | Shows | Priority |
|---|---|---|---|
| Retry mode | "Retry" button on flag | Board resets to position; Hint 1: motif label; Hint 2: target squares; Hint 3: arrow; Wrong attempt: punish category; Correct: best explain + PV | Next |
| Drill mode | Separate section | Missed positions as puzzles; Grouped by motif; Weakness tracking over games | Later |
