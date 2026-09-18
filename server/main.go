package main

import (
	"bufio"
	"context"
	"crypto/sha1"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"math"
	"math/rand"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
	"unicode/utf8"

	"github.com/ulxsth/nadesiko-reversi/server/internal/localgame"
	"github.com/ulxsth/nadesiko-reversi/server/internal/match"
	"github.com/ulxsth/nadesiko-reversi/server/internal/protocol"
	"github.com/ulxsth/nadesiko-reversi/server/internal/replay"
	gameruntime "github.com/ulxsth/nadesiko-reversi/server/internal/runtime"
)

type healthResponse struct {
	Status      string `json:"status"`
	GoReady     bool   `json:"goReady"`
	GonakoReady bool   `json:"gonakoReady"`
}

// 公開デモの既定値。ローカル開発の挙動を変えない値を選ぶ。
const (
	defaultAddr          = "127.0.0.1:4173"
	defaultMaxConns      = 20
	defaultMaxRooms      = 10
	defaultCommandRate   = 2
	defaultCommandBurst  = 8
	defaultIdleTimeout   = 10 * time.Minute
	defaultConnectionAge = 55 * time.Minute
	wsPingInterval       = 30 * time.Second
)

// WebSocketの終了コード。4000番台はapplication用の私的範囲。
const (
	closeGoingAway = 1001
	closeProtocol  = 1002
	closeTooLarge  = 1009
	closeTryLater  = 1013
	closeIdle      = 4001
	closeMaxAge    = 4002
)

// demoConfig は公開デモとして動かすときの待ち受け、受け入れ範囲、上限。
// 値は環境変数から読み、未指定なら開発用の既定値を使う。
type demoConfig struct {
	addr             string
	webDir           string
	rulesDir         string
	publicOrigin     string
	maxConnections   int
	maxRooms         int
	commandRate      float64
	commandBurst     int
	idleTimeout      time.Duration
	maxConnectionAge time.Duration
	pingInterval     time.Duration
	readTimeout      time.Duration
}

type demoApp struct {
	config   demoConfig
	rules    *gameruntime.Gonako
	matches  *match.Manager
	replayer *replay.Replayer
	replays  *replay.Service
	records  *recordLedger

	// rulesReady は同梱ルールを使えるかどうか。testで差し替える。
	rulesReady func() bool
	// rulesVerified は起動時のルール実行確認の結果。
	rulesVerified atomic.Bool
	// connections は現在のWebSocket接続数。
	connections atomic.Int64
}

type acceptedMove struct {
	command protocol.Command
	event   protocol.Event
}

type roomRecord struct {
	mu        sync.Mutex
	seed      uint32
	startedAt string
	moves     []acceptedMove
	ready     chan struct{}
	finished  bool
	saveError error
}

type recordLedger struct {
	mu    sync.Mutex
	rooms map[string]*roomRecord
}

func newRecordLedger() *recordLedger {
	return &recordLedger{rooms: make(map[string]*roomRecord)}
}

func (l *recordLedger) seedFor(roomID string) uint32 {
	seed := rand.Uint32()
	l.mu.Lock()
	l.rooms[roomID] = &roomRecord{seed: seed, startedAt: time.Now().UTC().Format(time.RFC3339), ready: make(chan struct{})}
	l.mu.Unlock()
	return seed
}

func (l *recordLedger) get(roomID string) *roomRecord {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.rooms[roomID]
}

func (l *recordLedger) discard(roomID string) {
	l.mu.Lock()
	delete(l.rooms, roomID)
	l.mu.Unlock()
}

// ===== 公開デモの構成 =====

