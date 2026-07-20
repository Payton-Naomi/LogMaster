# LogMaster Agent MVP

Windows serial log collector with durable local buffering and idempotent HTTP upload.

Requires Go 1.26.4.

## Run with the mock source

1. Copy `config/config.yaml` to `config/config.local.yaml`.
2. Comment the serial device and enable the `SIM001` mock device.
3. Start the receiver:

```powershell
go run ./cmd/mockserver
```

4. In another terminal, start the agent:

```powershell
go run ./cmd/agent -config config/config.local.yaml
```

Accepted batches are written to `data/mock-received`. Raw logs are written by device and day under `logs`. Pending uploads remain in `data/agent.db` until a valid acknowledgement is received.

## Run with COM24

The checked-in example uses `COM24`, `115200`, 8 data bits, 1 stop bit, no parity, and no flow control. Start the mock receiver, then run the agent with `config/config.yaml`. Stop the receiver to verify buffering, restart it to verify automatic replay, and unplug/reconnect the USB serial adapter to verify serial reconnection.

The reader preserves raw bytes until it finds `CRLF`, `CR`, or `LF`. Partial lines are flushed after `idle_flush`, on disconnect, or during graceful shutdown. This avoids corrupting UTF-8 characters split across serial reads.

## Upload contract

`POST /api/v1/logs/upload` sends gzip-compressed JSON and uses `Idempotency-Key` with a persistent batch ID. The server must return the same batch ID and received count. The Agent deletes local rows only after that acknowledgement.

Set `LOGMASTER_TOKEN` to override `upload.token` without storing credentials in YAML.

## Build and test

```powershell
go test ./...
go vet ./...
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\build.ps1
```

This project uses `modernc.org/sqlite`, so Windows builds do not require GCC or CGO.

## Design references

The serial behavior is informed by [SuperConnectX](https://github.com/SuperStudio/SuperConnectX) and [SuperCom](https://github.com/SuperStudio/SuperCom): preserve raw bytes until a complete line is available, support CRLF/CR/LF, flush partial data on idle/disconnect, keep per-connection state isolated, and test with a mock serial event stream. Their source code is not copied; this Agent is an independent Go implementation.
