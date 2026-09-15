package main

import (
	"encoding/binary"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
	"unicode/utf8"
)

// envMap はloadConfigへ渡す環境変数の差し替え。
func envMap(values map[string]string) func(string) string {
	return func(name string) string { return values[name] }
}

func TestResolveAddr(t *testing.T) {
	cases := []struct {
		name    string
		addr    string
		addrSet bool
		port    string
		want    string
		wantErr bool
	}{
		{name: "PORTなしは開発用の既定値", addr: defaultAddr, port: "", want: defaultAddr},
		{name: "PORTありは全インターフェース", addr: defaultAddr, port: "8080", want: "0.0.0.0:8080"},
		{name: "-addrの明示指定が最優先", addr: "127.0.0.1:5000", addrSet: true, port: "8080", want: "127.0.0.1:5000"},
		{name: "PORTが整数でない", addr: defaultAddr, port: "http", wantErr: true},
		{name: "PORTが範囲外", addr: defaultAddr, port: "70000", wantErr: true},
		{name: "-addrが空", addr: "  ", addrSet: true, wantErr: true},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			got, err := resolveAddr(testCase.addr, testCase.addrSet, testCase.port)
			if testCase.wantErr {
				if err == nil {
					t.Fatalf("errorを期待したが %q が返った", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("想定外のerror: %v", err)
			}
			if got != testCase.want {
				t.Fatalf("アドレスが違う: got %q want %q", got, testCase.want)
			}
		})
	}
}

func TestParsePublicOrigin(t *testing.T) {
	cases := []struct {
		name    string
		raw     string
		want    string
		wantErr bool
	}{
		{name: "未設定", raw: "", want: ""},
		{name: "run.appのURL", raw: "https://demo-abc.a.run.app", want: "https://demo-abc.a.run.app"},
		{name: "末尾スラッシュは落とす", raw: "https://demo-abc.a.run.app/", want: "https://demo-abc.a.run.app"},
		{name: "hostは小文字化する", raw: "https://Demo-ABC.a.run.app", want: "https://demo-abc.a.run.app"},
		{name: "portつき", raw: "https://example.com:8443", want: "https://example.com:8443"},
		{name: "httpは拒否", raw: "http://example.com", wantErr: true},
		{name: "ループバックのhttpは許す", raw: "http://127.0.0.1:8080", want: "http://127.0.0.1:8080"},
		{name: "localhostのhttpは許す", raw: "http://localhost:8080", want: "http://localhost:8080"},
		{name: "pathつきは拒否", raw: "https://example.com/demo", wantErr: true},
		{name: "queryつきは拒否", raw: "https://example.com?a=1", wantErr: true},
		{name: "userinfoつきは拒否", raw: "https://user@example.com", wantErr: true},
		{name: "hostなしは拒否", raw: "https://", wantErr: true},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			got, err := parsePublicOrigin(testCase.raw)
			if testCase.wantErr {
				if err == nil {
					t.Fatalf("errorを期待したが %q が返った", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("想定外のerror: %v", err)
			}
			if got != testCase.want {
				t.Fatalf("originが違う: got %q want %q", got, testCase.want)
			}
		})
	}
}

func TestPublicOriginMatches(t *testing.T) {
	const publicOrigin = "https://demo-abc.a.run.app"
	cases := []struct {
		name   string
		header string
		want   bool
	}{
		{name: "完全一致", header: "https://demo-abc.a.run.app", want: true},
		{name: "大文字のhost", header: "https://DEMO-ABC.a.run.app", want: true},
		{name: "別のsubdomain", header: "https://evil-demo-abc.a.run.app"},
		{name: "前方一致だけのhost", header: "https://demo-abc.a.run.app.example.com"},
		{name: "scheme違い", header: "http://demo-abc.a.run.app"},
		{name: "末尾スラッシュ", header: "https://demo-abc.a.run.app/"},
		{name: "portつき", header: "https://demo-abc.a.run.app:8443"},
		{name: "null origin", header: "null"},
		{name: "空", header: ""},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			if got := publicOriginMatches(publicOrigin, testCase.header); got != testCase.want {
				t.Fatalf("判定が違う: got %v want %v", got, testCase.want)
			}
		})
	}

	if publicOriginMatches("", "https://demo-abc.a.run.app") {
		t.Fatal("公開origin未設定なら、どのoriginも許可しない")
	}
}

// newTestApp はgonakoを起動しないdemoAppを組み立てる。
func newTestApp(config demoConfig) *demoApp {
	app := &demoApp{config: config, rulesReady: func() bool { return true }}
	app.rulesVerified.Store(true)
	return app
}