// loadConfig はflagと環境変数から構成を組み立てる。
// 不正な値はここで止める。公開してから気づく事態を避けるため、既定値へ落とさない。
func loadConfig(getenv func(string) string, addr string, addrSet bool, webDir, rulesDir string) (demoConfig, error) {
	listen, err := resolveAddr(addr, addrSet, getenv("PORT"))
	if err != nil {
		return demoConfig{}, err
	}
	publicOrigin, err := parsePublicOrigin(getenv("PUBLIC_ORIGIN"))
	if err != nil {
		return demoConfig{}, err
	}
	maxConnections, err := envInt(getenv, "MAX_CONNECTIONS", defaultMaxConns, 1, 10000)
	if err != nil {
		return demoConfig{}, err
	}
	maxRooms, err := envInt(getenv, "MAX_ROOMS", defaultMaxRooms, 1, 10000)
	if err != nil {
		return demoConfig{}, err
	}
	commandRate, err := envInt(getenv, "COMMAND_RATE_PER_SEC", defaultCommandRate, 1, 1000)
	if err != nil {
		return demoConfig{}, err
	}
	commandBurst, err := envInt(getenv, "COMMAND_BURST", defaultCommandBurst, 1, 1000)
	if err != nil {
		return demoConfig{}, err
	}
	idleTimeout, err := envSeconds(getenv, "IDLE_TIMEOUT_SECONDS", defaultIdleTimeout, time.Minute, 24*time.Hour)
	if err != nil {
		return demoConfig{}, err
	}
	maxAge, err := envSeconds(getenv, "MAX_CONNECTION_SECONDS", defaultConnectionAge, time.Minute, 24*time.Hour)
	if err != nil {
		return demoConfig{}, err
	}

	return demoConfig{
		addr:             listen,
		webDir:           webDir,
		rulesDir:         rulesDir,
		publicOrigin:     publicOrigin,
		maxConnections:   maxConnections,
		maxRooms:         maxRooms,
		commandRate:      float64(commandRate),
		commandBurst:     commandBurst,
		idleTimeout:      idleTimeout,
		maxConnectionAge: maxAge,
		pingInterval:     wsPingInterval,
		// pingへのpongが2回続けて落ちるまでは待つ。
		readTimeout: 2*wsPingInterval + 15*time.Second,
	}, nil
}

// resolveAddr は待ち受けアドレスを決める。
// -addrの明示指定が最優先、次にコンテナが渡すPORT、最後に開発用の既定値を使う。
func resolveAddr(addr string, addrSet bool, port string) (string, error) {
	if addrSet {
		if strings.TrimSpace(addr) == "" {
			return "", fmt.Errorf("-addrが空です")
		}
		return addr, nil
	}
	port = strings.TrimSpace(port)
	if port == "" {
		return addr, nil
	}
	number, err := strconv.Atoi(port)
	if err != nil || number < 1 || number > 65535 {
		return "", fmt.Errorf("PORTは1〜65535の整数である必要があります: %q", port)
	}
	return net.JoinHostPort("0.0.0.0", strconv.Itoa(number)), nil
}

// parsePublicOrigin は公開originを正規化する。
//
// 受け付けるのは`https://host[:port]`だけ。例外として、コンテナをローカルで
// 確認するときのために、ループバックhostに限り`http://`も許す。
// path・query・userinfoを持つ値は拒否する。
func parsePublicOrigin(raw string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", nil
	}
	parsed, err := url.Parse(trimmed)
	if err != nil {
		return "", fmt.Errorf("PUBLIC_ORIGINを解析できません: %v", err)
	}
	scheme := strings.ToLower(parsed.Scheme)
	if scheme != "https" && !(scheme == "http" && loopbackHost(parsed.Host)) {
		return "", fmt.Errorf("PUBLIC_ORIGINはhttpsで指定してください（httpはループバックhostのみ）: %q", raw)
	}
	if parsed.Host == "" || parsed.Opaque != "" || parsed.User != nil ||
		(parsed.Path != "" && parsed.Path != "/") || parsed.RawQuery != "" || parsed.Fragment != "" {
		return "", fmt.Errorf("PUBLIC_ORIGINはscheme+hostだけで指定してください: %q", raw)
	}
	return scheme + "://" + strings.ToLower(parsed.Host), nil
}

// loopbackHost はhostが同じ端末を指すかどうかを返す。
func loopbackHost(host string) bool {
	name := host
	if withoutPort, _, err := net.SplitHostPort(host); err == nil {
		name = withoutPort
	}
	if strings.EqualFold(name, "localhost") {
		return true
	}
	ip := net.ParseIP(strings.Trim(name, "[]"))
	return ip != nil && ip.IsLoopback()
}

