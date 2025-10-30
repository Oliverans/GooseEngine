package goosemg

import (
    "errors"
    "strconv"
    "strings"
)

// FENStartPos is the FEN string for the standard initial chess position.
const FENStartPos = "rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 1"

// pieceFromChar converts a FEN character to the corresponding Piece constant.
func pieceFromChar(ch rune) Piece {
    switch ch {
    case 'P':
        return WhitePawn
    case 'N':
        return WhiteKnight
    case 'B':
        return WhiteBishop
    case 'R':
        return WhiteRook
    case 'Q':
        return WhiteQueen
    case 'K':
        return WhiteKing
    case 'p':
        return BlackPawn
    case 'n':
        return BlackKnight
    case 'b':
        return BlackBishop
    case 'r':
        return BlackRook
    case 'q':
        return BlackQueen
    case 'k':
        return BlackKing
    default:
        return NoPiece
    }
}

// charFromPiece converts a Piece constant to its FEN character representation.
func charFromPiece(p Piece) rune {
    switch p {
    case WhitePawn:
        return 'P'
    case WhiteKnight:
        return 'N'
    case WhiteBishop:
        return 'B'
    case WhiteRook:
        return 'R'
    case WhiteQueen:
        return 'Q'
    case WhiteKing:
        return 'K'
    case BlackPawn:
        return 'p'
    case BlackKnight:
        return 'n'
    case BlackBishop:
        return 'b'
    case BlackRook:
        return 'r'
    case BlackQueen:
        return 'q'
    case BlackKing:
        return 'k'
    default:
        return '?' // should not happen for valid pieces
    }
}

// ParseFEN parses a FEN string and returns a new Board set up to that position.
// Returns an error if the FEN is invalid or cannot be parsed.
func ParseFEN(fen string) (*Board, error) {
    fields := strings.Split(fen, " ")
    if len(fields) < 4 {
        return nil, errors.New("invalid FEN: not enough fields")
    }

    board := &Board{}
    // Default no en passant square
    board.enPassantSquare = NoSquare

    // 1. Piece placement
    ranks := strings.Split(fields[0], "/")
    if len(ranks) != 8 {
        return nil, errors.New("invalid FEN: incorrect number of ranks")
    }

    for i, rankStr := range ranks {
        if len(rankStr) == 0 {
            return nil, errors.New("invalid FEN: empty rank description")
        }
        rankIndex := 7 - i // Rank 7 (index) is rank8, down to 0 for rank1
        file := 0
        for _, ch := range rankStr {
            if ch >= '1' && ch <= '8' {
                // Digit: skip that many files (empty squares)
                file += int(ch - '0')
            } else {
                piece := pieceFromChar(ch)
                if piece == NoPiece {
                    return nil, errors.New("invalid FEN: unrecognized piece character")
                }
                if file >= 8 {
                    return nil, errors.New("invalid FEN: too many squares in rank")
                }
                sq := rankIndex*8 + file
                board.pieces[sq] = piece

                // Determine piece color and set bitboards
                var color Color
                if piece&8 != 0 {
                    color = Black
                } else {
                    color = White
                }
                idx := int(color)
                board.occupancy[idx] |= uint64(1) << sq
                ptype := piece & 7 // piece type (1-6)
                switch ptype {
                case 1:
                    board.pawns[idx] |= uint64(1) << sq
                case 2:
                    board.knights[idx] |= uint64(1) << sq
                case 3:
                    board.bishops[idx] |= uint64(1) << sq
                case 4:
                    board.rooks[idx] |= uint64(1) << sq
                case 5:
                    board.queens[idx] |= uint64(1) << sq
                case 6:
                    board.kings[idx] |= uint64(1) << sq
                }
                file++
            }
        }
        if file != 8 {
            return nil, errors.New("invalid FEN: rank does not have 8 columns")
        }
    }

    // 2. Side to move
    switch fields[1] {
    case "w":
        board.sideToMove = White
    case "b":
        board.sideToMove = Black
    default:
        return nil, errors.New("invalid FEN: side to move must be 'w' or 'b'")
    }

    // 3. Castling rights
    board.castlingRights = 0
    if fields[2] != "-" {
        for _, ch := range fields[2] {
            switch ch {
            case 'K':
                board.castlingRights |= CastlingWhiteK
            case 'Q':
                board.castlingRights |= CastlingWhiteQ
            case 'k':
                board.castlingRights |= CastlingBlackK
            case 'q':
                board.castlingRights |= CastlingBlackQ
            default:
                return nil, errors.New("invalid FEN: invalid castling rights character")
            }
        }
    }

    // 4. En passant target square
    if fields[3] != "-" {
        if len(fields[3]) != 2 {
            return nil, errors.New("invalid FEN: invalid en passant square")
        }
        fileChar := fields[3][0]
        rankChar := fields[3][1]
        if fileChar < 'a' || fileChar > 'h' || rankChar < '1' || rankChar > '8' {
            return nil, errors.New("invalid FEN: en passant square out of range")
        }
        file := int(fileChar - 'a')
        rank := int(rankChar - '1')
        board.enPassantSquare = Square(rank*8 + file)
    } else {
        board.enPassantSquare = NoSquare
    }

    // 5. Halfmove clock
    if len(fields) > 4 {
        halfmove, err := strconv.Atoi(fields[4])
        if err != nil {
            return nil, errors.New("invalid FEN: halfmove clock is not a number")
        }
        board.halfmoveClock = halfmove
    }

    // 6. Fullmove number
    if len(fields) > 5 {
        fullmove, err := strconv.Atoi(fields[5])
        if err != nil {
            return nil, errors.New("invalid FEN: fullmove number is not a number")
        }
        board.fullmoveNumber = fullmove
    }

    // Compute initial Zobrist hash for this position
    board.zobristKey = board.ComputeZobrist()
    return board, nil
}

