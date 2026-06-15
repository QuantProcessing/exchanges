# Strategy Authoring

Strategy authoring follows the NautilusTrader lifecycle shape with Go callback
interfaces. Strategies receive a `strategy.Runtime`, subscribe in `OnStart`,
react to typed market and execution callbacks, create orders through
`model.OrderFactory`, and submit commands through the runtime.

The canonical bracket guide is:

- [Strategy Authoring With Brackets](./superpowers/guides/strategy-authoring-bracket.md)

Runnable evidence:

```bash
env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 ./examples/nautilus_style -v
```

The side-by-side strategy comparison is:

- [Side-By-Side Nautilus And Go Examples](./superpowers/guides/side-by-side-nautilus-go-examples.md)