// envInt は整数の環境変数を範囲つきで読む。
func envInt(getenv func(string) string, name string, fallback, min, max int) (int, error) {
	raw := strings.TrimSpace(getenv(name))
	if raw == "" {
		return fallback, nil
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value < min || value > max {
		return 0, fmt.Errorf("%sは%d〜%dの整数である必要があります: %q", name, min, max, raw)
	}
	return value, nil
}

// envSeconds は秒数の環境変数を範囲つきで読む。
func envSeconds(getenv func(string) string, name string, fallback, min, max time.Duration) (time.Duration, error) {
	seconds, err := envInt(getenv, name, int(fallback/time.Second), int(min/time.Second), int(max/time.Second))
	if err != nil {
		return 0, err
	}
	return time.Duration(seconds) * time.Second, nil
}

// publicOriginMatches はOriginヘッダーが公開originと完全一致するかを返す。
// Hostや転送ヘッダーは前段が書き換えられるので、受け入れ判断には使わない。
func publicOriginMatches(publicOrigin, header string) bool {
	if publicOrigin == "" {
		return false
	}
	trimmed := strings.TrimSpace(header)
	if trimmed == "" {
		return false
	}
	parsed, err := url.Parse(trimmed)
	if err != nil {
		return false
	}
	// schemeの許可範囲はPUBLIC_ORIGINの検証で決まっている。ここは字面の一致だけを見る。
	scheme := strings.ToLower(parsed.Scheme)
	if scheme != "https" && scheme != "http" {
		return false
	}
	if parsed.Host == "" || parsed.Opaque != "" || parsed.User != nil ||
		parsed.Path != "" || parsed.RawQuery != "" || parsed.Fragment != "" {
		return false
	}
	return scheme+"://"+strings.ToLower(parsed.Host) == publicOrigin
}

// rateLimiter は1接続あたりのcommand頻度を抑えるtoken bucket。
// 呼び出しは1接続の読み取りloopからだけなので、lockを持たない。
type rateLimiter struct {
	rate     float64
	burst    float64
	tokens   float64
	lastFill time.Time
}

func newRateLimiter(rate float64, burst int, now time.Time) *rateLimiter {
	return &rateLimiter{rate: rate, burst: float64(burst), tokens: float64(burst), lastFill: now}
}

// allow は1件分のtokenを消費できたかどうかを返す。
func (l *rateLimiter) allow(now time.Time) bool {
	if elapsed := now.Sub(l.lastFill).Seconds(); elapsed > 0 {
		l.tokens = math.Min(l.burst, l.tokens+elapsed*l.rate)
		l.lastFill = now
	}
	if l.tokens < 1 {
		return false
	}
	l.tokens--
	return true
}

// admit は同時接続の上限内なら接続を数え、解放関数を返す。
func (a *demoApp) admit() (func(), bool) {
	if a.connections.Add(1) > int64(a.config.maxConnections) {
		a.connections.Add(-1)
		return func() {}, false
	}
	var once sync.Once
	return func() { once.Do(func() { a.connections.Add(-1) }) }, true
}

// gonakoReady は同梱ルールを実際に使えるかどうかを返す。
func (a *demoApp) gonakoReady() bool {
	if !a.rulesVerified.Load() {
		return false
	}
	return a.rulesReady == nil || a.rulesReady()
}

// handleHealth はGo側とgonako側のready状態を返す。
// 未readyのときは成功以外のstatusにして、健全性確認から失敗と分かるようにする。
func (a *demoApp) handleHealth(w http.ResponseWriter, _ *http.Request) {
	ready := a.gonakoReady()
	status, label := http.StatusOK, "ok"
	if !ready {
		status, label = http.StatusServiceUnavailable, "degraded"
	}
	writeJSON(w, status, healthResponse{Status: label, GoReady: true, GonakoReady: ready})
}

// verifyRules は同梱ルールを1回実行して、実際に動くことを確かめる。
func verifyRules(ctx context.Context, rules *gameruntime.Gonako) error {
	response, err := rules.Evaluate(ctx, protocol.NewGameRequest("startup-check", 1))
	if err != nil {
		return err
	}
	if !response.OK || response.State == nil {
		return fmt.Errorf("ルールの起動確認が拒否されました")
	}
	return nil
}

func main() {
	addr := flag.String("addr", defaultAddr, "listen address")
	webDir := flag.String("web", "web", "web root")
	rulesDir := flag.String("rules", "rules", "rule source directory")
	flag.Parse()
	addrSet := false
	flag.Visit(func(f *flag.Flag) {
		if f.Name == "addr" {
			addrSet = true
		}
	})

	config, err := loadConfig(os.Getenv, *addr, addrSet, *webDir, *rulesDir)
	if err != nil {
		log.Fatalf("構成が不正です: %v", err)
	}

	gonakoBin := os.Getenv("GONAKO_BIN")
	if gonakoBin == "" {
		gonakoBin = filepath.Join(".tools", "bin", "gonako")
	}
	ruleFile := filepath.Join(*rulesDir, "game", "main.nako3")
	rules, err := gameruntime.New(gameruntime.Config{
		GonakoPath: gonakoBin,
		RulesPath:  ruleFile,
		Timeout:    5 * time.Second,
	})
	if err != nil {
		log.Fatalf("ルールruntimeを初期化できません: %v", err)
	}
	game := localgame.NewService(rules)
	records := newRecordLedger()
	matches, err := match.NewManager(rules, match.WithSeedFunc(records.seedFor))
	if err != nil {
		log.Fatalf("対戦セッションを初期化できません: %v", err)
	}
	defer matches.Close("サーバーを終了します")

	harnessFile := filepath.Join(*rulesDir, "replay", "harness.nako3")
	harness, err := replay.LoadHarness(harnessFile)
	if err != nil {
		log.Fatalf("棋譜ハーネスを読み込めません: %v", err)
	}
	scriptRunner, err := replay.NewGonakoScriptRunner(gonakoBin, replay.DefaultScriptTimeout)
	if err != nil {
		log.Fatalf("棋譜runtimeを初期化できません: %v", err)
	}
	decoder, err := replay.NewDecoder(harness, scriptRunner)
	if err != nil {
		log.Fatalf("棋譜decoderを初期化できません: %v", err)
	}
	replayer, err := replay.NewReplayer(decoder)
	if err != nil {
		log.Fatalf("棋譜再生を初期化できません: %v", err)
	}
	replays, err := replay.NewService(replay.NewMemoryStore(), replayer)
	if err != nil {
		log.Fatalf("棋譜保管庫を初期化できません: %v", err)
	}
	app := &demoApp{
		config:     config,
		rules:      rules,
		matches:    matches,
		replayer:   replayer,
		replays:    replays,
		records:    records,
		rulesReady: rules.Ready,
	}

	// 同梱物が揃っているだけでなく、実際にルールを実行できることを起動時に確かめる。
	// 失敗しても起動は続け、/healthzから未readyを観測できるようにする。
	verifyCtx, cancelVerify := context.WithTimeout(context.Background(), 30*time.Second)
	if err := verifyRules(verifyCtx, rules); err != nil {
		log.Printf("ルールの起動確認に失敗しました: %v", err)
	} else {
		app.rulesVerified.Store(true)
	}
	cancelVerify()

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", app.handleHealth)
	mux.Handle("POST /api/local/runtime", localgame.NewHandler(game))
	mux.HandleFunc("GET /api/match", app.handleMatch)
	mux.HandleFunc("GET /api/replays/{gameId}", app.handleReplay)
	mux.Handle("/", http.FileServer(http.Dir(config.webDir)))

	server := &http.Server{
		Addr:              config.addr,
		Handler:           withHeaders(mux),
		ReadHeaderTimeout: 5 * time.Second,
	}

	// 停止signalを受けたら、まず対局中の全席へ理由を配信してからHTTPを閉じる。
	// これでインスタンス停止時に、クライアントは理由なしの切断ではなく再接続の案内を受け取れる。
	signalCtx, stopSignals := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stopSignals()
	go func() {
		<-signalCtx.Done()
		log.Printf("停止signalを受け取りました。接続を閉じます")
		matches.Close("サーバーを再起動します。少し待ってから再接続してください")
		shutdownCtx, cancelShutdown := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancelShutdown()
		if err := server.Shutdown(shutdownCtx); err != nil {
			log.Printf("停止処理でエラーが発生しました: %v", err)
		}
	}()

	log.Printf("なでしこ・リバーシサーバー: http://%s", config.addr)
	if config.publicOrigin == "" {
		log.Printf("公開originが未設定です。WebSocketはローカルの同一originだけを受け付けます")
	} else {
		log.Printf("公開origin: %s", config.publicOrigin)
	}
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatalf("サーバーを起動できません: %v", err)
	}
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(value); err != nil {
		log.Printf("JSON応答エラー: %v", err)
	}
}

func withHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")
		next.ServeHTTP(w, r)
	})
}

const wsGUID = "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"
const maxWSMessage = 64 << 10

var errWSProtocol = errors.New("WebSocketの形式が不正です")
var errWSMessageTooLarge = errors.New("WebSocketのメッセージが大きすぎます")

type wsFrame struct {
	opcode byte
	data   []byte
}

type wsSocket struct {
	conn   net.Conn
	reader *bufio.Reader
	writer *bufio.Writer
}

func headerHasToken(value, token string) bool {
	for _, item := range strings.Split(value, ",") {
		if strings.EqualFold(strings.TrimSpace(item), token) {
			return true
		}
	}
	return false
}

func localWebSocketOrigin(r *http.Request) bool {
	remoteHost, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil || !net.ParseIP(remoteHost).IsLoopback() {
		return false
	}
	requestHost := r.Host
	if host, _, splitErr := net.SplitHostPort(requestHost); splitErr == nil {
		requestHost = host
	}
	if !strings.EqualFold(requestHost, "localhost") {
		ip := net.ParseIP(requestHost)
		if ip == nil || !ip.IsLoopback() {
			return false
		}
	}
	origin, err := url.Parse(r.Header.Get("Origin"))
	if err != nil || (origin.Scheme != "http" && origin.Scheme != "https") {
		return false
	}
	return strings.EqualFold(origin.Host, r.Host) && origin.Path == "" && origin.RawQuery == ""
}

