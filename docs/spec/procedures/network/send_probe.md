# Procedure Specification: P1.2 - SendInquiryProbe
## Module: M1 - Network Engine

## 1. Purpose

Build and send one observed SADP inquiry probe to the configured multicast endpoint.

## 2. Go Contract

```go
func SendInquiryProbe(ctx context.Context, conn net.PacketConn, target net.Addr) error
```

- Reject a nil context, connection, or target with an error.
- Return an error if `ctx` is already canceled before the write, if probe construction fails, or if writing fails or reports a short datagram.
- Do not take ownership of `conn`; the caller may pass the same connection to M1 `Listen` after this call.
- Send exactly one UDP datagram per call. Do not wait for a response or retry.

## 3. Probe Payload

The user-provided HiToolsDelivery capture observes this UTF-8 XML request shape:

```xml
<?xml version="1.0" encoding="utf-8"?><Probe><Uuid>...</Uuid><Types>inquiry</Types><ResponseMode>2</ResponseMode></Probe>
```

- Serialize XML with `encoding/xml`; do not construct it with string formatting.
- Include a UUID value in the captured canonical hexadecimal UUID form. Generate a new UUID for each call; the capture shows six UUID values, each occurring twice, but does not establish retry semantics.
- Emit the exact element names and values shown above. Preserve the observed lowercase `utf-8` declaration and exact `inquiry` and `2` values.
- Send the complete XML as one datagram to the caller-supplied target. `239.255.255.250:37020` is the endpoint observed in this capture and the current compatibility target; do not silently replace a caller-supplied target.
- Do not append multiple XML documents into one datagram.

## 4. Response and Error Policy

- A subsequent user-provided capture observes a 1,382-byte device `<ProbeMatch>` XML response containing `<Uuid>`, `<Types>inquiry</Types>`, and device information fields. The response UUID is present in both displayed response records, but the corresponding request payload is not included, so equality with the probe UUID cannot be confirmed. The response payload is shown in an exchange record between `192.168.0.128` and `192.168.0.160` on UDP/37020 and in a device-originated record to `239.255.255.250:37020`. This is one device observation only; it does not establish universal response delivery, response validation, correlation policy, required reply count, retries, or timing.
- A later capture shows fourteen probes containing seven distinct UUID values, each repeated twice. This repeated pattern does not by itself specify retransmission or UUID reuse semantics. The same capture includes a separate 1,319-byte datagram from `192.168.0.160:37020` to `239.255.255.250:37020` with root `<Hello>` and `<Types>hello</Types>`; it contains no displayed UUID and is not established as a reply to a probe. M2 handles the observed Hello advertisement shape.
- Parse the observed response with M2; this send procedure does not consume or correlate responses.
- Do not consume responses in this procedure. Response reception remains the responsibility of M1 `Listen` and downstream handlers.
- Return errors with operation context and preserve underlying write errors. Do not include packet contents, device identifiers, salt values, or other security-related data in errors or logs.
- Check the context immediately before writing. If it has a deadline, bound the write by that deadline without altering the read deadline or closing the caller-owned connection. Cancellation after a UDP write begins cannot recall a datagram already accepted by the operating system.

## 5. Tests

Test nil arguments, pre-canceled context, exact XML declaration and element names/values, valid UUID format, one datagram per invocation, exact target preservation, short-write handling, and propagation of write errors. Use a fake `net.PacketConn`; tests must not require live devices or multicast networking.

---

*Level 2 atomic procedure specification.*
