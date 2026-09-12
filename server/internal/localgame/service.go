// Package localgame provides the single-match, in-memory application service
// used by the local hot-seat MVP.
package localgame

import (
	"context"
	"sync"

	"github.com/ulxsth/nadesiko-reversi/server/internal/protocol"
	"github.com/ulxsth/nadesiko-reversi/server/internal/runtime"
)

type Service struct {
	mu     sync.Mutex
	runner runtime.Runner
	state  *protocol.State
}

func NewService(runner runtime.Runner) *Service {
	return &Service{runner: runner}
}

// Evaluate applies one runtime request to the server-owned match. Calls are
// serialized so two commands cannot both consume the same turn.
func (s *Service) Evaluate(ctx context.Context, request protocol.Request) (*protocol.Response, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	switch request.Action {
	case protocol.ActionNewGame:
		response, err := s.runner.Evaluate(ctx, request)
		if err != nil {
			return nil, err
		}
		if response.OK && response.State != nil {
			state := response.State.Clone()
			s.state = &state
		}
		return cloneResponse(response), nil

	case protocol.ActionApplyCommand:
		if request.Version != protocol.Version {
			return rejected(protocol.CodeUnsupportedVersion, "対応していないバージョンです", s.state), nil
		}
		if request.Command == nil {
			return rejected(protocol.CodeInvalidRequest, "commandが必要です", s.state), nil
		}
		if err := request.Command.Validate(); err != nil {
			return rejected(err.Code, err.Message, s.state), nil
		}
		if s.state == nil {
			return rejected(protocol.CodeInvalidState, "先に新しい対局を開始してください", nil), nil
		}

		authoritative := s.state.Clone()
		response, err := runtime.ApplyCommand(ctx, s.runner, authoritative, *request.Command)
		if err != nil {
			return nil, err
		}
		if response.OK && response.State != nil {
			state := response.State.Clone()
			s.state = &state
		} else if response.State == nil {
			state := s.state.Clone()
			response.State = &state
		}
		return cloneResponse(response), nil

	default:
		return rejected(protocol.CodeInvalidRequest, "actionはnewGameまたはapplyCommandです", s.state), nil
	}
}

func (s *Service) Snapshot() *protocol.State {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.state == nil {
		return nil
	}
	state := s.state.Clone()
	return &state
}

func rejected(code protocol.ErrorCode, message string, state *protocol.State) *protocol.Response {
	response := &protocol.Response{
		Version: protocol.Version,
		OK:      false,
		Error:   protocol.NewError(code, message),
	}
	if state != nil {
		clone := state.Clone()
		response.State = &clone
	}
	return response
}

func cloneResponse(response *protocol.Response) *protocol.Response {
	if response == nil {
		return nil
	}
	clone := *response
	if response.State != nil {
		state := response.State.Clone()
		clone.State = &state
	}
	if response.Event != nil {
		event := *response.Event
		if response.Event.Changes != nil {
			event.Changes = append([]protocol.Change(nil), response.Event.Changes...)
		}
		clone.Event = &event
	}
	if response.Error != nil {
		runtimeError := *response.Error
		clone.Error = &runtimeError
	}
	return &clone
}