// allowWebSocketOrigin は接続元を受け入れるかどうかを返す。
// ローカルのループバック同一originか、設定された公開originとの完全一致だけを許す。
func (a *demoApp) allowWebSocketOrigin(r *http.Request) bool {
	if localWebSocketOrigin(r) {
		return true
	}
	return publicOriginMatches(a.config.publicOrigin, r.Header.Get("Origin"))
}

func (a *demoApp) upgradeWebSocket(w http.ResponseWriter, r *http.Request) (*wsSocket, error) {
	if !a.allowWebSocketOrigin(r) {
		http.Error(w, "許可されていない接続元です", http.StatusForbidden)
		return nil, errWSProtocol
	}
	if !headerHasToken(r.Header.Get("Connection"), "Upgrade") ||
		!headerHasToken(r.Header.Get("Upgrade"), "websocket") ||
		r.Header.Get("Sec-WebSocket-Version") != "13" {
		http.Error(w, "WebSocketの接続条件が不正です", http.StatusBadRequest)
		return nil, errWSProtocol
	}
	key := r.Header.Get("Sec-WebSocket-Key")
	decoded, err := base64.StdEncoding.DecodeString(key)
	if err != nil || len(decoded) != 16 {
		http.Error(w, "WebSocketの鍵が不正です", http.StatusBadRequest)
		return nil, errWSProtocol
	}
	hijacker, ok := w.(http.Hijacker)
	if !ok {
		http.Error(w, "WebSocketを開始できません", http.StatusInternalServerError)
		return nil, errWSProtocol
	}
	conn, buffer, err := hijacker.Hijack()
	if err != nil {
		return nil, err
	}
	accept := sha1.Sum([]byte(key + wsGUID))
	_, err = buffer.WriteString("HTTP/1.1 101 Switching Protocols\r\n" +
		"Upgrade: websocket\r\n" +
		"Connection: Upgrade\r\n" +
		"Sec-WebSocket-Accept: " + base64.StdEncoding.EncodeToString(accept[:]) + "\r\n\r\n")
	if err == nil {
		err = buffer.Flush()
	}
	if err != nil {
		conn.Close()
		return nil, err
	}
	return &wsSocket{conn: conn, reader: buffer.Reader, writer: buffer.Writer}, nil
}

func (s *wsSocket) readFrame() (wsFrame, error) {
	var header [2]byte
	if _, err := io.ReadFull(s.reader, header[:]); err != nil {
		return wsFrame{}, err
	}
	opcode := header[0] & 0x0f
	fin := header[0]&0x80 != 0
	if !fin || header[0]&0x70 != 0 || header[1]&0x80 == 0 {
		return wsFrame{}, errWSProtocol
	}
	if opcode != 1 && opcode != 8 && opcode != 9 && opcode != 10 {
		return wsFrame{}, errWSProtocol
	}
	length := uint64(header[1] & 0x7f)
	if length == 126 {
		var extended [2]byte
		if _, err := io.ReadFull(s.reader, extended[:]); err != nil {
			return wsFrame{}, err
		}
		length = uint64(binary.BigEndian.Uint16(extended[:]))
		if length < 126 {
			return wsFrame{}, errWSProtocol
		}
	} else if length == 127 {
		var extended [8]byte
		if _, err := io.ReadFull(s.reader, extended[:]); err != nil {
			return wsFrame{}, err
		}
		length = binary.BigEndian.Uint64(extended[:])
		if length < 65536 || length>>63 != 0 {
			return wsFrame{}, errWSProtocol
		}
	}
	if length > maxWSMessage {
		return wsFrame{}, errWSMessageTooLarge
	}
	if opcode >= 8 && length > 125 {
		return wsFrame{}, errWSProtocol
	}
	var mask [4]byte
	if _, err := io.ReadFull(s.reader, mask[:]); err != nil {
		return wsFrame{}, err
	}
	data := make([]byte, int(length))
	if _, err := io.ReadFull(s.reader, data); err != nil {
		return wsFrame{}, err
	}
	for i := range data {
		data[i] ^= mask[i%4]
	}
	if opcode == 1 && !utf8.Valid(data) || opcode == 8 && (len(data) == 1 || len(data) >= 2 && !utf8.Valid(data[2:])) {
		return wsFrame{}, errWSProtocol
	}
	return wsFrame{opcode: opcode, data: data}, nil
}

