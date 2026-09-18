package protocol

import "fmt"

// Validate はrequestが契約のoneOfを満たすかどうかを検証する。
// actionごとに必要なfieldを確認し、他方のfieldが混ざっていないことも確認する。
func (r Request) Validate() *Error {
	if r.Version != Version {
		return NewError(CodeUnsupportedVersion, "対応していないバージョンです")
	}

	switch r.Action {
	case ActionNewGame:
		if r.GameID == "" {
			return NewError(CodeInvalidRequest, "gameIdが必要です")
		}
		if r.Seed == nil {
			return NewError(CodeInvalidRequest, "seedが必要です")
		}
		if r.State != nil || r.Command != nil {
			return NewError(CodeInvalidRequest, "newGameはstateとcommandを持てません")
		}
		return nil

	case ActionApplyCommand:
		if r.State == nil {
			return NewError(CodeInvalidRequest, "stateが必要です")
		}
		if r.Command == nil {
			return NewError(CodeInvalidRequest, "commandが必要です")
		}
		if r.GameID != "" || r.Seed != nil {
			return NewError(CodeInvalidRequest, "applyCommandはgameIdとseedを持てません")
		}
		if err := r.State.Validate(); err != nil {
			return err
		}
		return r.Command.Validate()

	default:
		return NewError(CodeInvalidRequest, fmt.Sprintf("未対応のactionです: %q", string(r.Action)))
	}
}

// Validate はcommandが契約のoneOfを満たすかどうかを検証する。
func (c Command) Validate() *Error {
	if c.CommandID == "" {
		return NewError(CodeInvalidCommand, "commandIdが必要です")
	}
	if !c.Player.Valid() {
		return NewError(CodeInvalidCommand, "playerが不正です")
	}
	if c.ExpectedTurn < 0 {
		return NewError(CodeInvalidCommand, "expectedTurnが負の値です")
	}

	switch c.Type {
	case CommandPlace:
		if c.Row == nil || c.Col == nil {
			return NewError(CodeInvalidCommand, "placeにはrowとcolが必要です")
		}
		if !InBounds(*c.Row, *c.Col) {
			return NewError(CodeInvalidCoordinate, "座標が盤外です")
		}
		return nil

	case CommandPass:
		if c.Row != nil || c.Col != nil {
			return NewError(CodeInvalidCommand, "passはrowとcolを持てません")
		}
		return nil

	default:
		return NewError(CodeInvalidCommand, fmt.Sprintf("未対応のcommand typeです: %q", string(c.Type)))
	}
}

// Validate はboardが64マスで、各マスがnullまたは0〜255であることを検証する。
// Cellがuint8なので値域はdecode時点で保証され、ここでは要素数だけを確認する。
func (b Board) Validate() *Error {
	if len(b) != BoardCells {
		return NewError(CodeInvalidState, fmt.Sprintf("boardは%d要素である必要があります: %d", BoardCells, len(b)))
	}
	return nil
}

// Validate はstateが契約の必須fieldと値域、phase依存の条件を満たすかどうかを検証する。
func (s State) Validate() *Error {
	if s.Version != Version {
		return NewError(CodeUnsupportedVersion, "対応していないバージョンです")
	}
	if s.GameID == "" {
		return NewError(CodeInvalidState, "gameIdが必要です")
	}
	if err := s.Board.Validate(); err != nil {
		return err
	}
	if s.TurnNumber < 0 {
		return NewError(CodeInvalidState, "turnNumberが負の値です")
	}
	if !s.CurrentPlayer.Valid() {
		return NewError(CodeInvalidState, "currentPlayerが不正です")
	}
	if s.ConsecutivePasses < 0 || s.ConsecutivePasses > 2 {
		return NewError(CodeInvalidState, "consecutivePassesが0〜2の外です")
	}
	if !s.Phase.Valid() {
		return NewError(CodeInvalidState, "phaseが不正です")
	}

	for _, move := range s.LegalMoves {
		if !InBounds(move.Row, move.Col) {
			return NewError(CodeInvalidState, "legalMovesに盤外の座標があります")
		}
	}

	switch s.Phase {
	case PhasePlaying:
		if s.Winner != nil {
			return NewError(CodeInvalidState, "playing中はwinnerがnullである必要があります")
		}
	case PhaseFinished:
		// winnerがnullの終局は引き分け。playingとはphaseで区別できる
		if s.Winner != nil && !s.Winner.Valid() {
			return NewError(CodeInvalidState, "winnerが不正です")
		}
		if len(s.LegalMoves) != 0 {
			return NewError(CodeInvalidState, "finished時はlegalMovesが空である必要があります")
		}
	}

	return nil
}

// Validate はeventが契約のtypeごとの必須fieldを満たすかどうかを検証する。
func (e Event) Validate() *Error {
	if e.CommandID == "" {
		return NewError(CodeInvalidCommand, "eventにcommandIdが必要です")
	}
	if !e.Player.Valid() {
		return NewError(CodeInvalidCommand, "eventのplayerが不正です")
	}
	if e.TurnNumber < 1 {
		return NewError(CodeInvalidCommand, "eventのturnNumberは1以上である必要があります")
	}

	switch e.Type {
	case EventPlaced:
		if e.Row == nil || e.Col == nil {
			return NewError(CodeInvalidCommand, "placed eventにはrowとcolが必要です")
		}
		if !InBounds(*e.Row, *e.Col) {
			return NewError(CodeInvalidCoordinate, "eventの座標が盤外です")
		}
		if e.PlacedColor == nil {
			return NewError(CodeInvalidCommand, "placed eventにはplacedColorが必要です")
		}
		for _, change := range e.Changes {
			if !InBounds(change.Row, change.Col) {
				return NewError(CodeInvalidCoordinate, "changesに盤外の座標があります")
			}
		}
		return nil

	case EventPassed:
		if e.Row != nil || e.Col != nil || e.PlacedColor != nil || len(e.Changes) > 0 {
			return NewError(CodeInvalidCommand, "passed eventは着手情報を持てません")
		}
		return nil

	default:
		return NewError(CodeInvalidCommand, fmt.Sprintf("未対応のevent typeです: %q", string(e.Type)))
	}
}

// Validate はresponseが契約のoneOfを満たすかどうかを検証する。
// 成功応答はstateを持ちerrorを持たない。拒否応答はerrorを持ちeventを持たない。
func (r Response) Validate() *Error {
	if r.Version != Version {
		return NewError(CodeUnsupportedVersion, "対応していないバージョンです")
	}

	if r.OK {
		if r.Error != nil {
			return NewError(CodeInvalidRequest, "成功応答はerrorを持てません")
		}
		if r.State == nil {
			return NewError(CodeInvalidState, "成功応答にはstateが必要です")
		}
		if err := r.State.Validate(); err != nil {
			return err
		}
		if r.Event != nil {
			return r.Event.Validate()
		}
		return nil
	}

	if r.Error == nil {
		return NewError(CodeInvalidRequest, "拒否応答にはerrorが必要です")
	}
	if !r.Error.Code.Valid() {
		return NewError(CodeInvalidRequest, fmt.Sprintf("未知のerror codeです: %q", string(r.Error.Code)))
	}
	if r.Error.Message == "" {
		return NewError(CodeInvalidRequest, "errorにmessageが必要です")
	}
	if r.Event != nil {
		return NewError(CodeInvalidRequest, "拒否応答はeventを持てません")
	}
	if r.State != nil {
		return r.State.Validate()
	}
	return nil
}
