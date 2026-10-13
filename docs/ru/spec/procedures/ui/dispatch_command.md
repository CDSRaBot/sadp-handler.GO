# Спецификация процедуры: P4.5 — DispatchCommand
## Модуль: M4 — Интерфейс командной строки

## 1. Назначение

Определить общий типизированный каталог команд, границу диспетчеризации и оболочку результата для командной консоли TUI, CLI REPL и CLI one-shot. Процедура маршрутизирует проверенные запросы приложения; она не отвечает за разбор ввода терминала или реализацию протокола.

## 2. Контракт Go

```go
type Command interface {
    isCommand()
}

type DiscoverCommand struct {
    InterfaceIndex *int
    Policy         DiscoveryPolicy
}

type DiscoveryPolicy struct {
    ProbeInterval time.Duration
    CycleTimeout  time.Duration
    RepeatCount   int
}

type ActivateCommand struct {
    Targets       TargetSelector
    PasswordInput PasswordSource
    Email         string
    Confirm       bool
}

type NetworkCommand struct {
    Targets       TargetSelector
    PasswordInput PasswordSource
    Config        NetworkConfig
    Confirm       bool
}

type LanguageCommand struct {
    Action LanguageAction
    Code   string
}

type HelpCommand struct{}
type VersionCommand struct{}
type QuitCommand struct{}
type ExportLocaleTemplateCommand struct {
    Destination string
}

type CommandResult struct {
    Command   string             `json:"command,omitempty"`
    OK        bool               `json:"ok"`
    Exit      bool               `json:"exit,omitempty"`
    Message   string             `json:"message,omitempty"`
    Discovery []DiscoveredDevice `json:"discovery,omitempty"`
    Devices   []DeviceOutcome    `json:"devices,omitempty"`
    Locales   []string           `json:"locales,omitempty"`
    Version   string             `json:"version,omitempty"`
    Error     *CommandFailure    `json:"error,omitempty"`
}

type TargetSelector struct {
    MACs              []net.HardwareAddr
    UseTUISelection   bool
    UseFocusedDevice  bool
}

type PasswordSource uint8
const (
    PasswordFromPrompt PasswordSource = iota + 1
    PasswordFromStdin
)

type LanguageAction uint8
const (
    LanguageList LanguageAction = iota + 1
    LanguageSwitch
)

type DiscoveredDevice struct {
    MAC             string    `json:"mac"`
    Sender          string    `json:"sender"`
    IPv4            string    `json:"ipv4"`
    Model           string    `json:"model,omitempty"`
    Activated       bool      `json:"activated"`
    SoftwareVersion string    `json:"software_version,omitempty"`
    LastSeen        time.Time `json:"last_seen"`
}

type DeviceOutcome struct {
    MAC    string `json:"mac"`
    Sender string `json:"sender,omitempty"`
    Status string `json:"status"`
    Error  string `json:"error,omitempty"`
}

type CommandFailure struct {
    Code    string `json:"code"`
    Message string `json:"message"`
}

func DispatchCommand(
    ctx context.Context,
    command Command,
    dependencies Dependencies,
) (CommandResult, error)
```

`NetworkConfig` — типизированный контракт M3, а не отдельная модель M4. `Dependencies` предоставляет сервисы приложения для диспетчеризации, включая менеджер фонового сеанса обнаружения TUI. Варианты команд закрыты внутри пакета M4; их создают оба парсера. `DispatchCommand` — единственная общая граница маршрутизации команд в рабочие процессы.

- `PasswordFromPrompt` используется для маскированного запроса TUI; `PasswordFromStdin` — для CLI `--password-stdin`. Ни один источник не читается до проверки команды, целей, подтверждения и protocol gate.
- `LanguageList` перечисляет доступные локали; `LanguageSwitch` использует точное значение `Code`. Переключение не сохраняется в настройках.
- `TargetSelector` должен задавать ровно одну стратегию: один или несколько явных `MACs`, `UseTUISelection` либо `UseFocusedDevice`. Отклонять пустые и конфликтующие селекторы; перед вводом любого секрета проверить наличие выбранных устройств.

- Непрерывная `DiscoveryPolicy` имеет `ProbeInterval > 0`, `CycleTimeout == 0` и `RepeatCount == 0`.
- Ограниченная `DiscoveryPolicy` имеет `ProbeInterval == 0`, `CycleTimeout > 0` и `RepeatCount` в диапазоне `1..100`.
- Отклонять смешанные политики и политики без режима. CLI использует ограниченную политику; TUI — непрерывную с интервалом probe 60 секунд.

## 3. Диспетчеризация и выбор целей