func (s *wsSocket) writeFrame(frame wsFrame) error {
	if len(frame.data) > maxWSMessage {
		return errWSMessageTooLarge
	}
	if err := s.writer.WriteByte(0x80 | frame.opcode); err != nil {
		return err
	}
	length := len(frame.data)
	if length < 126 {
		if err := s.writer.WriteByte(byte(length)); err != nil {
			return err
		}
	} else {
		var extended [3]byte
		extended[0] = 126
		binary.BigEndian.PutUint16(extended[1:], uint16(length))
		if _, err := s.writer.Write(extended[:]); err != nil {
			return err
		}
	}
	if _, err := s.writer.Write(frame.data); err != nil {
		return err
	}
	return s.writer.Flush()
}

type queuedFrame struct {
	frame wsFrame
	ack   chan struct{}
}

type wsPeer struct {
	socket   *wsSocket
	outgoing chan queuedFrame
	done     chan struct{}
	once     sync.Once
	// lastActive は最後にcommandを受けたかeventを送った時刻。アイドル上限はここから計る。
	lastActive atomic.Int64
}

func newWSPeer(socket *wsSocket) *wsPeer {
	peer := &wsPeer{socket: socket, outgoing: make(chan queuedFrame, 32), done: make(chan struct{})}
	peer.touch()
	go peer.writeLoop()
	return peer
}

// touch は対局が動いた時刻を記録する。
func (p *wsPeer) touch() {
	p.lastActive.Store(time.Now().UnixNano())
}

// keepAlive はpingで接続を維持し、アイドル上限と接続時間上限に達したら理由つきで閉じる。
//
// ブラウザはping frameへ自動でpongを返すので、操作がなくても接続は切れない。
// 代わりに、両者が何もしない時間と1接続の総時間へ明示的な上限を置く。
func (p *wsPeer) keepAlive(config demoConfig) {
	ticker := time.NewTicker(config.pingInterval)
	defer ticker.Stop()
	deadline := time.Now().Add(config.maxConnectionAge)
	for {
		select {
		case <-p.done:
			return
		case now := <-ticker.C:
			if !now.Before(deadline) {
				p.shutdown(closeMaxAge, "接続時間の上限に達しました。再接続してください")
				return
			}
			if now.Sub(time.Unix(0, p.lastActive.Load())) >= config.idleTimeout {
				p.shutdown(closeIdle, "一定時間操作がなかったため切断しました。再接続してください")
				return
			}
			if !p.send(wsFrame{opcode: 9}) {
				return
			}
		}
	}
}

// shutdown は理由を伝えてから接続を閉じる。
func (p *wsPeer) shutdown(code uint16, reason string) {
	p.flush(wsCloseFrame(code, reason))
	p.close()
}

func (p *wsPeer) close() {
	p.once.Do(func() {
		close(p.done)
		p.socket.conn.Close()
	})
}

func (p *wsPeer) writeLoop() {
	for {
		select {
		case <-p.done:
			return
		case queued := <-p.outgoing:
			_ = p.socket.conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
			err := p.socket.writeFrame(queued.frame)
			if queued.ack != nil {
				close(queued.ack)
			}
			if err != nil {
				p.close()
				return
			}
		}
	}
}

func (p *wsPeer) send(frame wsFrame) bool {
	select {
	case p.outgoing <- queuedFrame{frame: frame}:
		return true
	case <-p.done:
		return false
	}
}

func (p *wsPeer) sendJSON(value any) bool {
	data, err := json.Marshal(value)
	if err != nil {
		log.Printf("WebSocket JSON生成エラー: %v", err)
		return false
	}
	return p.send(wsFrame{opcode: 1, data: data})
}

func (p *wsPeer) flush(frame wsFrame) {
	ack := make(chan struct{})
	select {
	case p.outgoing <- queuedFrame{frame: frame, ack: ack}:
		select {
		case <-ack:
		case <-p.done:
		}
	case <-p.done:
	}
}

// wsCloseFrame は終了コードと表示可能な理由を持つclose frameを作る。
func wsCloseFrame(code uint16, reason string) wsFrame {
	data := make([]byte, 2, 2+len(reason))
	binary.BigEndian.PutUint16(data, code)
	return wsFrame{opcode: 8, data: append(data, truncateCloseReason(reason)...)}
}

// truncateCloseReason は理由をclose frameのpayload上限（125byte）へ収める。
// 途中で切るとUTF-8として不正になるので、有効な境界まで戻す。
func truncateCloseReason(reason string) []byte {
	const limit = 123
	if len(reason) <= limit {
		return []byte(reason)
	}
	truncated := reason[:limit]
	for len(truncated) > 0 && !utf8.ValidString(truncated) {
		truncated = truncated[:len(truncated)-1]
	}
	return []byte(truncated)
}

