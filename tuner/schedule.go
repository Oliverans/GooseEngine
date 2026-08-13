package tuner

import (
	"errors"
	"fmt"
)

type LearningRateDrop struct {
	Epoch  uint64
	Factor float64
}

// EpochLearningRateSchedule is deterministic and stateless. Epochs are
// zero-based; a drop applies at the beginning of its named epoch.
type EpochLearningRateSchedule struct {
	Initial float64
	Drops   []LearningRateDrop
}

func ConstantLearningRate(rate float64) EpochLearningRateSchedule {
	return EpochLearningRateSchedule{Initial: rate}
}

func (s EpochLearningRateSchedule) Validate() error {
	if !finite(s.Initial) || s.Initial <= 0 {
		return fmt.Errorf("initial learning rate must be finite and positive, got %v", s.Initial)
	}
	previous := uint64(0)
	for index, drop := range s.Drops {
		if drop.Epoch == 0 {
			return errors.New("learning-rate drops must occur after epoch zero")
		}
		if index != 0 && drop.Epoch <= previous {
			return errors.New("learning-rate drop epochs must be strictly increasing")
		}
		if !finite(drop.Factor) || drop.Factor <= 0 || drop.Factor >= 1 {
			return fmt.Errorf("learning-rate drop factor must be in (0,1), got %v", drop.Factor)
		}
		previous = drop.Epoch
	}
	return nil
}

func (s EpochLearningRateSchedule) Rate(epoch uint64) (float64, error) {
	if err := s.Validate(); err != nil {
		return 0, err
	}
	rate := s.Initial
	for _, drop := range s.Drops {
		if epoch < drop.Epoch {
			break
		}
		rate *= drop.Factor
	}
	if !finite(rate) || rate <= 0 {
		return 0, fmt.Errorf("learning rate at epoch %d is invalid: %v", epoch, rate)
	}
	return rate, nil
}
