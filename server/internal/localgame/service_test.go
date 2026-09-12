package localgame

import (
	"context"
	"reflect"
	"testing"

	"github.com/ulxsth/nadesiko-reversi/server/internal/protocol"
	"github.com/ulxsth/nadesiko-reversi/server/internal/runtime/runtimetest"
)

func TestServiceKeepsAuthoritativeStateOnRejection(t *testing.T) {
	initial := runtimetest.NewState("local")
	fake := runtimetest.New(func(_ context.Context, request protocol.Request) (*protocol.Response, error) {
		if request.Action == protocol.ActionNewGame {
			return runtimetest.Accepted(initial), nil
		}
		return runtimetest.Rejected(protocol.CodeOccupied, "そのマスには既に駒があります", *request.State), nil
	})
	service := NewService(fake)
	seed := uint32(1)
	created, err := service.Evaluate(context.Background(), protocol.NewGameRequest("local", seed))
	if err != nil || !created.OK {
		t.Fatalf("new game = %#v, %v", created, err)
	}

	forged := created.State.Clone()
	forged.TurnNumber = 99
	request := protocol.ApplyCommandRequest(forged, protocol.PlaceCommand("bad", protocol.PlayerDark, 0, 3, 3))
	rejectedResponse, err := service.Evaluate(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if rejectedResponse.OK || rejectedResponse.Error == nil || rejectedResponse.Error.Code != protocol.CodeOccupied {
		t.Fatalf("unexpected rejection: %#v", rejectedResponse)
	}
	if !reflect.DeepEqual(*rejectedResponse.State, initial) {
		t.Fatalf("rejected state changed: got %#v want %#v", *rejectedResponse.State, initial)
	}
	if got := fake.Requests()[1].State.TurnNumber; got != 0 {
		t.Fatalf("runner received client state: turn = %d", got)
	}
}

func TestServiceStoresAcceptedState(t *testing.T) {
	initial := runtimetest.NewState("local")
	next := initial.Clone()
	next.TurnNumber = 1
	next.CurrentPlayer = protocol.PlayerLight
	fake := runtimetest.New(func(_ context.Context, request protocol.Request) (*protocol.Response, error) {
		if request.Action == protocol.ActionNewGame {
			return runtimetest.Accepted(initial), nil
		}
		return runtimetest.Accepted(next), nil
	})
	service := NewService(fake)
	seed := uint32(1)
	if _, err := service.Evaluate(context.Background(), protocol.NewGameRequest("local", seed)); err != nil {
		t.Fatal(err)
	}
	request := protocol.ApplyCommandRequest(initial, protocol.PlaceCommand("ok", protocol.PlayerDark, 0, 2, 2))
	if _, err := service.Evaluate(context.Background(), request); err != nil {
		t.Fatal(err)
	}
	if got := service.Snapshot(); got == nil || got.TurnNumber != 1 || got.CurrentPlayer != protocol.PlayerLight {
		t.Fatalf("snapshot = %#v", got)
	}
}
