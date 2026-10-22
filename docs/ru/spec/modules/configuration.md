# Спецификация модуля: M7 — Конфигурация

## 1. Назначение

Загружать внешний `settings.json`, проверять каждую настройку, применять документированные значения по умолчанию и передавать приложению одну типизированную конфигурацию. Файл необязателен: при его отсутствии используются те же значения, что в шаблоне. Нечитаемый, некорректный или невалидный существующий файл является явной ошибкой запуска, а не поводом молча перейти к defaults.

`settings.json` — единый источник базовых настроек. Параметры командной строки имеют более высокий приоритет и не записываются обратно в файл.

## 2. Формат

Файл является валидным JSON и содержит `_comments` и группы `ui`, `localization`, `network`, `discovery`, `logging`. Так как стандартный JSON не имеет комментариев, `_comments` — обязательные метаданные документации: для каждого ключа настройки указывается описание допустимых значений и границ. Неизвестные ключи являются ошибкой. Шаблон: [`settings.json`](../../../settings.json).

## 3. Контракт

```go
type Settings struct {
    UI UISettings
    Localization LocalizationSettings
    Network NetworkSettings
    Discovery DiscoverySettings
    Logging LoggingSettings
}

func Load(path string) (Settings, error)
func Defaults() Settings
```

```go
type ConfigurationErrorKind string

const (
    ConfigurationReadError               ConfigurationErrorKind = "read"
    ConfigurationDecodeError             ConfigurationErrorKind = "decode"
    ConfigurationUnknownKeyError         ConfigurationErrorKind = "unknown_key"
    ConfigurationMissingCommentError     ConfigurationErrorKind = "missing_comment"
    ConfigurationInvalidValueError       ConfigurationErrorKind = "invalid_value"
    ConfigurationInvalidCombinationError ConfigurationErrorKind = "invalid_combination"
)

type ConfigurationError struct {
    Kind  ConfigurationErrorKind
    Field string
    Err   error
}
```

`Load("")` читает `settings.json` из текущего каталога. При отсутствии файла возвращаются defaults; при ошибке чтения, JSON или валидации возвращается ошибка без пригодной частичной конфигурации.

| Ключ | По умолчанию | Допустимые значения |
|---|---:|---|
| `ui.mode` | `yes` | `yes` или `no` |
| `ui.format` | `text` | `text` или `json` |
| `ui.language` | `en` | непустой код локали M5 |
| `ui.once` | `false` | boolean; `true` только для CLI |
| `localization.directory` | `locales` | непустой относительный путь |
| `network.multicast_group` | `239.255.255.250` | multicast IPv4 |
| `network.multicast_port` | `37020` | целое `1..65535` |
| `network.multicast_ttl` | `64` | целое `1..255` |
| `network.max_sadp_datagram_size` | `1500` | целое `1..65507`; ноль означает default |
| `discovery.probe_interval` | пусто | пусто/ноль для bounded или положительная duration Go для continuous |
| `discovery.cycle_timeout` | `5s` | пусто/ноль для continuous или положительная duration Go для bounded |
| `discovery.repeat_count` | `1` | `0` для continuous или целое `1..100` для bounded |
| `logging.level` | `info` | `debug`, `info`, `warn`, `error` |

Приоритет: defaults → `settings.json` → глобальные CLI-флаги → аргументы команды. Секреты и payload в settings не допускаются и не журналируются.

Duration принимаются только как JSON-строки Go (`"5s"`, `"60s"`), разбираются
через `time.ParseDuration`; пустая строка означает ноль, числовые JSON-значения
недопустимы. `logging.level` M8 преобразует в уровень `slog`.
