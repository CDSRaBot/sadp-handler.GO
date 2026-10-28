# Аудит контракта workflow мутаций

## Статус

Аудит Issue #36 завершён. Raw SADP UDP активация и настройка сети остаются
отключёнными до описания контракта ответа и покрытия его детерминированными
fixture. Проверка SDK закрывает документированный контракт ISAPI управления
SADP терминала; грамматика ответа raw SADP UDP остаётся отдельной открытой
границей. Issue #40 определяет семантические контракты; Issue #45 должна
сформировать и утвердить исполняемые fixture до начала production-реализации
raw SADP.

## Текущая цепочка контрактов

1. M4 разбирает `ActivateCommand` или `NetworkCommand` и определяет цели.
2. M3 проверяет входные данные и формирует один XML payload запроса:
   - `BuildActivationXML`;
   - `BuildNetworkXML`.
3. M1 `SendSADPDatagram` отправляет один payload и не ожидает и не
   интерпретирует ответ.
4. M4 пока получает результаты только через внедрённые интерфейсы
   `ActivationService` и `NetworkService`.

Цепочка намеренно не завершена для реальных workflow мутации: у шагов 3 и 4
нет специфицированной границы получения ответа и корреляции.

## Результаты аудита

### Активация

- Форма XML-запроса описана в `build_activation.md`.
- Нет исполняемого контракта успеха, отказа, повреждённого ответа,
  тайм-аута или корреляции ответа с целью.
- Нет fixture парсера, определяющей поля ответа или допустимые статусы.
- Включение production-сервиса потребовало бы предположений о поведении
  устройства.

### Настройка сети

- `BuildNetworkXML` требует непустой challenge и корректно прекращает работу
  при его отсутствии.
- Нет исполняемого контракта получения challenge, извлечения значения,
  привязки к MAC цели или истечения срока.
- Нет fixture парсера для успеха, отказа или повреждённого ответа настройки
  сети.
- Fallback на пароль открытым текстом явно запрещён.

### Управление SADP терминала через ISAPI согласно SDK

Руководство Information Release документирует:

- `GET /ISAPI/Publish/TerminalMgr/terminals/discovery/capabilities`;
- `GET /ISAPI/Publish/TerminalMgr/terminals/discovery/<ID>`;
- `PUT /ISAPI/Publish/TerminalMgr/terminals/discovery`;
- XML-корни `DiscoveryMode` и `DiscoveryModeCfg`;
- грамматику ответа `XML_ResponseStatus`;
- функциональные коды `notActivated`, `hasActivated`, `forbiddenIP` и
  `bondMacAddressNotMatch`.

Этого достаточно для отдельного парсера и набора fixture ISAPI. Эти сведения
не задают ответ raw SADP UDP. URI `TerminalMgr` — HTTP endpoint ISAPI, хотя
его назначение связано с настройкой SADP.

### Транспорт и жизненный цикл

- M1 может отправить управляющую датаграмму, но API отправки не ожидает ответ
  и не задаёт семантику корреляции.
- Любая реализация workflow должна использовать `context.Context`,
  ограниченные deadline, явное владение сокетом и отсутствие неограниченных
  повторов.
- Пароли, challenge, хеши, XML и packet payload нельзя записывать в логи или
  возвращать в пользовательских ошибках.

## Обновление контракта

Недостающие границы workflow описаны в:

- [P1.6 MutationTransport.Exchange](../network/exchange_mutation.md);
- [P3.6 AcquireChallenge](./acquire_challenge.md);
- [P3.7 ParseMutationResponse](./parse_mutation_response.md);
- [каталоге fixture мутаций](./mutation_fixtures.md).

Спецификации модулей [M1](../../modules/network.md) и
[M3](../../modules/logic.md), а также gate реализации диспетчеризации M4
ссылаются на эти границы. Контракты задают семантические результаты и правила
корреляции, не изобретая неподтверждённые wire-поля. Подтверждённые SDK поля
и статусы ISAPI можно использовать в ISAPI fixture. Для raw SADP UDP fixture
по-прежнему должны установить грамматику ответа конкретной операции.

## Обязательные работы в режиме test-first

До включения production-кода workflow Issue #45 должна определить и
протестировать:

```text
ChallengeProvider:
  acquire(ctx, target) -> challenge or typed failure

MutationTransport:
  exchange(ctx, target, payload) -> raw response + sender metadata

ResponseParser:
  parse(operation, target, sender, response) -> accepted/rejected outcome
```

Приведённые сигнатуры Go являются нормативными интерфейсами. Сопоставление
wire-полей для конкретных операций остаётся основанным на fixture.
Реализация должна возвращать один `DeviceOutcome` на каждую запрошенную цель,
сохранять частичные результаты и преобразовывать сбои в
`DeviceOperationError`.

## Критерии приёмки

Работа над workflow мутаций не завершена, пока детерминированные тесты не
покрывают:

- успешную активацию и настройку сети;
- отказы устройств;
- повреждённые и неожиданные ответы;
- отсутствие challenge;
- тайм-аут и отмену контекста;
- несовпадение отправителя и цели;
- частичные результаты пакетной операции;
- редактирование секретов и payload.
