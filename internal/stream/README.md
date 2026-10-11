# NATS JetStream tasks

Broker domains publish and subscribe on the `foreverbull` JetStream (`subjects: foreverbull.>`).

Topic templates (`internal/stream/stream.go`):

| Kind | Subject |
| --- | --- |
| Task | `foreverbull.{module}.{component}.{method}` |
| State | `foreverbull.{module}.{component}.{method}.{id}.state` |
| Abort | `foreverbull.{module}.{component}.{method}.{id}.abort` |
| Event | `foreverbull.{module}.{component}.{method}.event` |

Example (backtest create): `foreverbull.backtest.backtest.create`, then `foreverbull.backtest.backtest.create.{id}.state`.

States (`proto/foreverbull/internal/stream/message.proto`): `CREATED`, `PUBLISHED`, `RECEIVED`, `COMPLETED`, `ERROR`, `CANCELED`.

`Publish` writes the task payload and `STATE_PUBLISHED`. The subscriber writes `STATE_RECEIVED`, runs the handler, then `STATE_COMPLETED` (and an `.event`) or `STATE_ERROR`. `WaitForCompletion` watches the state subject. On cancel it publishes `.abort`; the subscriber then writes `STATE_CANCELED`.

Orchestrations run steps in order. Messages in a step are published in parallel with `WaitForCompletion`.

```mermaid
sequenceDiagram
    participant Publisher
    participant NATS
    participant Subscriber

    Subscriber->>NATS: Subscribe foreverbull.{module}.{component}.{method}
    Publisher->>NATS: Publish task on foreverbull.{module}.{component}.{method}
    Publisher->>NATS: STATE_PUBLISHED on ...{id}.state
    NATS->>Subscriber: Deliver task
    Subscriber->>NATS: STATE_RECEIVED on ...{id}.state
    Subscriber->>NATS: Subscribe ...{id}.abort

    alt Success
        Subscriber->>NATS: STATE_COMPLETED on ...{id}.state
        Subscriber->>NATS: event on foreverbull.{module}.{component}.{method}.event
    else Handler error
        Subscriber->>NATS: STATE_ERROR on ...{id}.state
    else Abort
        Publisher->>NATS: Publish ...{id}.abort
        NATS->>Subscriber: Deliver abort
        Subscriber->>NATS: STATE_CANCELED on ...{id}.state
    end
```
