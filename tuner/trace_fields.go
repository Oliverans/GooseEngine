package tuner

import (
	"fmt"
	"reflect"
	"sort"
	"strings"

	eng "chess-engine/engine"
)

// TraceFieldUse records why an emitted JSON leaf exists.
type TraceFieldUse string

const (
	TraceFormulaInput TraceFieldUse = "formula_input"
	TraceForwardInput TraceFieldUse = "forward_input"
	TraceDiagnostic   TraceFieldUse = "diagnostic"
)

// TraceFieldBinding is one explicit disposition in the tuning-trace schema.
// Slice element fields use [] in their path, for example units.psqt[].u.
type TraceFieldBinding struct {
	Path string
	Use  TraceFieldUse
}

func validatedTraceFieldBindings() ([]TraceFieldBinding, error) {
	bindings := declaredTraceFieldBindings()
	declared := make(map[string]TraceFieldUse, len(bindings))
	for _, binding := range bindings {
		if binding.Path == "" {
			return nil, fmt.Errorf("trace field binding has an empty path")
		}
		switch binding.Use {
		case TraceFormulaInput, TraceForwardInput, TraceDiagnostic:
		default:
			return nil, fmt.Errorf("trace field %q has unknown use %q", binding.Path, binding.Use)
		}
		if _, exists := declared[binding.Path]; exists {
			return nil, fmt.Errorf("trace field %q has more than one disposition", binding.Path)
		}
		declared[binding.Path] = binding.Use
	}

	emitted := make(map[string]struct{})
	collectJSONLeafPaths(reflect.TypeOf(eng.TuningTrace{}), "", emitted)
	var missing, stale []string
	for path := range emitted {
		if _, exists := declared[path]; !exists {
			missing = append(missing, path)
		}
	}
	for path := range declared {
		if _, exists := emitted[path]; !exists {
			stale = append(stale, path)
		}
	}
	if len(missing) != 0 || len(stale) != 0 {
		sort.Strings(missing)
		sort.Strings(stale)
		return nil, fmt.Errorf("tuning trace field coverage mismatch: unbound=%v stale=%v", missing, stale)
	}

	sort.Slice(bindings, func(i, j int) bool {
		return bindings[i].Path < bindings[j].Path
	})
	return bindings, nil
}

func collectJSONLeafPaths(valueType reflect.Type, prefix string, out map[string]struct{}) {
	for valueType.Kind() == reflect.Pointer {
		valueType = valueType.Elem()
	}
	switch valueType.Kind() {
	case reflect.Struct:
		for i := 0; i < valueType.NumField(); i++ {
			field := valueType.Field(i)
			if field.PkgPath != "" {
				continue
			}
			name := jsonFieldName(field)
			if name == "-" {
				continue
			}
			path := name
			if prefix != "" {
				path = prefix + "." + name
			}
			collectJSONLeafPaths(field.Type, path, out)
		}
	case reflect.Slice:
		element := valueType.Elem()
		for element.Kind() == reflect.Pointer {
			element = element.Elem()
		}
		if element.Kind() == reflect.Struct {
			collectJSONLeafPaths(element, prefix+"[]", out)
		} else {
			out[prefix] = struct{}{}
		}
	case reflect.Array:
		out[prefix] = struct{}{}
	default:
		out[prefix] = struct{}{}
	}
}

func jsonFieldName(field reflect.StructField) string {
	tag := field.Tag.Get("json")
	if tag == "" {
		return field.Name
	}
	name, _, _ := strings.Cut(tag, ",")
	if name == "" {
		return field.Name
	}
	return name
}

func declaredTraceFieldBindings() []TraceFieldBinding {
	var out []TraceFieldBinding
	add := func(use TraceFieldUse, paths ...string) {
		for _, path := range paths {
			out = append(out, TraceFieldBinding{Path: path, Use: use})
		}
	}

	add(TraceForwardInput,
		"schemaVersion",
		"sideToMove",
		"piecePhase",
		"totalPhase",
		"theoreticalDraw",
		"fixed.mg",
		"fixed.eg",
	)
	add(TraceDiagnostic,
		"fen",
		"reference.buckets.mg",
		"reference.buckets.eg",
		"reference.whitePerspective",
		"reference.sideToMove",
		"units.center.locked",
		"units.pawn.candidatePassers[].source",
	)
	add(TraceFormulaInput,
		"units.material",
		"units.psqt[].p",
		"units.psqt[].s",
		"units.psqt[].u",
		"units.mobility.knight",
		"units.mobility.bishop",
		"units.mobility.rook",
		"units.mobility.queen",
		"units.pawn.isolatedOpposed",
		"units.pawn.isolatedUnopposed",
		"units.pawn.doubledOpposed",
		"units.pawn.doubledUnopposed",
		"units.pawn.backwardOpposed",
		"units.pawn.backwardUnopposed",
		"units.pawn.weakLever",
		"units.pawn.blocked",
		"units.pawn.connected.white",
		"units.pawn.connected.black",
		"units.pawn.passed[].i",
		"units.pawn.passed[].u",
		"units.pawn.candidatePassers[].side",
		"units.pawn.candidatePassers[].targets",
		"units.piece.knightOutpost",
		"units.piece.knightTropism",
		"units.piece.bishopOutpost",
		"units.piece.badBishop",
		"units.piece.bishopPair",
		"units.piece.rookSemiOpen",
		"units.piece.rookOpen",
		"units.piece.rookFileCountOpen",
		"units.piece.rookFileCountSemi",
		"units.piece.rookStacked",
		"units.piece.rookSeventh",
		"units.piece.queenCentralized",
		"units.piece.kingMinorDefenders",
		"units.center.openness",
		"units.space.white.safe",
		"units.space.white.behindPawn",
		"units.space.white.semiOpen",
		"units.space.white.open",
		"units.space.white.pieceCount",
		"units.space.black.safe",
		"units.space.black.behindPawn",
		"units.space.black.semiOpen",
		"units.space.black.open",
		"units.space.black.pieceCount",
		"units.space.blockedPawns",
		"units.shelterStorm.shelter",
		"units.shelterStorm.stormFree",
		"units.shelterStorm.stormBlocked",
		"units.danger.white.attackers",
		"units.danger.white.ringHits",
		"units.danger.white.safeChecks",
		"units.danger.white.unsafeChecks",
		"units.danger.white.hasQueen",
		"units.danger.black.attackers",
		"units.danger.black.ringHits",
		"units.danger.black.safeChecks",
		"units.danger.black.unsafeChecks",
		"units.danger.black.hasQueen",
		"units.kingPassers[].side",
		"units.kingPassers[].relativeRank",
		"units.kingPassers[].enemyDistance",
		"units.kingPassers[].ownDistance",
		"units.imbalance.totalPawns",
		"units.imbalance.knightDiff",
		"units.tempo",
	)
	return out
}
