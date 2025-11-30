# Eval Tuning Stages – Plan

General rule after Stage 1:

- **Tuned** = toggles `true`, included in θ, gradients applied.
- **Frozen** = parameters from *earlier* stages; still used in eval, but *not* in θ anymore.
- **Disabled** = toggles `false`; term contributes `0` during that stage and is not tuned yet.

---

## Stage 1 – Material + PSQT Baseline

**Goal:** Get a clean “raw score = material + PSQT” skeleton.

**Tuned (and active in eval):**
- `MaterialMG`, `MaterialEG`
- `PSTMG`, `PSTEG`  
  → All piece/pawn PSQT tables + MG/EG material values.

**Frozen:**
- None (first stage).

**Disabled (both in eval & tuning):**

- `PassersMG`, `PassersEG`
- All `P1[...]`:
  - `BishopPairMG`, `BishopPairEG`
  - `RookSemiOpenFileMG`, `RookOpenFileMG`
  - `SeventhRankEG`
  - `QueenCentralizedEG`
  - `QueenInfiltrationMG`, `QueenInfiltrationEG`
- All `PawnStruct[...]`:
  - `DoubledMG/EG`, `IsolatedMG/EG`, `ConnectedMG/EG`, `PhalanxMG/EG`
  - `BlockedMG/EG`, `BackwardMG/EG`
  - `PawnLeverMG/EG`, `WeakLeverMG/EG`
- All `MobilityMG[*]`, `MobilityEG[*]`
- `KingTable`, `KingCorr[...]`:
  - `KingSemiOpen`, `KingOpen`, `KingMinor`, `KingPawnMG`
- All `Extras[...]`
- All `Imbalance[...]`
- `WeakSquaresMG`, `WeakKingsMG`, `Tempo`

---

## Stage 2 – Passed Pawns (Passers)

**Goal:** Let passed-pawn terms sit on top of fixed pawn PSQT.

**Tuned:**
- `PassersMG`, `PassersEG`  
  → Passed pawn PSQT / bonuses.

**Frozen (active, not trainable):**
- Stage 1:
  - `MaterialMG`, `MaterialEG`
  - `PSTMG`, `PSTEG`

**Disabled:**
- All `P1[...]`
- All `PawnStruct[...]`
- All `MobilityMG[*]`, `MobilityEG[*]`
- `KingTable`, `KingCorr[...]`
- All `Extras[...]`
- All `Imbalance[...]`
- `WeakSquaresMG`, `WeakKingsMG`, `Tempo`

---

## Stage 3 – Core Pawn Structure

**Goal:** Basic pawn structure terms, no aggressive levers/storm yet.

**Tuned (in `PawnStruct`):**
- `DoubledMG`, `DoubledEG`
- `IsolatedMG`, `IsolatedEG`
- `ConnectedMG`, `ConnectedEG`
- `PhalanxMG`, `PhalanxEG`
- `BlockedMG`, `BlockedEG`
- `BackwardMG`, `BackwardEG`

**Frozen:**
- Stage 1: Material + PSQT
- Stage 2: Passers

**Disabled:**
- `PawnLeverMG`, `PawnLeverEG`
- `WeakLeverMG`, `WeakLeverEG`
- All `P1[...]`
- Mobility, KingTable/KingCorr, Extras, Imbalance, WeakSquares/WeakKings, Tempo

---

## Stage 4 – Piece Activity (Mobility + Simple Piece Extras)

**Goal:** Make the eval understand active pieces and outposts.

### 4a. Mobility

**Tuned:**
- `MobilityMG_N`, `MobilityMG_B`, `MobilityMG_R`, `MobilityMG_Q`
- `MobilityEG_N`, `MobilityEG_B`, `MobilityEG_R`, `MobilityEG_Q`
(Optionally later: `Mobility*_P`, `Mobility*_K`.)

### 4b. Simple piece extras

**Tuned (subset of `P1` + `Extras`):**
- `BishopPairMG`, `BishopPairEG` (P1)
- `ExtraKnightOutpostMG`, `ExtraKnightOutpostEG`
- `ExtraKnightThreatsMG`, `ExtraKnightThreatsEG`
- `ExtraBishopOutpostMG`
- Optional: `ExtraKnightMobCenterMG`, `ExtraBishopMobCenterMG` (if used in eval).

**Frozen:**
- Stages 1–3.

**Disabled (for now):**
- Rook/queen structure extras:
  - `ExtraStackedRooksMG`, `ExtraRookXrayQueenMG`, `ExtraConnectedRooksMG`
  - `ExtraBishopXrayKingMG`, `ExtraBishopXrayRookMG`, `ExtraBishopXrayQueenMG`
