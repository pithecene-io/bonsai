package agent

import (
	"context"
	"sync"
)

// EvaluateCall records a call to Evaluate.
type EvaluateCall struct {
	SystemPrompt string
	UserPrompt   string
	Model        Model
	Tools        ToolPolicy
}

// SessionCall records a call to Session.
type SessionCall struct {
	SystemPrompt string
	ExtraArgs    []string
}

// ExecuteCall records a call to Execute.
type ExecuteCall struct {
	SystemPrompt string
	UserPrompt   string
	Model        Model
}

// MockAgent is a test double implementing Agent.
// It records calls and returns configurable responses.
// All methods are safe for concurrent use.
type MockAgent struct {
	mu               sync.Mutex
	NameVal          string
	EvaluateResponse string
	EvaluateErr      error
	EvaluateCalls    []EvaluateCall
	SessionErr       error
	SessionCalls     []SessionCall
	ExecuteErr       error
	ExecuteCalls     []ExecuteCall

	// EvaluateFunc, when set, is called instead of returning
	// the static EvaluateResponse/EvaluateErr. Useful for
	// per-call mock responses in parallel tests.
	EvaluateFunc func(ctx context.Context, systemPrompt, userPrompt string, model Model, tools ToolPolicy) (string, error)

	// ExecuteFunc, when set, is called instead of returning the
	// static ExecuteErr. Useful for per-call mock behavior.
	ExecuteFunc func(ctx context.Context, systemPrompt, userPrompt string, model Model) error
}

// Name returns the configured name.
func (m *MockAgent) Name() string { return m.NameVal }

// Session records the call and returns the configured error.
func (m *MockAgent) Session(_ context.Context, systemPrompt string, extraArgs []string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.SessionCalls = append(m.SessionCalls, SessionCall{
		SystemPrompt: systemPrompt,
		ExtraArgs:    extraArgs,
	})
	return m.SessionErr
}

// Evaluate records the call and returns the configured response/error.
func (m *MockAgent) Evaluate(ctx context.Context, systemPrompt, userPrompt string, model Model, tools ToolPolicy) (string, error) {
	m.mu.Lock()
	m.EvaluateCalls = append(m.EvaluateCalls, EvaluateCall{
		SystemPrompt: systemPrompt,
		UserPrompt:   userPrompt,
		Model:        model,
		Tools:        tools,
	})
	fn := m.EvaluateFunc
	resp := m.EvaluateResponse
	err := m.EvaluateErr
	m.mu.Unlock()

	if fn != nil {
		return fn(ctx, systemPrompt, userPrompt, model, tools)
	}
	return resp, err
}

// Execute records the call and returns the configured error.
func (m *MockAgent) Execute(ctx context.Context, systemPrompt, userPrompt string, model Model) error {
	m.mu.Lock()
	m.ExecuteCalls = append(m.ExecuteCalls, ExecuteCall{
		SystemPrompt: systemPrompt,
		UserPrompt:   userPrompt,
		Model:        model,
	})
	fn := m.ExecuteFunc
	err := m.ExecuteErr
	m.mu.Unlock()

	if fn != nil {
		return fn(ctx, systemPrompt, userPrompt, model)
	}
	return err
}

// CallCount returns the number of Evaluate calls recorded.
func (m *MockAgent) CallCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.EvaluateCalls)
}