func TestAllowWebSocketOrigin(t *testing.T) {
	app := newTestApp(demoConfig{publicOrigin: "https://demo-abc.a.run.app"})

	publicRequest := func() *http.Request {
		request := httptest.NewRequest(http.MethodGet, "/api/match?playerId=p1", nil)
		request.RemoteAddr = "203.0.113.7:44321"
		request.Host = "demo-abc.a.run.app"
		return request
	}

	allowed := publicRequest()
	allowed.Header.Set("Origin", "https://demo-abc.a.run.app")
	if !app.allowWebSocketOrigin(allowed) {
		t.Fatal("公開originからの接続を許可していない")
	}

	forged := publicRequest()
	forged.Header.Set("Origin", "https://attacker.example.com")
	forged.Header.Set("X-Forwarded-Host", "demo-abc.a.run.app")
	forged.Header.Set("X-Forwarded-Proto", "https")
	if app.allowWebSocketOrigin(forged) {
		t.Fatal("転送ヘッダーを信用して接続を許可している")
	}

	noOrigin := publicRequest()
	if app.allowWebSocketOrigin(noOrigin) {
		t.Fatal("Originなしの接続を許可している")
	}

	loopback := httptest.NewRequest(http.MethodGet, "/api/match?playerId=p1", nil)
	loopback.RemoteAddr = "127.0.0.1:52341"
	loopback.Host = "127.0.0.1:4173"
	loopback.Header.Set("Origin", "http://127.0.0.1:4173")
	if !app.allowWebSocketOrigin(loopback) {
		t.Fatal("ローカルの同一origin接続を許可していない")
	}

	remoteWithLocalHost := httptest.NewRequest(http.MethodGet, "/api/match?playerId=p1", nil)
	remoteWithLocalHost.RemoteAddr = "203.0.113.7:44321"
	remoteWithLocalHost.Host = "127.0.0.1:4173"
	remoteWithLocalHost.Header.Set("Origin", "http://127.0.0.1:4173")
	if app.allowWebSocketOrigin(remoteWithLocalHost) {
		t.Fatal("ループバック以外からのHost偽装を許可している")
	}
}

func TestUpgradeWebSocketRejectsDisallowedOrigin(t *testing.T) {
	app := newTestApp(demoConfig{publicOrigin: "https://demo-abc.a.run.app"})
	request := httptest.NewRequest(http.MethodGet, "/api/match?playerId=p1", nil)
	request.RemoteAddr = "203.0.113.7:44321"
	request.Host = "demo-abc.a.run.app"
	request.Header.Set("Origin", "https://attacker.example.com")
	request.Header.Set("Connection", "Upgrade")
	request.Header.Set("Upgrade", "websocket")
	request.Header.Set("Sec-WebSocket-Version", "13")

	recorder := httptest.NewRecorder()
	socket, err := app.upgradeWebSocket(recorder, request)
	if err == nil || socket != nil {
		t.Fatal("許可されないoriginのupgradeを受け入れている")
	}
	if recorder.Code != http.StatusForbidden {
		t.Fatalf("statusが違う: got %d want %d", recorder.Code, http.StatusForbidden)
	}
}

func TestRateLimiterRefills(t *testing.T) {
	start := time.Date(2026, 9, 15, 12, 0, 0, 0, time.UTC)
	limiter := newRateLimiter(2, 3, start)

	for i := 0; i < 3; i++ {
		if !limiter.allow(start) {
			t.Fatalf("burst内の%d件目を拒否している", i+1)
		}
	}
	if limiter.allow(start) {
		t.Fatal("burstを超えたcommandを通している")
	}
	if !limiter.allow(start.Add(600 * time.Millisecond)) {
		t.Fatal("補充後のcommandを通していない")
	}
	if limiter.allow(start.Add(600 * time.Millisecond)) {
		t.Fatal("補充量を超えてcommandを通している")
	}
	if !limiter.allow(start.Add(10 * time.Second)) {
		t.Fatal("十分な時間が経った後のcommandを通していない")
	}
}

func TestAdmitLimitsConnections(t *testing.T) {
	app := newTestApp(demoConfig{maxConnections: 2})

	first, ok := app.admit()
	if !ok {
		t.Fatal("1本目の接続を断っている")
	}
	second, ok := app.admit()
	if !ok {
		t.Fatal("2本目の接続を断っている")
	}
	if _, ok := app.admit(); ok {
		t.Fatal("上限を超えた接続を受け入れている")
	}

	first()
	first()
	third, ok := app.admit()
	if !ok {
		t.Fatal("解放後の接続を断っている")
	}
	if _, ok := app.admit(); ok {
		t.Fatal("二重解放で上限が緩んでいる")
	}
	second()
	third()
	if app.connections.Load() != 0 {
		t.Fatalf("接続数が残っている: %d", app.connections.Load())
	}
}

