package replay

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// DefaultScriptTimeout は棋譜1本の実行に許す既定の時間。
const DefaultScriptTimeout = 10 * time.Second

// stderrLimit は診断として保持するstderrの上限バイト数。
const stderrLimit = 8 << 10

// ScriptRunner は棋譜ソースを実行して標準出力を返す。
// 再生の入口をこのinterfaceへ閉じ、testでは実processなしで差し替えられる。
type ScriptRunner interface {
	Run(ctx context.Context, source string) ([]byte, error)
}

// ScriptExecError は棋譜の実行が失敗したことを表す。
type ScriptExecError struct {
	ExitCode int
	Stderr   string
	Stdout   string
	Err      error
}

func (e *ScriptExecError) Error() string {
	detail := e.Stderr
	if detail == "" {
		detail = e.Stdout
	}
	if detail == "" {
		return fmt.Sprintf("棋譜の実行が失敗しました (exit=%d)", e.ExitCode)
	}
	return fmt.Sprintf("棋譜の実行が失敗しました (exit=%d): %s", e.ExitCode, detail)
}

func (e *ScriptExecError) Unwrap() error { return e.Err }

// asScriptExecError はerrorからScriptExecErrorを取り出す。
func asScriptExecError(err error, target **ScriptExecError) bool {
	return errors.As(err, target)
}

// ScriptTimeoutError は棋譜の実行が時間内に終わらなかったことを表す。
type ScriptTimeoutError struct {
	Timeout time.Duration
	Err     error
}

func (e *ScriptTimeoutError) Error() string {
	return fmt.Sprintf("棋譜の実行が%sで打ち切られました", e.Timeout)
}

func (e *ScriptTimeoutError) Unwrap() error { return e.Err }

// GonakoScriptRunner はgonakoで棋譜を実行する。
// 保持するのは不変の構成だけなので、複数goroutineから同時に使える。
type GonakoScriptRunner struct {
	gonakoPath string
	timeout    time.Duration
}

var _ ScriptRunner = (*GonakoScriptRunner)(nil)

// NewGonakoScriptRunner はgonakoの実行ファイルを検証してrunnerを作る。
func NewGonakoScriptRunner(gonakoPath string, timeout time.Duration) (*GonakoScriptRunner, error) {
	if gonakoPath == "" {
		return nil, fmt.Errorf("gonako実行ファイルのパスが必要です")
	}
	if timeout < 0 {
		return nil, fmt.Errorf("timeoutは0以上である必要があります: %s", timeout)
	}
	info, err := os.Stat(gonakoPath)
	if err != nil {
		return nil, fmt.Errorf("gonako実行ファイルが見つかりません: %s", gonakoPath)
	}
	if info.IsDir() {
		return nil, fmt.Errorf("gonako実行ファイルがディレクトリです: %s", gonakoPath)
	}
	if timeout == 0 {
		timeout = DefaultScriptTimeout
	}
	return &GonakoScriptRunner{gonakoPath: gonakoPath, timeout: timeout}, nil
}

// Run は棋譜ソースを一時ファイルへ書き出してgonakoで実行する。
//
// gonakoはファイル引数で起動するため一時ファイルが要るが、呼び出しごとに
// 固有の名前を作るので同時実行が混線しない。
func (r *GonakoScriptRunner) Run(ctx context.Context, source string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()

	dir, err := os.MkdirTemp("", "nadesiko-replay-")
	if err != nil {
		return nil, fmt.Errorf("棋譜の一時領域を作れません: %w", err)
	}
	defer os.RemoveAll(dir)

	scriptPath := filepath.Join(dir, "record.nako3")
	if err := os.WriteFile(scriptPath, []byte(source), 0o600); err != nil {
		return nil, fmt.Errorf("棋譜を書き出せません: %w", err)
	}

	var stdout, stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, r.gonakoPath, scriptPath)
	cmd.Stdin = bytes.NewReader(nil)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	runErr := cmd.Run()
	stderrText := truncate(stderr.String(), stderrLimit)

	if ctxErr := ctx.Err(); errors.Is(ctxErr, context.DeadlineExceeded) {
		return nil, &ScriptTimeoutError{Timeout: r.timeout, Err: ctxErr}
	}
	if ctxErr := ctx.Err(); errors.Is(ctxErr, context.Canceled) {
		return nil, ctxErr
	}

	if runErr != nil {
		execError := &ScriptExecError{
			ExitCode: -1,
			Stderr:   stderrText,
			Stdout:   truncate(stdout.String(), stderrLimit),
			Err:      runErr,
		}
		var exitError *exec.ExitError
		if errors.As(runErr, &exitError) {
			execError.ExitCode = exitError.ExitCode()
		}
		return nil, execError
	}

	// 一時ファイル名が診断へ混ざらないよう、呼び出し側へは中身だけ返す。
	return stdout.Bytes(), nil
}

// truncate は診断用に文字列を指定バイト数まで切り詰める。
func truncate(value string, limit int) string {
	value = strings.TrimSpace(value)
	if len(value) <= limit {
		return value
	}
	return value[:limit] + "…"
}