- Pawn storm / proximity / lever-storm extras.
- King safety, Imbalance, WeakSquares/WeakKings, Tempo.
- P1 rook-file scalars & queen infiltration/centralization (except bishop pair already tuned).

---

## Stage 5 – King Safety (+ Optional Weak Squares)

**Goal:** Tune king safety once board/attack & activity are sane.

**Tuned:**
- `KingTable`
- `KingSemiOpen`, `KingOpen`, `KingMinor`, `KingPawnMG`

**Recommended also here:**
- `WeakSquaresMG`
- `WeakKingsMG`

**Frozen:**
- Stages 1–4.

**Disabled:**
- Pawn-lever terms (`PawnLever*`, `WeakLever*`)
- Pawn storm/proximity extras
- Rook/queen structure extras
- Imbalance
- `QueenInfiltrationMG`, `QueenInfiltrationEG`
- `QueenCentralizedEG`
- `Tempo`

---

## Stage 6 – Advanced Pawn Aggression (Levers & Storm)

**Goal:** Tune aggressive pawn terms now that structure + king safety are fixed.

**Tuned (in `PawnStruct`):**
- `PawnLeverMG`, `PawnLeverEG`
- `WeakLeverMG`, `WeakLeverEG`

**Tuned (in `Extras`):**
- `ExtraPawnStormMG`
- `ExtraPawnProximityMG`
- `ExtraPawnLeverStormMG`

**Frozen:**
- Stages 1–5.

**Disabled:**
- Rook/queen structure extras
- Imbalance
- `QueenInfiltrationMG`, `QueenInfiltrationEG`
- `QueenCentralizedEG`
- `Tempo`

---

## Stage 7 – Rook & Bishop/Queen Structure Extras + Rook-File P1

**Goal:** Tune rook-file use and rook/bishop x-ray patterns.

**Tuned (P1):**
- `RookSemiOpenFileMG`
- `RookOpenFileMG`
- `SeventhRankEG`

**Tuned (Extras):**
- `ExtraStackedRooksMG`
- `ExtraConnectedRooksMG`
- `ExtraRookXrayQueenMG`
- `ExtraBishopXrayKingMG`
- `ExtraBishopXrayRookMG`
- `ExtraBishopXrayQueenMG`

**Frozen:**
- Stages 1–6.

**Disabled:**
- `QueenInfiltrationMG`, `QueenInfiltrationEG`
- `QueenCentralizedEG`
- Imbalance
- `Tempo`

---

## Stage 8 – Material Imbalances

**Goal:** Non-linear material corrections (Kaufman-style).

**Tuned (all `Imbalance[...]`):**
- `ImbKnightPerPawnMG`, `ImbKnightPerPawnEG`
- `ImbBishopPerPawnMG`, `ImbBishopPerPawnEG`
- `ImbMinorsForMajorMG`, `ImbMinorsForMajorEG`
- `ImbRedundantRookMG`, `ImbRedundantRookEG`
- `ImbRookQueenOverlapMG`, `ImbRookQueenOverlapEG`
- `ImbQueenManyMinorsMG`, `ImbQueenManyMinorsEG`

**Frozen:**
- Stages 1–7.

**Disabled:**
- `QueenInfiltrationMG`, `QueenInfiltrationEG`
- `QueenCentralizedEG`
- `Tempo`

---

## Stage 9 – Weak Squares & Queen Infiltration (if not already done)

If you didn’t tune weak squares together with king safety in Stage 5, do them now with queen infiltration.

**Tuned:**
- `WeakSquaresMG`
- `WeakKingsMG`
- `QueenInfiltrationMG`
- `QueenInfiltrationEG`

**Frozen:**
- Stages 1–8.

**Disabled:**
- `QueenCentralizedEG`
- `Tempo`

(If `WeakSquaresMG/WeakKingsMG` were already tuned in Stage 5, just tune `QueenInfiltrationMG/EG` here.)

---

## Stage 10 – Tempo

**Goal:** Tune global tempo bonus last.

**Tuned:**
- `Tempo`

**Frozen:**
- Stages 1–9.

**Disabled:**
- `QueenCentralizedEG` (if you still haven’t touched it).

---

## Stage 11 – Queen Centralization EG (Optional, Strongly Constrained)

**Goal:** Optional small tweak on top of queen PSQT, not a replacement.

**Tuned:**
- `QueenCentralizedEG`

**Frozen:**
- All previous stages.

**Notes:**

- Consider:
  - Small learning rate,
  - Strong regularization,
  - Clamp to a narrow range (e.g. ±10–20cp),
  - Or simply **skip this stage** and hand-pick a small constant if you don’t trust it.

---