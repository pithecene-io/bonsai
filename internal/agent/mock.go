package agent

import (
	"context"
	"sync"
)

// NonInteractiveCall records a call to NonInteractive.
type NonInteractiveCall struct {
	SystemPrompt string
	UserPrompt   string
	Model        Model
}

// InteractiveCall records a call to Interactive.
type InteractiveCall struct {
	SystemPrompt string
	ExtraArgs    []string
}

// AutonomousCall records a call to Autonomous.
type AutonomousCall struct {
	SystemPrompt string
	UserPrompt   string
	Model        Model
}

// MockAgent is a test double implementing Agent.
// It records calls and returns configurable responses.
// All methods are safe for concurrent use.
type MockAgent struct {
	mu                     sync.Mutex
	NameVal                string
	NonInteractiveResponse string
	NonInteractiveErr      error
	NonInteractiveCalls    []NonInteractiveCall
	InteractiveErr         error
	InteractiveCalls       []InteractiveCall
	AutonomousErr          error
	AutonomousCalls        []AutonomousCall

	// NonInteractiveFunc, when set, is called instead of returning
	// the static NonInteractiveResponse/NonInteractiveErr. Useful for
	// per-call mock responses in parallel tests.
	NonInteractiveFunc func(ctx context.Context, systemPrompt, userPrompt string, model Model) (string, error)

	// AutonomousFunc, when set, is called instead of returning the
	// static AutonomousErr. Useful for per-call mock behavior.
	AutonomousFunc func(ctx context.Context, systemPrompt, userPrompt string, model Model) error
}

// Name returns the configured name.
func (m *MockAgent) Name() string { return m.NameVal }

// Interactive records the call and returns the configured error.
func (m *MockAgent) Interactive(_ context.Context, systemPrompt string, extraArgs []string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.InteractiveCalls = append(m.InteractiveCalls, InteractiveCall{
		SystemPrompt: systemPrompt,
		ExtraArgs:    extraArgs,
	})
	return m.InteractiveErr
}

// NonInteractive records the call and returns the configured response/error.
func (m *MockAgent) NonInteractive(ctx context.Context, systemPrompt, userPrompt string, model Model) (string, error) {
	m.mu.Lock()
	m.NonInteractiveCalls = append(m.NonInteractiveCalls, NonInteractiveCall{
		SystemPrompt: systemPrompt,
		UserPrompt:   userPrompt,
		Model:        model,
	})
	fn := m.NonInteractiveFunc
	resp := m.NonInteractiveResponse
	err := m.NonInteractiveErr
	m.mu.Unlock()

	if fn != nil {
		return fn(ctx, systemPrompt, userPrompt, model)
	}
	return resp, err
}

// Autonomous records the call and returns the configured error.
func (m *MockAgent) Autonomous(ctx context.Context, systemPrompt, userPrompt string, model Model) error {
	m.mu.Lock()
	m.AutonomousCalls = append(m.AutonomousCalls, AutonomousCall{
		SystemPrompt: systemPrompt,
		UserPrompt:   userPrompt,
		Model:        model,
	})
	fn := m.AutonomousFunc
	err := m.AutonomousErr
	m.mu.Unlock()

	if fn != nil {
		return fn(ctx, systemPrompt, userPrompt, model)
	}
	return err
}

// CallCount returns the number of NonInteractive calls recorded.
func (m *MockAgent) CallCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.NonInteractiveCalls)
}
