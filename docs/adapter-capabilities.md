# Adapter Capabilities

Adapter capability declarations must match implementation and shared contract
tests. A claimed capability is a release promise; an unsupported capability
must remain explicit and must not be hidden behind no-op success.

Current matrix:

- [Adapter Capability Matrix](./superpowers/gaps/adapter-capability-matrix.md)

Policy guide:

- [Adapter Capability Policy](./superpowers/guides/adapter-capability-policy.md)

Verification:

```bash
env GOCACHE=/private/tmp/go-build-exchanges go test -count=1 ./venue ./testsuite ./adapter/... ./config/all -run 'Adapter|Capability|Contract|PrivateStream|Resubscribe' -v
```
