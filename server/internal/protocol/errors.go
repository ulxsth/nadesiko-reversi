package protocol

import "fmt"

// ErrorCode はruntimeが返す安定したエラー識別子。
// 値はdocs/contracts/protocol.mdの表と一致する。
type ErrorCode string

const (
	// CodeInvalidJSON はJSONとして解析できない入力。
	CodeInvalidJSON ErrorCode = "invalid_json"
	// CodeUnsupportedVersion はversionが欠落または未対応。
	CodeUnsupportedVersion ErrorCode = "unsupported_version"
	// CodeInvalidRequest はactionに必要なfieldがない。
	CodeInvalidRequest ErrorCode = "invalid_request"
	// CodeInvalidState はstateの形または値が不正。
	CodeInvalidState ErrorCode = "invalid_state"
	// CodeInvalidCommand はcommandの形またはtypeが不正。
	CodeInvalidCommand ErrorCode = "invalid_command"
	// CodeInvalidCoordinate はrow/colが0〜7の外。
	CodeInvalidCoordinate ErrorCode = "invalid_coordinate"
	// CodeOccupied は対象マスが埋まっている。
	CodeOccupied ErrorCode = "occupied"
	// CodeIllegalMove は1方向も挟みが成立しない。
	CodeIllegalMove ErrorCode = "illegal_move"
	// CodeNotYourTurn はplayerが現在手番と違う。
	CodeNotYourTurn ErrorCode = "not_your_turn"
	// CodeStaleTurn はexpectedTurnが現在値と違う。
	CodeStaleTurn ErrorCode = "stale_turn"
	// CodePassNotAllowed は合法手が存在するのにpassした。
	CodePassNotAllowed ErrorCode = "pass_not_allowed"
	// CodeGameFinished は終了後のcommand。
	CodeGameFinished ErrorCode = "game_finished"
)

// errorCodes は契約が定義する全codeを宣言順に保持する。
var errorCodes = []ErrorCode{
	CodeInvalidJSON,
	CodeUnsupportedVersion,
	CodeInvalidRequest,
	CodeInvalidState,
	CodeInvalidCommand,
	CodeInvalidCoordinate,
	CodeOccupied,
	CodeIllegalMove,
	CodeNotYourTurn,
	CodeStaleTurn,
	CodePassNotAllowed,
	CodeGameFinished,
}

// ErrorCodes は契約が定義する全codeの複製を返す。
func ErrorCodes() []ErrorCode {
	return append([]ErrorCode(nil), errorCodes...)
}

// Valid はcodeが契約の列挙値かどうかを返す。
func (c ErrorCode) Valid() bool {
	for _, known := range errorCodes {
		if c == known {
			return true
		}
	}
	return false
}

// Error は拒否されたrequestの理由を表す。messageは表示可能な日本語。
type Error struct {
	Code    ErrorCode `json:"code"`
	Message string    `json:"message"`
}

// Error はGoのerror interfaceを満たす。
func (e *Error) Error() string {
	if e == nil {
		return "<nil protocol error>"
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

// NewError はcodeとmessageからErrorを組み立てる。
func NewError(code ErrorCode, message string) *Error {
	return &Error{Code: code, Message: message}
}
