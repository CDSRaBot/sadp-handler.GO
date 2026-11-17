# SADP-handler

SADP-handler is a Go terminal utility for discovering Hikvision devices over SADP, with device activation and configuration workflows in development.

## Project status

The Go implementation includes discovery and protocol components. SADP activation is disabled; the intended ISAPI activation and network configuration workflows are not yet wired into the executable. Real-device and platform compatibility gates remain open. See the [beta plan](./docs/BETA_PLAN.md) and system specification.

## Design

- **Go module path**: `github.com/CDSRaBot/sadp-handler.GO`
- **Target Go version**: Go 1.22+
- **Discovery**: SADP over the local network, with interface-aware UDP transport
- **XML**: Go standard library `encoding/xml`
- **Concurrency**: `context.Context`, goroutines, and channels where appropriate
- **Testing**: standard `testing` package, `go test`, and `go vet`

## Documentation

- [Command help (English)](./docs/HELP.md)
- [Справка по командам (русский)](./docs/ru/HELP.md)
- [Диаграмма работы и workflow (русский)](./docs/ru/WORKFLOW.md)
- [Development lifecycle guide](./docs/DEVELOPMENT_GUIDE.md)
- [System specification](./docs/spec/SYSTEM.md)
- [External settings template](./settings.json)
- [Module specifications](./docs/spec/modules/)
- [Procedure specifications](./docs/spec/procedures/)
- [Russian documentation](./docs/ru/)

## Implementation status

All repository filesystem paths in documentation, configuration, and build instructions are relative to the project root. Go package imports use the module path above. Do not treat the former starter-code layout or its placeholder APIs as implementation requirements. The Python reference's behavior is a local compatibility target, not an authoritative protocol specification; review the open gates in `docs/spec/SYSTEM.md` before claiming SADP compatibility.
