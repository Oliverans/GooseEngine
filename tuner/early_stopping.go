package tuner

import (
	"fmt"
	"math"
)

// EarlyStoppingConfig uses unregularized validation Brier. Patience zero
// disables stopping while retaining validation reporting.
type EarlyStoppingConfig struct {
	Patience uint64
	MinDelta float64
}

func (c EarlyStoppingConfig) Validate() error {
	if !finite(c.MinDelta) || c.MinDelta < 0 {
		return fmt.Errorf("early-stopping minimum delta must be finite and non-negative, got %v", c.MinDelta)
	}
	return nil
}

type EarlyStoppingState struct {
	Initialized bool    `json:"initialized"`
	BestBrier   float64 `json:"bestBrier"`
	BestEpoch   uint64  `json:"bestEpoch"`
	BadEpochs   uint64  `json:"badEpochs"`
	Stopped     bool    `json:"stopped"`
}

type EarlyStoppingDecision struct {
	Improved bool
	Stop     bool
	State    EarlyStoppingState
}

// ObserveValidation updates deterministic early-stopping state. Validation
// never changes optimizer state or the learning-rate schedule.
func (t *Trainer) ObserveValidation(epoch uint64, metrics ValidationMetrics) (EarlyStoppingDecision, error) {
	if t == nil {
		return EarlyStoppingDecision{}, fmt.Errorf("cannot update early stopping on a nil trainer")
	}
	brier := metrics.Data.Brier
	if !finite(brier) {
		return EarlyStoppingDecision{}, fmt.Errorf("validation Brier must be finite, got %v", brier)
	}
	config := t.config.EarlyStopping
	if err := config.Validate(); err != nil {
		return EarlyStoppingDecision{}, err
	}
	state := t.earlyStop
	improved := !state.Initialized || brier < state.BestBrier-config.MinDelta
	if improved {
		state = EarlyStoppingState{Initialized: true, BestBrier: brier, BestEpoch: epoch}
	} else {
		if state.BadEpochs == math.MaxUint64 {
			return EarlyStoppingDecision{}, fmt.Errorf("early-stopping bad-epoch counter overflow")
		}
		state.BadEpochs++
	}
	state.Stopped = config.Patience > 0 && state.BadEpochs >= config.Patience
	t.earlyStop = state
	return EarlyStoppingDecision{Improved: improved, Stop: state.Stopped, State: state}, nil
}

func (t *Trainer) EarlyStoppingState() EarlyStoppingState {
	if t == nil {
		return EarlyStoppingState{}
	}
	return t.earlyStop
}