func TestHandleHealth(t *testing.T) {
	app := newTestApp(demoConfig{})

	recorder := httptest.NewRecorder()
	app.handleHealth(recorder, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("readyのstatusが違う: got %d", recorder.Code)
	}
	var ready healthResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &ready); err != nil {
		t.Fatalf("応答を解析できません: %v", err)
	}
	if !ready.GoReady || !ready.GonakoReady {
		t.Fatalf("readyを返していない: %+v", ready)
	}

	app.rulesReady = func() bool { return false }
	degradedRecorder := httptest.NewRecorder()
	app.handleHealth(degradedRecorder, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if degradedRecorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("未readyのstatusが違う: got %d", degradedRecorder.Code)
	}
	var degraded healthResponse
	if err := json.Unmarshal(degradedRecorder.Body.Bytes(), &degraded); err != nil {
		t.Fatalf("応答を解析できません: %v", err)
	}
	if !degraded.GoReady || degraded.GonakoReady {
		t.Fatalf("Go側とgonako側を区別していない: %+v", degraded)
	}

	unverified := newTestApp(demoConfig{})
	unverified.rulesVerified.Store(false)
	if unverified.gonakoReady() {
		t.Fatal("起動確認に失敗した状態をreadyとして扱っている")
	}
}

func TestWSCloseFrameCarriesReason(t *testing.T) {
	frame := wsCloseFrame(closeIdle, "一定時間操作がなかったため切断しました。再接続してください")
	if frame.opcode != 8 {
		t.Fatalf("opcodeが違う: %d", frame.opcode)
	}
	if binary.BigEndian.Uint16(frame.data[:2]) != closeIdle {
		t.Fatalf("終了コードが違う: %d", binary.BigEndian.Uint16(frame.data[:2]))
	}
	if !utf8.Valid(frame.data[2:]) {
		t.Fatal("理由がUTF-8として不正")
	}

	// 3byte文字60個はpayload上限を超えるので、UTF-8境界で切り詰められる。
	long := wsCloseFrame(closeTryLater, strings.Repeat("あ", 60))
	if len(long.data) > 125 {
		t.Fatalf("close frameのpayloadが上限を超えている: %d", len(long.data))
	}
	if !utf8.Valid(long.data[2:]) {
		t.Fatal("切り詰めた理由がUTF-8として不正")
	}
}

func TestLoadConfig(t *testing.T) {
	config, err := loadConfig(envMap(nil), defaultAddr, false, "web", "rules")
	if err != nil {
		t.Fatalf("既定値で失敗した: %v", err)
	}
	if config.addr != defaultAddr || config.publicOrigin != "" {
		t.Fatalf("既定の待ち受けが違う: %+v", config)
	}
	if config.maxConnections != defaultMaxConns || config.maxRooms != defaultMaxRooms {
		t.Fatalf("既定の上限が違う: %+v", config)
	}
	if config.idleTimeout != defaultIdleTimeout || config.maxConnectionAge != defaultConnectionAge {
		t.Fatalf("既定の時間上限が違う: %+v", config)
	}
	if config.readTimeout <= config.pingInterval {
		t.Fatal("読み取りdeadlineがping間隔より短い")
	}

	configured, err := loadConfig(envMap(map[string]string{
		"PORT":                   "8080",
		"PUBLIC_ORIGIN":          "https://demo-abc.a.run.app",
		"MAX_CONNECTIONS":        "4",
		"MAX_ROOMS":              "2",
		"COMMAND_RATE_PER_SEC":   "3",
		"COMMAND_BURST":          "5",
		"IDLE_TIMEOUT_SECONDS":   "120",
		"MAX_CONNECTION_SECONDS": "1800",
	}), defaultAddr, false, "web", "rules")
	if err != nil {
		t.Fatalf("環境変数の解釈に失敗した: %v", err)
	}
	if configured.addr != "0.0.0.0:8080" || configured.publicOrigin != "https://demo-abc.a.run.app" {
		t.Fatalf("公開構成が違う: %+v", configured)
	}
	if configured.maxConnections != 4 || configured.maxRooms != 2 || configured.commandRate != 3 || configured.commandBurst != 5 {
		t.Fatalf("上限の解釈が違う: %+v", configured)
	}
	if configured.idleTimeout != 2*time.Minute || configured.maxConnectionAge != 30*time.Minute {
		t.Fatalf("時間上限の解釈が違う: %+v", configured)
	}

	invalid := []map[string]string{
		{"PUBLIC_ORIGIN": "http://example.com"},
		{"MAX_CONNECTIONS": "0"},
		{"MAX_ROOMS": "-1"},
		{"COMMAND_RATE_PER_SEC": "many"},
		{"IDLE_TIMEOUT_SECONDS": "10"},
		{"MAX_CONNECTION_SECONDS": "99999999"},
		{"PORT": "0"},
	}
	for _, values := range invalid {
		if _, err := loadConfig(envMap(values), defaultAddr, false, "web", "rules"); err == nil {
			t.Fatalf("不正な構成で起動を止めていない: %v", values)
		}
	}
}
