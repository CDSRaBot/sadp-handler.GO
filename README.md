# SADP-handler

SADP-handler is a planned Go command-line utility for discovering, activating, and configuring Hikvision devices over the Search Active Device Protocol (SADP).

## Project status

The repository currently contains design documentation; Go implementation has not started. The local Python reference is documented as a behavioral baseline, while protocol compatibility gates remain open. Generic HTTP/JSON starter files were removed because they did not implement the specified SADP system. See the system specification for reference behavior and unresolved protocol details.

## Design

- **Go module path**: `github.com/CDSRaBot/sadp-handler.GO`
- **Target Go version**: Go 1.22+
- **Discovery**: SADP over the local network, with interface-aware UDP transport
- **XML**: Go standard library `encoding/xml`
- **Concurrency**: `context.Context`, goroutines, and channels where appropriate
- **Testing**: standard `testing` package, `go test`, and `go vet`

## Documentation

- [Development lifecycle guide](./docs/DEVELOPMENT_GUIDE.md)
- [System specification](./docs/spec/SYSTEM.md)
- [Module specifications](./docs/spec/modules/)
- [Procedure specifications](./docs/spec/procedures/)
- [Russian documentation](./docs/ru/)

## Implementation status

All repository filesystem paths in documentation, configuration, and build instructions are relative to the project root. Go package imports use the module path above. Do not treat the former starter-code layout or its placeholder APIs as implementation requirements. The Python reference's behavior is a local compatibility target, not an authoritative protocol specification; review the open gates in `docs/spec/SYSTEM.md` before claiming SADP compatibility.
