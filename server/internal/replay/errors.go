package replay

import (
	"fmt"

	"github.com/ulxsth/nadesiko-reversi/server/internal/protocol"
)

// RecordErrorCode は棋譜の再生に固有のerror code。
// 値はdocs/contracts/game-record.mdの表と一致する。
type RecordErrorCode string

const (
	// CodeRecordSyntaxError は文法に合わない行がある。
	CodeRecordSyntaxError RecordErrorCode = "record_syntax_error"
	// CodeRecordMissingHeader は開始行が無い、または2回以上ある。
	CodeRecordMissingHeader RecordErrorCode = "record_missing_header"
	// CodeRecordIllegalMove は再生中にcommandがルールへ拒否された。
	CodeRecordIllegalMove RecordErrorCode = "record_illegal_move"
	// CodeRecordMismatch は厳密照合で注釈の導出値が再生結果と一致しない。
	CodeRecordMismatch RecordErrorCode = "record_mismatch"
	// CodeRecordUnfinished は完了記録として読んだが終了行が無い。
	CodeRecordUnfinished RecordErrorCode = "record_unfinished"
)

// recordErrorCodes は契約が定義する全codeを宣言順に保持する。
var recordErrorCodes = []RecordErrorCode{
	CodeRecordSyntaxError,
	CodeRecordMissingHeader,
	CodeRecordIllegalMove,
	CodeRecordMismatch,
	CodeRecordUnfinished,
}

// RecordErrorCodes は契約が定義する全codeの複製を返す。
func RecordErrorCodes() []RecordErrorCode {
	return append([]RecordErrorCode(nil), recordErrorCodes...)
}

// Valid はcodeが契約の列挙値かどうかを返す。
func (c RecordErrorCode) Valid() bool {
	for _, known := range recordErrorCodes {
		if c == known {
			return true
		}
	}
	return false
}

// SourceError は棋譜のどこが受け入れられないかを、行番号とその行のソースを
// 添えて示す。改ざんされた棋譜も壊れた棋譜も同じ形で拒否できる。
type SourceError struct {
	// Code は契約のrecord error code。
	Code RecordErrorCode
	// Line は棋譜ソースの行番号。1始まり。特定できない場合は0。
	Line int
	// Source はその行のソース。特定できない場合は空。
	Source string
	// MoveNumber は何手目で失敗したか。1始まり。指し手に紐付かない場合は0。
	MoveNumber int
	// RuleCode はルール側が返したcode。record_illegal_moveのときに入る。
	RuleCode protocol.ErrorCode
	// Message は表示可能な日本語。
	Message string
	// Err は元のerror。
	Err error
}

func (e *SourceError) Error() string {
	location := "棋譜"
	switch {
	case e.Line > 0 && e.MoveNumber > 0:
		location = fmt.Sprintf("棋譜%d行目(%d手目)", e.Line, e.MoveNumber)
	case e.Line > 0:
		location = fmt.Sprintf("棋譜%d行目", e.Line)
	case e.MoveNumber > 0:
		location = fmt.Sprintf("棋譜%d手目", e.MoveNumber)
	}

	detail := e.Message
	if e.RuleCode != "" {
		detail = fmt.Sprintf("%s (%s)", detail, e.RuleCode)
	}
	if e.Source != "" {
		return fmt.Sprintf("%s: %s [%s] | %s", location, detail, e.Code, e.Source)
	}
	return fmt.Sprintf("%s: %s [%s]", location, detail, e.Code)
}

func (e *SourceError) Unwrap() error { return e.Err }

// NotFoundError は指定された対局が保存されていないことを表す。
type NotFoundError struct {
	GameID string
}

func (e *NotFoundError) Error() string {
	if e.GameID == "" {
		return "棋譜が1件も保存されていません"
	}
	return fmt.Sprintf("棋譜が見つかりません: %s", e.GameID)
}