- Отклонять nil-контекст, nil-команду, неверные типизированные поля и некорректную конфигурацию зависимостей до побочных действий.
- `DiscoverCommand` вызывает M6 с выбранной политикой. TUI управляет не более чем одним непрерывным сеансом: начинает его сразу, продолжает приём и отправляет новый probe каждые 60 секунд до отмены. Диспетчеризация TUI запускает или переключает этот фоновый сеанс без блокировки цикла событий и возвращает подтверждение; события обнаружения перед обновлением сборщика передаются в цикл событий UI. Повторный выбор уже активного интерфейса не создаёт второй сеанс; при выборе другого интерфейса прежний сеанс отменяется и завершается до старта нового. CLI немедленно запускает первый ограниченный цикл; в каждом цикле отправляется probe и выполняется приём в течение полного timeout, затем сразу запускается следующий цикл. Получение устройства не сокращает цикл.
- Если в команде TUI отсутствует индекс интерфейса, получить его из явно выбранного интерфейса до диспетчеризации; отсутствие выбора — понятная ошибка. Для CLI индекс обязателен при разборе.
- Для команд изменения данных в TUI явный MAC задаёт устройство-цель. Иначе выбрать все отмеченные устройства; если выбор пуст — устройство в фокусе. До запроса пароля или диспетчеризации изменения отклонять пустой/недоступный набор целей.
- Для команд изменения данных в CLI найти в текущем списке все устройства с указанными MAC. Неизвестную или неоднозначную цель отклонять; не пропускать цели молча. Сохранять порядок аргументов в результатах по устройствам.
- Требовать явное подтверждение для двух и более целей изменения. TUI показывает подтверждение с точным набором целей; CLI требует `--yes` для нескольких целей и не запрашивает подтверждение в one-shot.
- Сохранять один `DeviceOutcome` на каждую запрошенную цель со статусом и безопасным описанием ошибки. Не сводить частичный результат пакета к общему успеху. `CommandResult.OK` равен true только при успехе команды для всех запрошенных целей.
- `help`, `version`, `quit`, `language` и экспорт шаблона направляются соответствующему сервису M4/M5. `quit` управляет завершением REPL; в one-shot завершается успешно и не запускает другую команду.
- Пока protocol gate операций управления устройствами не снят, `ActivateCommand` и `NetworkCommand` возвращают `CommandFailure{Code:"unsupported"}` до получения секретов, чтения пароля, запроса подтверждения или открытия сокета.
- Использовать стабильные коды ошибок, включая `invalid_argument`, `unsupported`, `operation_failed` и `canceled`. Очищать `Message`: не раскрывать секреты, исходные данные пакета или чувствительное содержимое файлов.

## 4. Агрегация результатов обнаружения

- M6 выдаёт проверенное событие объявления для каждого принятого пакета; он не выводит корреляцию UUID и не подавляет необработанные протокольные события.
- Общий сборщик M4 строит видимый список по уникальному ключу `(нормализованный MAC, сетевой адрес отправителя)`. Повторное объявление обновляет последние проверенные поля и время последнего обнаружения, не создавая дубликат строки.
- CLI `discover` возвращает конечный снимок, отсортированный по нормализованному MAC, затем по адресу отправителя. Отсутствие устройств — успешный пустой результат. TUI обновляет список при поступлении событий и сохраняет выделение устройств, которые остаются в списке.
- Ключ агрегации — политика представления/приложения, а не протокольная идентичность или гарантия корреляции ответов.

## 5. Политика вывода и процесса

- Представлять `CommandResult` как текст или JSON согласно процессному CLI-параметру `--format`. В JSON на каждую завершённую команду приходится один объект; REPL выдаёт по одному объекту на строку в stdout. Приглашение/ввод REPL и диагностические сообщения всегда направляются в stderr.
- Ошибки проверки команды и рабочего процесса отражаются в `CommandResult`, а в JSON остаются машиночитаемыми в stdout. Диагностика не содержит паролей, исходных данных пакетов или секретных полей. Не выдавать для одной команды второй JSON-объект.
- В REPL ошибка разбора/токенизации также выдаётся одним результатом с `OK=false`, `Error.Code="invalid_argument"` и без поля `Command`; REPL продолжает работу. Ошибка разбора one-shot завершается кодом 2 и тем же JSON-конвертом, если был запрошен JSON-вывод.
- В режиме REPL завершённая команда с ошибкой не завершает процесс. В one-shot успех соответствует коду 0, неверный запуск — 2, операционная ошибка/`unsupported` — 1, прерывание — 130.
- Никогда не журналировать историю команд с секретами, исходный XML, полные данные устройств или значения вроде `Salt`.

## 6. Ошибки и тесты

Возвращать ошибку только если диспетчеризация не может сформировать корректный результат команды (например, нарушен внутренний контракт команды/зависимостей); ожидаемые ошибки команды/рабочего процесса представлять в `CommandResult` с `OK=false` и безопасным `CommandFailure`. Сохранять исходные ошибки для внутренней обработки, не раскрывая чувствительные значения.

Проверять каждый вариант команды, некорректные типизированные запросы, разрешение и неоднозначность целей, дубликаты нормализованных целей, приоритет выделения над фокусом, подтверждение для нескольких целей, частичные результаты по устройствам, остановку protocol gate до ввода пароля/сетевого доступа, агрегацию и детерминированную сортировку обнаружения, пустой результат обнаружения, маршрутизацию M5, форму JSON, очистку секретов и сопоставление кодов завершения.

---

*Атомарная спецификация процедуры уровня 2.*
