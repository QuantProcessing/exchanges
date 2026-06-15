package live

import (
	"context"
	"testing"

	"github.com/QuantProcessing/exchanges/bus"
	"github.com/QuantProcessing/exchanges/cache"
	"github.com/QuantProcessing/exchanges/kernel"
	"github.com/QuantProcessing/exchanges/portfolio"
	"github.com/QuantProcessing/exchanges/risk"
	"github.com/stretchr/testify/require"
)

func TestLiveNodeBuilderBuildsCanonicalNodeWithDefaults(t *testing.T) {
	node, err := NewNodeBuilder().Build()
	require.NoError(t, err)
	require.NotNil(t, node)
	require.NotNil(t, node.Bus())
	require.NotNil(t, node.Cache())
	require.NotNil(t, node.Portfolio())
	require.NotNil(t, node.Platform())
	require.NotNil(t, node.Risk())
	require.Equal(t, kernel.ComponentStateInitialized, node.Health().State)
	require.Equal(t, kernel.ComponentStateInitialized, node.Health().Platform.State)
	require.Equal(t, kernel.ComponentStateInitialized, node.Health().Platform.Risk.State)
}

func TestLiveNodeBuilderPreservesConfiguredDependencies(t *testing.T) {
	b := bus.New()
	c := cache.New()
	r := risk.NewEngine(c, risk.Config{})
	pf := portfolio.New(c)
	data := newLiveDataClient()
	exec := newLiveExecutionClient()
	rec := &recordingStrategy{id: "builder"}

	node, err := NewNodeBuilder().
		WithBus(b).
		WithCache(c).
		WithRisk(r).
		WithPortfolio(pf).
		AddDataClient(data).
		AddExecutionClient(exec).
		AddStrategy(rec).
		Build()
	require.NoError(t, err)
	require.Same(t, b, node.Bus())
	require.Same(t, c, node.Cache())
	require.Same(t, pf, node.Portfolio())
	require.Same(t, r, node.Risk())

	require.NoError(t, node.Start(context.Background()))
	require.True(t, rec.isStarted())
	require.NoError(t, node.Stop(context.Background()))
	require.True(t, rec.isStopped())
}

func TestLiveNodeBuilderRejectsInvalidComponents(t *testing.T) {
	_, err := NewNodeBuilder().AddDataClient(nil).Build()
	require.ErrorContains(t, err, "data client 0 is nil")

	_, err = NewNodeBuilder().AddExecutionClient(nil).Build()
	require.ErrorContains(t, err, "execution client 0 is nil")

	_, err = NewNodeBuilder().AddStrategy(nil).Build()
	require.ErrorContains(t, err, "strategy 0 is nil")
}

func TestNewTradingNodeReturnsCanonicalLiveNode(t *testing.T) {
	node, err := NewTradingNode(NodeConfig{})
	require.NoError(t, err)

	var canonical *Node = node
	require.Same(t, node, canonical)
	require.NotNil(t, canonical.Risk())
}
