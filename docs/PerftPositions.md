# Perft Test Positions

Perft (performance test) positions are used to verify the correctness of move generation. For each position, we count the number of possible move sequences (nodes) to a certain depth and compare with known correct values. Below are some standard test positions and their known perft results:

## 1. Initial Position
**FEN:** `rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 1`
- Depth 1: **20** moves
- Depth 2: **400** nodes
- Depth 3: **8,902** nodes
- Depth 4: **197,281** nodes
- Depth 5: **4,865,609** nodes

## 2. "Kiwipete" Position (canonical)
**FEN:** `r3k2r/p1ppqpb1/bn2pnp1/2PpP3/1p2P3/P1N2N2/1P2BPPP/R2QKB1R w KQkq - 0 1`
- Depth 1: **48** moves
- Depth 2: **2,039** nodes
- Depth 3: **97,862** nodes
- Depth 4: **4,085,603** nodes
- Depth 5: **193,690,690** nodes

## 3. En Passant Test
**FEN:** `k7/8/8/3pP3/8/8/8/7K w - d6 0 2`
- Depth 1: **5** moves
- Depth 2: **19** nodes

## 4. Promotion Test
**FEN:** `1n5k/P7/8/8/8/8/8/7K w - - 0 1`
- Depth 1: **11** moves

----
By running `engine.Perft` on these positions at various depths and comparing to the known node counts above, you can confirm the move generator's correctness.

## Additional Standard Positions

### Position 3
FEN: `8/2p5/3p4/KP5r/1R3p1k/8/4P1P1/8 w - - 0 1`
- Depth 1: 14
- Depth 2: 191
- Depth 3: 2,812

### Position 4
FEN: `r3k2r/Pppp1ppp/1b3nbN/nP6/BBPQP3/q4N2/Pp2PPPP/R3K2R w KQkq - 0 1`
- Depth 1: 6
- Depth 2: 264
- Depth 3: 9,467

### Position 5
FEN: `rnbq1k1r/pp1Pbppp/2p5/8/2B5/8/PPP1NnPP/RNBQK2R w KQ - 0 1`
- Depth 1: 44
- Depth 2: 1,486
- Depth 3: 62,379

### Position 6
FEN: `r4rk1/1pp1qppp/p1np1n2/2b1p3/2B1P3/2NP1N2/PPP1QPPP/R4RK1 w - - 0 10`
- Depth 1: 46
- Depth 2: 2,079
- Depth 3: 89,890
