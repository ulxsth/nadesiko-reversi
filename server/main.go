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
	"math/rand"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
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

type demoApp struct {
	rules    *gameruntime.Gonako
	matches  *match.Manager
	replayer *replay.Replayer
	replays  *replay.Service
	records  *recordLedger
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

func main() {
	addr := flag.String("addr", "127.0.0.1:4173", "listen address")
	webDir := flag.String("web", "web", "web root")
	rulesDir := flag.String("rules", "rules", "rule source directory")
	flag.Parse()

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
	app := &demoApp{rules: rules, matches: matches, replayer: replayer, replays: replays, records: records}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, healthResponse{Status: "ok", GoReady: true, GonakoReady: app.rules.Ready()})
	})
	mux.Handle("POST /api/local/runtime", localgame.NewHandler(game))
	mux.HandleFunc("GET /api/match", app.handleMatch)
	mux.HandleFunc("GET /api/replays/{gameId}", app.handleReplay)
	mux.Handle("/", http.FileServer(http.Dir(*webDir)))

	server := &http.Server{
		Addr:              *addr,
		Handler:           withHeaders(mux),
		ReadHeaderTimeout: 5 * time.Second,
	}

	log.Printf("なでしこ・リバーシ開発サーバー: http://%s", *addr)
	log.Fatal(server.ListenAndServe())
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

func upgradeWebSocket(w http.ResponseWriter, r *http.Request) (*wsSocket, error) {
	if !localWebSocketOrigin(r) {
		http.Error(w, "同じ端末の画面から接続してください", http.StatusForbidden)
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
}

func newWSPeer(socket *wsSocket) *wsPeer {
	peer := &wsPeer{socket: socket, outgoing: make(chan queuedFrame, 32), done: make(chan struct{})}
	go peer.writeLoop()
	return peer
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

func wsCloseFrame(code uint16) wsFrame {
	data := make([]byte, 2)
	binary.BigEndian.PutUint16(data, code)
	return wsFrame{opcode: 8, data: data}
}

func (a *demoApp) handleMatch(w http.ResponseWriter, r *http.Request) {
	playerID := r.URL.Query().Get("playerId")
	if err := replay.ValidateGameID(playerID); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "参加者IDは半角英数字と_-の1〜64文字で指定してください"})
		return
	}
	socket, err := upgradeWebSocket(w, r)
	if err != nil {
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
	for {
		_ = socket.conn.SetReadDeadline(time.Now().Add(90 * time.Second))
		frame, err := socket.readFrame()
		if err != nil {
			if errors.Is(err, errWSProtocol) {
				peer.flush(wsCloseFrame(1002))
			} else if errors.Is(err, errWSMessageTooLarge) {
				peer.flush(wsCloseFrame(1009))
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