// ToFEN produces the FEN string representation of the board's current state.
func (b *Board) ToFEN() string {
    var sb strings.Builder

    // 1. Piece placement
    for rank := 7; rank >= 0; rank-- {
        emptyCount := 0
        for file := 0; file < 8; file++ {
            sq := rank*8 + file
            p := b.pieces[sq]
            if p == NoPiece {
                emptyCount++
            } else {
                if emptyCount > 0 {
                    sb.WriteByte('0' + byte(emptyCount))
                    emptyCount = 0
                }
                sb.WriteRune(charFromPiece(p))
            }
        }
        if emptyCount > 0 {
            sb.WriteByte('0' + byte(emptyCount))
        }
        if rank > 0 {
            sb.WriteByte('/')
        }
    }
    sb.WriteByte(' ')

    // 2. Side to move
    if b.sideToMove == White {
        sb.WriteByte('w')
    } else {
        sb.WriteByte('b')
    }
    sb.WriteByte(' ')

    // 3. Castling rights
    if b.castlingRights == 0 {
        sb.WriteByte('-')
    } else {
        if b.castlingRights&CastlingWhiteK != 0 {
            sb.WriteByte('K')
        }
        if b.castlingRights&CastlingWhiteQ != 0 {
            sb.WriteByte('Q')
        }
        if b.castlingRights&CastlingBlackK != 0 {
            sb.WriteByte('k')
        }
        if b.castlingRights&CastlingBlackQ != 0 {
            sb.WriteByte('q')
        }
    }
    sb.WriteByte(' ')

    // 4. En passant square
    if b.enPassantSquare != NoSquare {
        file := b.enPassantSquare % 8
        rank := b.enPassantSquare / 8
        sb.WriteByte('a' + byte(file))
        sb.WriteByte('1' + byte(rank))
    } else {
        sb.WriteByte('-')
    }
    sb.WriteByte(' ')

    // 5. Halfmove clock
    sb.WriteString(strconv.Itoa(b.halfmoveClock))
    sb.WriteByte(' ')

    // 6. Fullmove number
    sb.WriteString(strconv.Itoa(b.fullmoveNumber))
    return sb.String()
}
