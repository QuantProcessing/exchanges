package model

import "time"

type TraderID string
type StrategyID string
type CommandID string
type CorrelationID string
type ExecutionClientID string

type CommandMetadata struct {
	TraderID        TraderID
	StrategyID      StrategyID
	CommandID       CommandID
	CorrelationID   CorrelationID
	ClientID        ExecutionClientID
	ComponentID     ComponentID
	ExecAlgorithmID ExecAlgorithmID
	ExecSpawnID     ExecSpawnID
	TsInit          time.Time
	Params          map[string]string
}

func (m CommandMetadata) Clone() CommandMetadata {
	m.Params = cloneCommandParams(m.Params)
	return m
}

func (m CommandMetadata) WithDefaults(defaults CommandMetadata) CommandMetadata {
	m = m.Clone()
	if m.TraderID == "" {
		m.TraderID = defaults.TraderID
	}
	if m.StrategyID == "" {
		m.StrategyID = defaults.StrategyID
	}
	if m.CommandID == "" {
		m.CommandID = defaults.CommandID
	}
	if m.CorrelationID == "" {
		m.CorrelationID = defaults.CorrelationID
	}
	if m.ClientID == "" {
		m.ClientID = defaults.ClientID
	}
	if m.ComponentID == "" {
		m.ComponentID = defaults.ComponentID
	}
	if m.ExecAlgorithmID == "" {
		m.ExecAlgorithmID = defaults.ExecAlgorithmID
	}
	if m.ExecSpawnID == "" {
		m.ExecSpawnID = defaults.ExecSpawnID
	}
	if m.TsInit.IsZero() {
		m.TsInit = defaults.TsInit
	}
	if len(m.Params) == 0 && len(defaults.Params) > 0 {
		m.Params = cloneCommandParams(defaults.Params)
	}
	return m
}

func cloneCommandParams(params map[string]string) map[string]string {
	if len(params) == 0 {
		return nil
	}
	clone := make(map[string]string, len(params))
	for key, value := range params {
		clone[key] = value
	}
	return clone
}
