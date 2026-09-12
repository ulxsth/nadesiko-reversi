package runtime

import (
	"fmt"
	"strings"
	"time"

	"github.com/ulxsth/nadesiko-reversi/server/internal/protocol"
)

// stderrLimit は診断メッセージとして保持するstderrの上限バイト数。
// ルールエンジンの診断は1行なので、暴走時のメモリ増加だけを防ぐ。
const stderrLimit = 8 << 10

// ConfigError はrunnerの構成が不正であることを表す。
type ConfigError struct {
	Field   string
	Value   string
	Message string
}

func (e *ConfigError) Error() string {
	if e.Value == "" {
		return fmt.Sprintf("runtime設定が不正です (%s): %s", e.Field, e.Message)
	}
	return fmt.Sprintf("runtime設定が不正です (%s=%q): %s", e.Field, e.Value, e.Message)
}

// TimeoutError はルール評価が制限時間内に終わらなかったことを表す。
type TimeoutError struct {
	Timeout time.Duration
	Stderr  string
	Err     error
}

func (e *TimeoutError) Error() string {
	if e.Stderr == "" {
		return fmt.Sprintf("ルール評価が%sで打ち切られました", e.Timeout)
	}
	return fmt.Sprintf("ルール評価が%sで打ち切られました: %s", e.Timeout, e.Stderr)
}

func (e *TimeoutError) Unwrap() error { return e.Err }

// ExecError はルールengine processが異常終了したことを表す。
// 壊れたJSONを渡した場合、gonakoは非0終了とstderr診断を返す。
type ExecError struct {
	ExitCode int
	Stderr   string
	Stdout   string
	Err      error
}

func (e *ExecError) Error() string {
	detail := e.Stderr
	if detail == "" {
		detail = e.Stdout
	}
	if detail == "" {
		return fmt.Sprintf("ルールengineが異常終了しました (exit=%d)", e.ExitCode)
	}
	return fmt.Sprintf("ルールengineが異常終了しました (exit=%d): %s", e.ExitCode, detail)
}

func (e *ExecError) Unwrap() error { return e.Err }

// ProtocolError はExecErrorを契約上のerror codeへ写す。
// ルールengineがresponseを作れなかった場合はinvalid_jsonとして扱う。
func (e *ExecError) ProtocolError() *protocol.Error {
	return protocol.NewError(protocol.CodeInvalidJSON, "ルールengineが要求を解析できませんでした")
}

// DecodeError はルールengineの出力が契約のresponseでないことを表す。
type DecodeError struct {
	Output string
	Err    error
}

func (e *DecodeError) Error() string {
	return fmt.Sprintf("ルールengineの出力を解釈できません: %v (出力: %s)", e.Err, truncate(e.Output, 512))
}

func (e *DecodeError) Unwrap() error { return e.Err }

// truncate は診断用に文字列を指定バイト数まで切り詰める。
func truncate(value string, limit int) string {
	value = strings.TrimSpace(value)
	if len(value) <= limit {
		return value
	}
	return value[:limit] + "…"
}
