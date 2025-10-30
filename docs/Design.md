# Go Chess Move Generator – Design Overview

This document outlines the structure and design of the chess move generation engine written in Go. The project is organized into separate files and components for clarity and maintainability, with an emphasis on high performance (avoiding allocations in critical code paths) and ease of testing.

## Project Structure
- **engine/** – Core engine package containing chess logic:
  - *board.go* – Board representation, piece constants, and fundamental types.
  - *fen.go* – FEN parsing and formatting (for setting up board positions from strings).
  - *move.go* – Move representation and encoding (compact move storage and utilities).
  - *movegen.go* – Move generation logic (currently stubbed out with TODOs for implementation).
  - *zobrist.go* – Zobrist hashing for representing board state with a 64-bit hash.
- **docs/** – Documentation and test data:
  - *Design.md* – (This file) Design overview of the chess move generator.
  - *PerftPositions.md* – Known test positions (in FEN) and expected move counts (perft results).
- **tests/** – Test suite for the engine:
  - *movegen_test.go* – Tests for move generation (currently uses placeholders/skips).
  - *perft_test.go* – Perft tests for move counting (placeholders until movegen is implemented).

(See the code and tests for details and TODOs.)
