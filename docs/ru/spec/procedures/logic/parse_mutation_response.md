# Спецификация процедуры: P3.7 — ParseMutationResponse
## Модуль: M3 — Логика устройств

## 1. Назначение

Интерпретировать один ответ на мутацию активации или сетевых параметров
после того, как M1 получил исходные байты и метаданные отправителя. Вернуть
явное решение «принято/отклонено» либо типизированную ошибку.

Процедура задаёт семантический контракт ответа. Имена wire-элементов,
значения статусов и обязательные поля являются нормативными только при
наличии в утверждённых fixture для соответствующей операции. Неизвестные
или недокументированные значения никогда не означают успех.

## 1.1 Грамматика ISAPI, подтверждённая SDK

Руководство Hikvision Device Network SDK Information Release описывает
отличный от raw SADP UDP mutation HTTP-интерфейс ISAPI для управления SADP
терминала:

| Операция | Метод и URI | Ответ при успехе | Ответ при ошибке |
| --- | --- | --- | --- |
| Получение возможностей SADP терминала | `GET /ISAPI/Publish/TerminalMgr/terminals/discovery/capabilities` | `XML_Cap_DiscoveryMode` | `XML_ResponseStatus` |
| Получение SADP-параметров терминала | `GET /ISAPI/Publish/TerminalMgr/terminals/discovery/<ID>` | `XML_DiscoveryMode` | `XML_ResponseStatus` |
| Установка SADP-параметров терминала | `PUT /ISAPI/Publish/TerminalMgr/terminals/discovery` | `XML_ResponseStatus` | `XML_ResponseStatus` |

Соответствующие схемы запроса и ответа:

- `DiscoveryMode` с пространством имён
  `http://www.isapi.org/ver20/XMLSchema` и версией `2.0`;
- `DiscoveryModeCfg` с теми же пространством имён и версией;
- `ResponseStatus` с полями `requestURL`, `statusCode`, `statusString`,
  `subStatusCode`, необязательными `MErrCode` и `MErrDevSelfEx`.

Документированные поля конфигурации обнаружения: `ONVIFDiscoveryMode`,
`ISDiscoveryMode`, `PcapMode` и `SADP`; допустимые значения — `discoverable`
и `nonDiscoverable`. `DiscoveryModeCfg` дополнительно содержит селектор
операции и списки терминалов/организаций.

Таблица функциональных ошибок содержит следующие связанные с активацией
сопоставления:

| `MErrCode` | `subStatusCode` | Значение |
| --- | --- | --- |
| `0x01400004` | `notActivated` | Устройство не активировано |
| `0x01400005` | `hasActivated` | Устройство активировано |
| `0x01400006` | `forbiddenIP` | IP-адрес заблокирован |
| `0x01400007` | `bondMacAddressNotMatch` | MAC-адрес не связан с пользователем |

Эти сопоставления нормативны для документированной грамматики ответов
ISAPI/Text Protocol. Они не доказывают, что те же поля или коды присутствуют
в raw SADP UDP-ответе.

**Источник:** Hikvision *Device Network SDK (Information Release) Developer
Guide V6.1.7.X*, разделы `/ISAPI/Publish/TerminalMgr/terminals/discovery`,
`XML_DiscoveryMode`, `XML_DiscoveryModeCfg`, `XML_ResponseStatus` и *Error
Codes Categorized by Functional Modules*.

## 2. Контракт Go

```go
type MutationOperation string

const (
    MutationActivation MutationOperation = "activation"
    MutationNetwork    MutationOperation = "network"
)

type MutationDecisionStatus string

const (
    MutationAccepted MutationDecisionStatus = "accepted"
    MutationRejected MutationDecisionStatus = "rejected"
)

type MutationDecision struct {
    Operation MutationOperation
    Target    MutationTarget
    Status    MutationDecisionStatus
    Reason    string
}

type ResponseParser interface {
    Parse(MutationOperation, MutationTarget, net.Addr, []byte) (MutationDecision, error)
}
```

## 3. Семантический контракт

- До разбора проверить операцию, цель, отправителя и размер ответа.
- Разбирать только грамматику, утверждённую для выбранной операции. Не
  принимать поля активации в ответе на настройку сети и наоборот.
- Проверить, что ответ идентифицирует запрошенную цель, а отправитель
  соответствует правилу корреляции для этой цели.
- Возвращать `MutationAccepted` только для явно документированного статуса
  успеха при наличии и валидности всех обязательных полей.
- Возвращать `MutationRejected` только для явно документированного отказа
  устройства.
- Возвращать ошибку при неизвестном статусе, отсутствии обязательных полей,
  повреждённых байтах, конфликтующих дубликатах статуса, неожиданной
  операции или несоответствии корреляции.
- Сохранять цель в каждом решении. `Reason` — ограниченная безопасная
  сводка, а не копия ответа.

## 4. Контракт ошибок

Возвращать `DeviceOperationError` со стабильными типами:

- `invalid_input`;
- `malformed_response`;
- `response_mismatch`;
- `device_rejected` — только если синтаксически корректный ответ содержит
  документированный статус отказа;
- `missing_data` — если отсутствуют обязательные семантические поля;
- `timeout` или `canceled` — только если парсер является частью workflow с
  контекстом, а workflow сообщает соответствующую причину транспорта.

Парсер — чистое преобразование: он не пишет логи, не выполняет сетевой ввод-
вывод, повторы или доступ к глобальному состоянию. Он никогда не возвращает
успешное решение с нулевыми значениями.

## 5. Пакетные операции и безопасность

Владеющий workflow вызывает парсер отдельно для каждой запрошенной цели.
Он сохраняет результат `DeviceOutcome` для каждого входного устройства и
порядок входа. Ошибка парсера для одного устройства не должна удалять уже
сопоставленные результаты остальных. Workflow может агрегировать ошибки
только после записи результата каждого устройства.

Исходные байты ответа, XML, пароли, challenge, хеши и полные поля
безопасности запрещено включать в ошибки, логи, вывод CLI/TUI и сообщения
об ошибках тестов.

## 6. Требования к проверкам

Тесты по этой процедуре должны охватывать успешные ответы активации и
настройки сети, явный отказ, повреждённые и неожиданные ответы, отсутствие
полей, конфликтующие поля, несоответствие отправителя/цели, дубликаты целей,
частичные пакетные результаты и редактирование секретов. Каждая fixture
использует синтетические секреты и задаёт ожидаемое семантическое решение.

---

*Спецификация процедуры уровня 2.*
