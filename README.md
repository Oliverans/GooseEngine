# GooseEngine
Chess engine written in Golang. Written with a grand plan in mind, but is for now a ~2400 ELO classic evaluation engine.

Using bitboards and strong core evaluation features.

## Search algorithm
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
- Bishop: Outposts, Bishop Pair, Bad Bishop
- Rook: Open File, Semi-Open File, Stacked/Connected Rooks, Seventh Rank
- Queen: Centralization (EG only)
- King: Attack Units (inner/outer ring), Open/Semi-Open File Penalty, Minor Piece Defense, Pawn Shield Defense, Weak King Squares, King Passer Proximity, King Centralization Penalty (mop-up), mop-up (Chebyshev distance)
- Positional features: Space Evaluation, Material Imbalance (knight/bishop imbalance vs pawn count), Center State (knight/bishop scaled by locked/open center), Theoretical Draw Detection & draw Score Divider, Tempo Bonus, Tapered Evaluation
