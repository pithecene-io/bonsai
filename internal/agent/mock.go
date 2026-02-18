package agent

import "context"

// NonInteractiveCall records a call to NonInteractive.
type NonInteractiveCall struct {
	SystemPrompt string
	UserPrompt   string
}

// InteractiveCall records a call to Interactive.
type InteractiveCall struct {
	SystemPrompt string
	ExtraArgs    []string
}

// MockAgent is a test double implementing Agent.
// It records calls and returns configurable responses.
type MockAgent struct {
	Name_                  string
	NonInteractiveResponse string
	NonInteractiveErr      error
	NonInteractiveCalls    []NonInteractiveCall
	InteractiveErr         error
	InteractiveCalls       []InteractiveCall
}

// Name returns the configured name.
func (m *MockAgent) Name() string { return m.Name_ }

// Interactive records the call and returns the configured error.
func (m *MockAgent) Interactive(_ context.Context, systemPrompt string, extraArgs []string) error {
	m.InteractiveCalls = append(m.InteractiveCalls, InteractiveCall{
		SystemPrompt: systemPrompt,
		ExtraArgs:    extraArgs,
	})
	return m.InteractiveErr
}

// NonInteractive records the call and returns the configured response/error.
func (m *MockAgent) NonInteractive(_ context.Context, systemPrompt, userPrompt string) (string, error) {
	m.NonInteractiveCalls = append(m.NonInteractiveCalls, NonInteractiveCall{
		SystemPrompt: systemPrompt,
		UserPrompt:   userPrompt,
	})
	return m.NonInteractiveResponse, m.NonInteractiveErr
}
