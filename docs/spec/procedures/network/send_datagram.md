# Procedure Specification: P1.4 - SendSADPDatagram
## Module: M1 - Network Engine

## 1. Purpose

Send one caller-built SADP command datagram to a target using the caller-owned packet connection.

## 2. Go Contract

```go
func SendSADPDatagram(
    ctx context.Context,
    conn net.PacketConn,
    target net.Addr,
    payload []byte,
    maxDatagramSize int,
) error
```

- Reject nil context, connection, target, empty payload, a limit outside `1..65507`, or a payload larger than the configured limit.
- Do not take ownership of the connection, wait for a response, or retry.
- Send exactly one datagram, preserving the caller-supplied target and payload bytes.

## 3. Processing Requirements

1. Check the context immediately before writing. If it has a deadline, bound the write by that deadline without changing the read deadline or closing the caller-owned connection.
2. Call `WriteTo` once. Treat a short write as an error; do not retry because that could duplicate a command.
3. M3 uses this API for SADP XML such as activation built by P3.1. The selected connection and target endpoint follow M0/M6; the XML `TargetMAC` identifies the intended device.
4. Do not parse or correlate responses here. M1 `Listen` and M2 handle incoming datagrams; device-operation result semantics belong to M3.
5. Do not log payload bytes, credentials, or device security fields.

## 4. Errors and Tests

Return `TransportError` with stable stage `validation`, `write`, or `short_write`, preserving the underlying cause. Cancellation remains detectable using `errors.Is`. Test nil inputs, invalid limits, empty and oversized payloads, pre-canceled context, exact one-write behavior, target/payload preservation, short writes, write errors, and a caller-owned connection that remains open.

---

*Level 2 atomic procedure specification.*
