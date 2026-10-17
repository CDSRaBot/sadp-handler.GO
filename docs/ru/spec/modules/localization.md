# Спецификация модуля: M5 — Локализация

## 1. Назначение

Загружать, проверять, разрешать и атомарно переключать каталоги локалей независимо от отображения терминала и разбора команд.

## 2. Обязанности

- Считать встроенный `locales/en.json` каноническим набором ключей сообщений и английским резервным каталогом.
- Проверять идентификаторы локалей и плоские JSON-каталоги.
- Загружать запрошенный каталог и дополнять отсутствующие переводы английскими сообщениями.
- Разрешать приоритет локали при запуске, перечислять доступные внешние локали и экспортировать шаблон английского каталога.
- Заменять активные код локали и каталог атомарно и только после полной загрузки кандидата.

M5 не разбирает параметры CLI, не отображает пользовательские строки, не сохраняет переключение локали во время работы и не изменяет каталоги после их публикации.

## 3. Контракт API Go

```go
var ErrNoExternalLocales = errors.New("no external locales")
var ErrCatalogTooLarge = errors.New("locale catalog exceeds 1 MiB")
```

Вызывающая сторона распознаёт ожидаемое состояние «только встроенный английский» и превышение размера каталога через `errors.Is`. Оборачивать sentinel контекстом операции и сохранять его.

Пути пакетов будут окончательно определены при реализации. Контракты задаются процедурами:

```go
func ValidateLocaleCode(langCode string) error
func DecodeLocaleCatalog(data []byte) (map[string]string, error)
func MergeLocaleCatalog(english, translated map[string]string) (map[string]string, error)
func LoadLocale(filesystem fs.FS, langCode string, english map[string]string) (map[string]string, error)
func ResolveLocale(settingsLocale, cliLocale string, hasSettings, hasCLI bool) (string, error)
func ListLocales(filesystem fs.FS) ([]string, error)
func ExportLocaleTemplate(english map[string]string, destination string) error
func SwitchLocale(active *atomic.Pointer[LocaleState], candidate LocaleState) error

type LocaleState struct {
    Code     string
    Messages map[string]string
}
```

- `LoadLocale` объединяет проверку кода, ограниченное чтение/декодирование каталога и слияние; активное состояние приложения не меняет.
- `SwitchLocale` создаёт защитную копию и публикует одно полное неизменяемое состояние через `atomic.Pointer`. Читатели загружают один указатель состояния и не изменяют его карту.
- Английский каталог передаётся из канонической встроенной локали; внешний `en.json` во время работы не требуется.
- Выбор и перечисление локалей, а также экспорт шаблона возвращают явные ошибки. Отсутствующий или некорректный запрошенный каталог не считается успешным переходом на английский.

## 4. Поток данных и политика ошибок

1. `ResolveLocale` выбирает эффективный код из значений по умолчанию (английский), настроек и переопределения CLI. Проверяется только эффективное значение; неверная локаль с более низким приоритетом, переопределённая корректным значением, не препятствует запуску.
2. `LoadLocale` возвращает полную независимую карту либо ошибку без карты.
3. При запуске активное состояние инициализируется проверенным английским каталогом. При переключении во время работы сначала загружается кандидат, затем вызывается `SwitchLocale`.
4. Неудачная загрузка или переключение оставляет активное состояние неизменным.
5. Неполные неанглийские каталоги допустимы: отсутствующие сообщения берутся из английского. Некорректные присутствующие записи вызывают ошибку.

Не журналировать содержимое каталогов или недоверенные переведённые строки. Ошибки указывают операцию и локаль/путь, не раскрывая содержимое каталога. Не использовать `panic` для ошибок каталогов или обычных ошибок файловой системы.

## 5. Индекс атомарных процедур

- [P5.1 LoadLocale](../procedures/i18n/load.md)
- [P5.2 ValidateLocaleCode](../procedures/i18n/validate_locale_code.md)
- [P5.3 DecodeLocaleCatalog](../procedures/i18n/decode_catalog.md)
- [P5.4 MergeLocaleCatalog](../procedures/i18n/merge_catalog.md)
- [P5.5 ResolveLocale](../procedures/i18n/resolve_locale.md)
- [P5.6 ListLocales](../procedures/i18n/list_locales.md)
- [P5.7 ExportLocaleTemplate](../procedures/i18n/export_template.md)
- [P5.8 SwitchLocale](../procedures/i18n/switch_locale.md)

---

*Спецификация модуля уровня 1.*
