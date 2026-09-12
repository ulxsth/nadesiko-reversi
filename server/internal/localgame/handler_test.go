package localgame

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ulxsth/nadesiko-reversi/server/internal/protocol"
	"github.com/ulxsth/nadesiko-reversi/server/internal/runtime/runtimetest"
)

func TestHandlerReturnsRuntimeResponse(t *testing.T) {
	fake := runtimetest.New(func(_ context.Context, request protocol.Request) (*protocol.Response, error) {
		return runtimetest.Accepted(runtimetest.NewState(request.GameID)), nil
	})
	handler := NewHandler(NewService(fake))
	requestBody, err := json.Marshal(protocol.NewGameRequest("handler-game", 1))
	if err != nil {
		t.Fatal(err)
	}

	request := httptest.NewRequest(http.MethodPost, "/api/local/runtime", bytes.NewReader(requestBody))
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	var response protocol.Response
	if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
		t.Fatal(err)
	}
	if !response.OK || response.State == nil || response.State.GameID != "handler-game" {
		t.Fatalf("response = %#v", response)
	}
}

func TestHandlerRejectsMalformedJSON(t *testing.T) {
	handler := NewHandler(NewService(runtimetest.New(nil)))
	request := httptest.NewRequest(http.MethodPost, "/api/local/runtime", bytes.NewBufferString("{"))
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d", recorder.Code)
	}
	var response protocol.Response
	if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
		t.Fatal(err)
	}
	if response.OK || response.Error == nil || response.Error.Code != protocol.CodeInvalidJSON {
		t.Fatalf("response = %#v", response)
	}
}
