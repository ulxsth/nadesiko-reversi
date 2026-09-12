package e2e

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

type position struct {
	Row int `json:"row"`
	Col int `json:"col"`
}

type gameState struct {
	Version           string     `json:"version"`
	GameID            string     `json:"gameId"`
	Board             []*uint8   `json:"board"`
	TurnNumber        int        `json:"turnNumber"`
	CurrentPlayer     string     `json:"currentPlayer"`
	NextColor         uint8      `json:"nextColor"`
	RNGState          uint32     `json:"rngState"`
	ConsecutivePasses int        `json:"consecutivePasses"`
	Phase             string     `json:"phase"`
	Winner            *string    `json:"winner"`
	LegalMoves        []position `json:"legalMoves"`
}

type apiError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type apiResponse struct {
	Version string     `json:"version"`
	OK      bool       `json:"ok"`
	State   *gameState `json:"state"`
	Error   *apiError  `json:"error"`
}

func TestLocalPlayMVP(t *testing.T) {
	baseURL, stop := startServer(t)
	defer stop()

	initial := postRuntime(t, baseURL, map[string]any{
		"version": "1", "action": "newGame", "gameId": "e2e-game", "seed": 1,
	})
	if !initial.OK || initial.State == nil || len(initial.State.LegalMoves) == 0 {
		t.Fatalf("newGame response = %#v", initial)
	}

	t.Run("invalid move keeps authoritative state", func(t *testing.T) {
		before := *initial.State
		response := postRuntime(t, baseURL, applyRequest(initial.State, map[string]any{
			"commandId": "invalid-occupied", "type": "place", "player": "dark",
			"expectedTurn": 0, "row": 3, "col": 3,
		}))
		if response.OK || response.Error == nil || response.Error.Code != "occupied" {
			t.Fatalf("response = %#v", response)
		}
		if response.State == nil || !reflect.DeepEqual(*response.State, before) {
			t.Fatalf("state changed after rejection\n got: %#v\nwant: %#v", response.State, before)
		}
	})

	t.Run("legal move changes color and turn", func(t *testing.T) {
		move := initial.State.LegalMoves[0]
		response := postRuntime(t, baseURL, applyRequest(initial.State, map[string]any{
			"commandId": "legal-place", "type": "place", "player": initial.State.CurrentPlayer,
			"expectedTurn": initial.State.TurnNumber, "row": move.Row, "col": move.Col,
		}))
		if !response.OK || response.State == nil {
			t.Fatalf("response = %#v", response)
		}
		index := move.Row*8 + move.Col
		if response.State.TurnNumber != 1 || response.State.CurrentPlayer == initial.State.CurrentPlayer || response.State.Board[index] == nil {
			t.Fatalf("state did not advance = %#v", response.State)
		}
	})

	reset := postRuntime(t, baseURL, map[string]any{
		"version": "1", "action": "newGame", "gameId": "e2e-game", "seed": 1,
	})
	if !reset.OK || reset.State == nil {
		t.Fatalf("reset response = %#v", reset)
	}
	if !reflect.DeepEqual(reset.State.Board, initial.State.Board) || reset.State.RNGState != initial.State.RNGState || reset.State.NextColor != initial.State.NextColor {
		t.Fatal("same seed did not reproduce the initial state")
	}

	t.Run("game reaches a winner including passes when needed", func(t *testing.T) {
		state := reset.State
		for step := 0; state.Phase != "finished"; step++ {
			if step >= 80 {
				t.Fatal("game did not finish within 80 commands")
			}
			command := map[string]any{
				"commandId": fmt.Sprintf("autoplay-%d", step),
				"player":    state.CurrentPlayer, "expectedTurn": state.TurnNumber,
			}
			if len(state.LegalMoves) == 0 {
				command["type"] = "pass"
			} else {
				move := state.LegalMoves[0]
				command["type"] = "place"
				command["row"] = move.Row
				command["col"] = move.Col
			}
			response := postRuntime(t, baseURL, applyRequest(state, command))
			if !response.OK || response.State == nil {
				t.Fatalf("step %d response = %#v", step, response)
			}
			state = response.State
		}
		if state.Winner == nil || (*state.Winner != "dark" && *state.Winner != "light") {
			t.Fatalf("winner = %#v", state.Winner)
		}
	})
}

func applyRequest(state *gameState, command map[string]any) map[string]any {
	return map[string]any{
		"version": "1", "action": "applyCommand", "state": state, "command": command,
	}
}

func postRuntime(t *testing.T, baseURL string, request any) apiResponse {
	t.Helper()
	payload, err := json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	response, err := http.Post(baseURL+"/api/local/runtime", "application/json", bytes.NewReader(payload))
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.StatusCode, body)
	}
	var decoded apiResponse
	if err := json.Unmarshal(body, &decoded); err != nil {
		t.Fatalf("decode response: %v, body = %s", err, body)
	}
	return decoded
}

func startServer(t *testing.T) (string, func()) {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	goBinary := filepath.Join(root, ".tools", "go", "bin", "go")
	serverBinary := filepath.Join(t.TempDir(), "nadesiko-reversi-server")
	build := exec.Command(goBinary, "build", "-o", serverBinary, "./server")
	build.Dir = root
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build server: %v\n%s", err, output)
	}

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	address := listener.Addr().String()
	if err := listener.Close(); err != nil {
		t.Fatal(err)
	}

	command := exec.Command(serverBinary,
		"-addr", address,
		"-web", filepath.Join(root, "web"),
		"-rules", filepath.Join(root, "rules"),
	)
	command.Dir = root
	command.Env = append(os.Environ(), "GONAKO_BIN="+filepath.Join(root, ".tools", "bin", "gonako"))
	var logs bytes.Buffer
	command.Stdout = &logs
	command.Stderr = &logs
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}

	baseURL := "http://" + address
	deadline := time.Now().Add(10 * time.Second)
	for {
		response, err := http.Get(baseURL + "/healthz")
		if err == nil {
			response.Body.Close()
			if response.StatusCode == http.StatusOK {
				break
			}
		}
		if time.Now().After(deadline) {
			_ = command.Process.Kill()
			_ = command.Wait()
			t.Fatalf("server did not start: %v\n%s", err, logs.String())
		}
		time.Sleep(50 * time.Millisecond)
	}

	stop := func() {
		_ = command.Process.Kill()
		_ = command.Wait()
	}
	return baseURL, stop
}