// closeSocket は参加前に断る接続へ、理由を伝えてから閉じる。
func closeSocket(socket *wsSocket, code uint16, reason string) {
	_ = socket.conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
	_ = socket.writeFrame(wsCloseFrame(code, reason))
	_ = socket.conn.Close()
}

func (a *demoApp) handleMatch(w http.ResponseWriter, r *http.Request) {
	playerID := r.URL.Query().Get("playerId")
	if err := replay.ValidateGameID(playerID); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "参加者IDは半角英数字と_-の1〜64文字で指定してください"})
		return
	}
	socket, err := a.upgradeWebSocket(w, r)
	if err != nil {
		return
	}
	// 上限超過はupgradeしてから理由つきで閉じる。
	// ブラウザはupgrade前のHTTP statusを読めず、理由なしの1006になってしまうため。
	release, admitted := a.admit()
	if !admitted {
		closeSocket(socket, closeTryLater, "接続が混み合っています。しばらくしてから開き直してください")
		return
	}
	defer release()
	if a.matches.RoomCount() >= a.config.maxRooms {
		closeSocket(socket, closeTryLater, "対局数の上限に達しています。しばらくしてから開き直してください")
		return
	}
	membership, err := a.matches.Join(context.Background(), playerID)
	if err != nil {
		data, _ := json.Marshal(protocol.Response{Version: protocol.Version, OK: false,
			Error: protocol.NewError(protocol.CodeInvalidRequest, err.Error())})
		_ = socket.writeFrame(wsFrame{opcode: 1, data: data})
		_ = socket.conn.Close()
		return
	}
	peer := newWSPeer(socket)
	defer peer.close()
	defer func() {
		_ = a.matches.Leave(context.Background(), playerID)
		if _, exists := a.matches.Room(membership.RoomID); !exists {
			a.records.discard(membership.RoomID)
		}
	}()

	record := a.records.get(membership.RoomID)
	go a.forwardEvents(peer, membership.Events, record)
	go peer.keepAlive(a.config)
	limiter := newRateLimiter(a.config.commandRate, a.config.commandBurst, time.Now())
	for {
		_ = socket.conn.SetReadDeadline(time.Now().Add(a.config.readTimeout))
		frame, err := socket.readFrame()
		if err != nil {
			if errors.Is(err, errWSProtocol) {
				peer.flush(wsCloseFrame(closeProtocol, "WebSocketの形式が不正です"))
			} else if errors.Is(err, errWSMessageTooLarge) {
				peer.flush(wsCloseFrame(closeTooLarge, "メッセージが大きすぎます"))
			}
			return
		}
		switch frame.opcode {
		case 8:
			peer.flush(wsFrame{opcode: 8, data: frame.data})
			return
		case 9:
			peer.send(wsFrame{opcode: 10, data: frame.data})
		case 10:
			// pongは接続の生存確認にのみ使用する。
		case 1:
			if !limiter.allow(time.Now()) {
				peer.sendJSON(protocol.Response{Version: protocol.Version, OK: false,
					Error: protocol.NewError(protocol.CodeInvalidRequest, "操作が速すぎます。少し待ってからもう一度お試しください")})
				continue
			}
			peer.touch()
			var command protocol.Command
			if err := json.Unmarshal(frame.data, &command); err != nil {
				peer.sendJSON(protocol.Response{Version: protocol.Version, OK: false,
					Error: protocol.NewError(protocol.CodeInvalidJSON, "JSONを解析できません")})
				continue
			}
			response, err := a.submitCommand(membership.RoomID, playerID, record, command)
			if err != nil {
				log.Printf("対局 %s のcommand処理エラー: %v", membership.RoomID, err)
				peer.sendJSON(protocol.Response{Version: protocol.Version, OK: false,
					Error: protocol.NewError(protocol.CodeInvalidState, "対局を処理できませんでした")})
				continue
			}
			if !response.OK {
				peer.sendJSON(response)
			}
		}
	}
}

func (a *demoApp) forwardEvents(peer *wsPeer, events <-chan match.Event, record *roomRecord) {
	for {
		select {
		case <-peer.done:
			return
		case event, ok := <-events:
			if !ok {
				// roomが片付いた後に読み取りdeadlineまで黙って待たせない。
				// 直前までに積んだeventを書き切ってから、理由つきで閉じる。
				peer.shutdown(closeGoingAway, "サーバーが接続を終了しました。再接続してください")
				return
			}
			if record != nil && event.State != nil && event.State.Phase == protocol.PhaseFinished {
				select {
				case <-record.ready:
					if record.saveError != nil {
						peer.sendJSON(map[string]string{"type": "serverError", "reason": "棋譜を保存できませんでした"})
					}
				case <-peer.done:
					return
				}
			}
			if !peer.sendJSON(event) {
				return
			}
			peer.touch()
		}
	}
}

