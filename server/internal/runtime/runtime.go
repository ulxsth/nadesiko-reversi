// Package runtime はGoからなでしこ製ルールengineを呼ぶ境界を提供する。
// gonakoの実行ファイル、ルールsource、stdin/stdoutの扱いはこのpackageへ閉じる。
package runtime

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"time"

	"github.com/ulxsth/nadesiko-reversi/server/internal/protocol"
)

// DefaultTimeout はConfig.Timeoutが未指定のときに使う制限時間。
const DefaultTimeout = 5 * time.Second

// Runner はルール評価の境界。handlerとmatch serviceはこのinterfaceだけに依存する。
// 実装はrequestをそのまま評価し、ゲーム上の拒否はerrorではなくResponse.OK=falseで返す。
type Runner interface {
	Evaluate(ctx context.Context, request protocol.Request) (*protocol.Response, error)
}

// Config はgonako runnerの構成。
type Config struct {
	// GonakoPath はgonako実行ファイルのパス。
	GonakoPath string
	// RulesPath はルールengineのentry point（rules/game/main.nako3）。
	RulesPath string
	// Timeout は1回の評価に許す最大時間。0のときDefaultTimeoutを使う。
	Timeout time.Duration
}

// Gonako はgonako processとしてルールengineを実行するRunner実装。
// 保持するのは不変の構成だけなので、複数goroutineから同時に使える。
type Gonako struct {
	gonakoPath string
	rulesPath  string
	timeout    time.Duration
}

// Gonakoがinterfaceを満たすことをcompile時に確認する。
var _ Runner = (*Gonako)(nil)

// New はConfigを検証してgonako runnerを作る。
// 実行ファイルとルールsourceが存在しない場合はConfigErrorを返す。
func New(config Config) (*Gonako, error) {
	if config.GonakoPath == "" {
		return nil, &ConfigError{Field: "GonakoPath", Message: "gonako実行ファイルのパスが必要です"}
	}
	if config.RulesPath == "" {
		return nil, &ConfigError{Field: "RulesPath", Message: "ルールsourceのパスが必要です"}
	}
	if config.Timeout < 0 {
		return nil, &ConfigError{Field: "Timeout", Value: config.Timeout.String(), Message: "Timeoutは0以上である必要があります"}
	}

	if info, err := os.Stat(config.GonakoPath); err != nil {
		return nil, &ConfigError{Field: "GonakoPath", Value: config.GonakoPath, Message: "gonako実行ファイルが見つかりません"}
	} else if info.IsDir() {
		return nil, &ConfigError{Field: "GonakoPath", Value: config.GonakoPath, Message: "gonako実行ファイルがディレクトリです"}
	}

	if info, err := os.Stat(config.RulesPath); err != nil {
		return nil, &ConfigError{Field: "RulesPath", Value: config.RulesPath, Message: "ルールsourceが見つかりません"}
	} else if info.IsDir() {
		return nil, &ConfigError{Field: "RulesPath", Value: config.RulesPath, Message: "ルールsourceがディレクトリです"}
	}

	timeout := config.Timeout
	if timeout == 0 {
		timeout = DefaultTimeout
	}

	return &Gonako{
		gonakoPath: config.GonakoPath,
		rulesPath:  config.RulesPath,
		timeout:    timeout,
	}, nil
}

// Ready はルールengineを起動できる状態かどうかを返す。/healthzから利用する。
func (g *Gonako) Ready() bool {
	if _, err := os.Stat(g.gonakoPath); err != nil {
		return false
	}
	_, err := os.Stat(g.rulesPath)
	return err == nil
}

// Timeout は1回の評価に許す最大時間を返す。
func (g *Gonako) Timeout() time.Duration {
	return g.timeout
}

// Evaluate はrequestをルールengineへ渡し、responseを返す。
//
// requestが契約を満たさない場合はprocessを起動せず、契約上のerror codeを持つ
// 拒否responseを返す。ゲーム上の拒否も同じく拒否responseとして返り、errorはnilになる。
// errorが返るのはtimeout、異常終了、出力の解釈失敗といった実行基盤側の失敗だけ。
func (g *Gonako) Evaluate(ctx context.Context, request protocol.Request) (*protocol.Response, error) {
	if protocolErr := request.Validate(); protocolErr != nil {
		return rejected(protocolErr), nil
	}

	payload, err := json.Marshal(request)
	if err != nil {
		return nil, &DecodeError{Err: err}
	}

	stdout, stderr, err := g.run(ctx, payload)
	if err != nil {
		return nil, err
	}

	var response protocol.Response
	if err := json.Unmarshal(stdout, &response); err != nil {
		return nil, &DecodeError{Output: string(stdout), Err: err}
	}
	if protocolErr := response.Validate(); protocolErr != nil {
		return nil, &DecodeError{Output: string(stdout), Err: protocolErr}
	}

	// 契約を満たす応答が出ていれば、stderrの警告は無視してよい。
	_ = stderr

	return &response, nil
}

// run はgonako processを1回起動し、stdoutとstderrを返す。
// 一時ファイルを使わずpipeだけで完結するため、同時呼び出しが混線しない。
func (g *Gonako) run(ctx context.Context, payload []byte) ([]byte, []byte, error) {
	ctx, cancel := context.WithTimeout(ctx, g.timeout)
	defer cancel()

	var stdout, stderr bytes.Buffer

	cmd := exec.CommandContext(ctx, g.gonakoPath, g.rulesPath)
	cmd.Stdin = bytes.NewReader(payload)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	stderrText := truncate(stderr.String(), stderrLimit)

	if ctxErr := ctx.Err(); errors.Is(ctxErr, context.DeadlineExceeded) {
		return nil, nil, &TimeoutError{Timeout: g.timeout, Stderr: stderrText, Err: ctxErr}
	}
	if ctxErr := ctx.Err(); errors.Is(ctxErr, context.Canceled) {
		return nil, nil, ctxErr
	}

	if err != nil {
		execError := &ExecError{
			ExitCode: -1,
			Stderr:   stderrText,
			Stdout:   truncate(stdout.String(), stderrLimit),
			Err:      err,
		}
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			execError.ExitCode = exitErr.ExitCode()
		}
		return nil, nil, execError
	}

	return stdout.Bytes(), stderr.Bytes(), nil
}

// rejected は契約上の拒否responseを組み立てる。
func rejected(protocolErr *protocol.Error) *protocol.Response {
	return &protocol.Response{
		Version: protocol.Version,
		OK:      false,
		Error:   protocolErr,
	}
}

// NewGame は新規ゲームを作る。Runner実装を問わず使える糖衣。
func NewGame(ctx context.Context, runner Runner, gameID string, seed uint32) (*protocol.Response, error) {
	return runner.Evaluate(ctx, protocol.NewGameRequest(gameID, seed))
}

// ApplyCommand はstateへcommandを適用する。Runner実装を問わず使える糖衣。
func ApplyCommand(ctx context.Context, runner Runner, state protocol.State, command protocol.Command) (*protocol.Response, error) {
	return runner.Evaluate(ctx, protocol.ApplyCommandRequest(state, command))
}
