# Texel-Style PST Tuner (Go)

Minimal, modular Texel-style tuner for PST (MG/EG, tapered) with logistic link, MSE loss, and AdaGrad.
It reads a TSV/CSV with `FEN<TAB>LABEL` where `LABEL` is in `{1-0, 0-1, 1/2-1/2}` or numeric `{1, 0, 0.5}`.

## Build
```bash
go build -o texel ./cmd/texel
```

## Run
```bash
./texel -data positions.tsv -epochs 3 -batch 32768 -lr 0.2 -k 0.004 -label white
# If labels mean side-to-move probabilities, use:
./texel -data positions.tsv -label side
# Start from existing PST JSON and save results:
./texel -data positions.tsv -init pst_seed.json -out pst_out.json
```

## File layout
- `cmd/texel/main.go` - CLI entry.
- `tuner/*.go` - dataset loader, PST eval, loss (Texel), optimizer (AdaGrad), trainer, JSON IO, utilities.

## Notes
- Logistic mapping `p = 1/(1+exp(-k*E))` (Texel uses 10-base; equivalent via scale).
- AdaGrad is stable for sparse-ish linear features.
- You can extend the feature set beyond PST by following the same gradient pattern.

### Regularization (Anchored L2)
To keep dense, always-on features from over-compensating, we use anchored L2 that penalizes deviation from engine defaults (evaluation.go baselines):

- Mobility (MG/EG): anchored to `mobilityValueMG/EG` defaults.
- KingSafetyTable (100 bins): anchored to `KingSafetyTable` default.

Anchored L2 adds `λ * (θ - θ0)` to the gradient for the corresponding block (`θ0` is the engine default), in addition to the standard L2 applied to parameters that were touched in the current batch. This stabilizes tuning while allowing movement where the data strongly supports it.

### Per‑group LR scaling
Group-specific learning-rate multipliers balance update magnitudes:

- PST: 1.0
- Material: 0.25
- Passers: 0.5
- Phase 1 / Pawn‑structure scalars: 0.5
- Mobility MG/EG: 0.02
- KingSafetyTable: 0.05
- King-safety correlates: 0.5
- Extras (outposts, xray, etc.): 0.5
- Weak squares + tempo: 0.5

## References
- Chess Programming Wiki - Texel's Tuning Method.
- Andrew Grant (Ethereal) - Tuning.pdf.