func (a *demoApp) submitCommand(roomID, playerID string, record *roomRecord, command protocol.Command) (*protocol.Response, error) {
	if record == nil {
		return nil, fmt.Errorf("roomの記録がありません")
	}
	record.mu.Lock()
	defer record.mu.Unlock()
	response, err := a.matches.Submit(context.Background(), roomID, playerID, command)
	if err != nil || response == nil || !response.OK {
		return response, err
	}
	if response.Event == nil || response.State == nil {
		return nil, fmt.Errorf("受理結果のeventまたはstateがありません")
	}
	record.moves = append(record.moves, acceptedMove{command: command, event: *response.Event})
	if response.State.Phase == protocol.PhaseFinished && !record.finished {
		record.finished = true
		record.saveError = a.saveFinished(roomID, record, *response.State)
		close(record.ready)
	}
	return response, nil
}

func (a *demoApp) saveFinished(roomID string, record *roomRecord, final protocol.State) error {
	// 引き分けの終局はwinnerがnilになるので、勝者の有無では弾かない
	lines := append(replay.HeaderLines(record.startedAt, time.Now().UTC().Format(time.RFC3339)), "")
	lines = append(lines, replay.RulesVersionLine())
	startLine, err := replay.StartLine(roomID, record.seed)
	if err != nil {
		return err
	}
	lines = append(lines, startLine, "")
	for _, move := range record.moves {
		var line string
		switch move.command.Type {
		case protocol.CommandPlace:
			if move.command.Row == nil || move.command.Col == nil || move.event.PlacedColor == nil {
				return fmt.Errorf("着手の座標か色がありません")
			}
			line, err = replay.PlaceLine(move.command.Player, *move.command.Row, *move.command.Col,
				*move.event.PlacedColor, len(move.event.Changes))
		case protocol.CommandPass:
			line, err = replay.PassLine(move.command.Player)
		default:
			return fmt.Errorf("棋譜に使えないcommandです")
		}
		if err != nil {
			return err
		}
		lines = append(lines, line)
	}
	colorSum, pieceCount := 0, 0
	for _, cell := range final.Board {
		if cell != nil {
			colorSum += int(*cell)
			pieceCount++
		}
	}
	// 引き分けの終局はwinnerがnilのまま。EndLineが「引分」を書く
	endLine, err := replay.EndLine(final.Winner, len(record.moves), colorSum, pieceCount)
	if err != nil {
		return err
	}
	lines = append(lines, "", endLine, "")
	source := strings.Join(lines, "\n")
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	result, err := a.replayer.Replay(ctx, source)
	if err != nil {
		return fmt.Errorf("棋譜の再生検証に失敗: %w", err)
	}
	if !reflect.DeepEqual(result.FinalState(), final) {
		return fmt.Errorf("再生した最終盤面が対局結果と一致しません")
	}
	_, err = a.replays.SaveResult(ctx, result)
	return err
}

type replayFrameResponse struct {
	MoveNumber int            `json:"moveNumber"`
	State      protocol.State `json:"state"`
}

func (a *demoApp) handleReplay(w http.ResponseWriter, r *http.Request) {
	gameID := r.PathValue("gameId")
	if err := replay.ValidateGameID(gameID); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "対局IDが不正です"})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 45*time.Second)
	defer cancel()
	source, err := a.replays.Source(ctx, gameID)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "完了した対局の棋譜がありません"})
		return
	}
	result, err := a.replays.Replay(ctx, gameID)
	if err != nil {
		log.Printf("棋譜 %s の再生エラー: %v", gameID, err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "棋譜を再生できませんでした"})
		return
	}
	frames := make([]replayFrameResponse, len(result.Frames))
	for i, frame := range result.Frames {
		frames[i] = replayFrameResponse{MoveNumber: frame.MoveNumber, State: frame.State}
	}
	writeJSON(w, http.StatusOK, struct {
		GameID string                `json:"gameId"`
		Source string                `json:"source"`
		Frames []replayFrameResponse `json:"frames"`
		// Winner は引き分けならnull
		Winner *protocol.Player `json:"winner"`
	}{GameID: gameID, Source: source, Frames: frames, Winner: result.Script.Winner})
}
