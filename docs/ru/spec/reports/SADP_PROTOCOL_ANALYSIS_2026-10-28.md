# SADP: сводный анализ протокола и рабочих процедур

**Дата анализа:** 12.11.2026 (уточнено по трассам от 11–12.11.2026)
**Статус:** исследовательский отчёт; подтверждённые wire-факты отделены от
гипотез и незавершённых реконструкций  
**Объект:** Hikvision SADP SDK 4.6.0.151; исходная трасса —
DS-2CD1021-I, прошивка V5.5.801 build 210701. Дополнительно учтены
трассы от 11.11.2026 с несколькими моделями и прошивками.

## 1. Источники и границы достоверности

Использованы:

- сетевой перехват
  `SADP-Activation-Net-DHCP-HC.txt` (локальный материал, не включён в репозиторий);
- лог SADP SDK
  `SdkLog_20261028_214108754_w.log` (локальный материал, не включён в репозиторий);
- лог зависимого OTAPDCM SDK
  `OTAPDCM_20261028_214108928_w.log` (локальный материал, не включён в репозиторий);
- текущая Go-спецификация SADP/ISAPI
  [SADP_ISAPI_TRUSTED_PIPELINES.md](../procedures/SADP_ISAPI_TRUSTED_PIPELINES.md);
- текущая реализация сетевого транспорта
  [mutation_transport.go](../../../../internal/network/mutation_transport.go);
- текущая реализация XML-билдеров
  [build_activation_xml.go](../../../../internal/logic/build_activation_xml.go);
- текущий SADP-парсер
  [parser.go](../../../../internal/parser/parser.go).
- дополнительные сетевые трассы и логи SDK из
  [SadpLog.zip](../../../../../SadpLog.zip).
- сетевой перехват успешной активации и последующих изменений IP
  `SADP-Activation success + IP modification.txt` (локальный материал, не включён в репозиторий)
  (3479 строк; устройство MAC `08-3b-c1-21-15-16`);
- лог SDK для этого сеанса
  [SdkLog_20261112_174917417_w.log](../../../../../SdkLog_20261112_174917417_w.log).

Сетевой перехват подтверждает фактические сообщения, которые были
переданы по сети. Логи SDK дополнительно показывают исходный XML до
бинарной упаковки, внутренний выбор интерфейса и порядок вызовов SDK.
Лог не является полной спецификацией: он содержит только операции,
выполненные в конкретном сеансе.

Секреты, пароли, ответы на контрольные вопросы и ciphertext намеренно не
воспроизводятся в отчёте.

## 2. Краткий итог

SADP в исследованном сеансе — самостоятельный локальный UDP-протокол:

| Свойство | Подтверждённое значение |
|---|---|
| Основной IPv4 multicast | `239.255.255.250:37020` |
| IPv6 multicast | `FF02::C:37020` |
| Локальный IPC/multicast SDK | `239.255.255.251:47020` на loopback |
| Формат полезной нагрузки | XML `Probe`/`ProbeMatch`, иногда внутри бинарного заголовка |
| Идентичность устройства | MAC, например `94-e1-ac-dd-a3-6f` |
| Идентификатор операции | UUID сообщения |
| Тип операции | `<Types>` |
| Подтверждённые типы в новых трассах | `inquiry`, `exchangecode`, `activate`, `getsecurityquestion`, `SetVerificationCode`, `SetHCPlatform`, `SetMailBox` |
| Интерфейсы SDK в новых трассах | `192.168.10.102`, `0.0.0.0` и loopback-копия `127.0.0.1` |

Главные инженерные выводы:

1. IP-адрес устройства изменяем и не может быть постоянным ключом
   корреляции. Главным ключом является MAC.
2. Одна команда может породить несколько UDP-сообщений: непосредственный
   ответ, `update`, `inquiry`, дубликат через loopback.
3. Защищённые команды используют `<Password>`. Для сетевого `update` в
   профиле SDK 4.4.1.151 подтверждены `PWErrorParse=true` и
   `bSalt="true"`. Live x86-инструментирование показало AES-128-ECB
   преобразование отдельных блоков; точный вывод AES-ключа по-прежнему не
   восстановлен.
4. После `success` требуется проверка post-condition через последующее
   объявление устройства.
5. DHCP имеет промежуточные состояния с `0.0.0.0` и `169.254.x.x`.
6. Активация, контрольные вопросы и Hik-Connect используют отдельные
   SADP-типы и отдельный этап `exchangecode`.

## 3. Подготовка SADP-сеанса

### 3.1. Инициализация SDK

SDK загружает:

- `wpcap.dll`;
- `HCNetSDK.dll`;
- `HCOTAPDCM.dll`;
- `libssl-3-x64.dll`;
- `libcrypto-3-x64.dll`.

Версия SADP SDK:

```text
HCSADPSDK 4.6.0.151 build20261010
```

Для каждого обнаруженного интерфейса SDK пытается создать:

1. IPv4 multicast socket;
2. IPv6 multicast socket;
3. локальный IPv4 multicast socket;
4. subnet UDP socket;
5. при возможности pcap/Ethernet канал.

Интерфейсы без IPv4 или с недоступным pcap могут завершить отдельную
инициализацию ошибкой, но это не останавливает работу на пригодном
интерфейсе.

### 3.2. Подтверждённый выбор интерфейса

Рабочий интерфейс:

```text
Adapter: USB2.0 Ethernet Adapter
IPv4:   192.168.1.250
MAC:    c8-17-f5-fa-71-42
```

Внутренние ошибки для Bluetooth, Wi-Fi Direct и других интерфейсов не
должны автоматически превращаться в ошибку всей операции, если хотя бы
один пригодный IPv4-канал успешно запущен.

### 3.3. Схема подготовки

```mermaid
flowchart TD
    A[Запуск SADP SDK] --> B[Загрузка WinPcap и crypto-библиотек]
    B --> C[Перебор локальных интерфейсов]
    C --> D{Есть IPv4 и доступен канал?}
    D -- нет --> E[Записать per-interface ошибку]
    D -- да --> F[Создать IPv4 multicast socket]
    F --> G[Создать IPv6/local/subnet каналы при возможности]
    G --> H{Есть рабочий IPv4 SADP-канал?}
    H -- нет --> X[Ошибка запуска]
    H -- да --> I[Сеанс готов]
```

## 4. Общая структура SADP-сообщения

На логическом уровне запрос имеет форму:

```xml
<?xml version="1.0" encoding="utf-8"?>
<Probe>
  <Uuid>operation-uuid</Uuid>
  <Types>operation-type</Types>
  ...
</Probe>
```

Ответ имеет форму:

```xml
<?xml version="1.0" encoding="UTF-8"?>
<ProbeMatch>
  <Uuid>response-uuid</Uuid>
  <Types>response-type</Types>
  <Result>success</Result>
  ...
</ProbeMatch>
```

Для обычного объявления устройства присутствуют:

```xml
<MAC>94-e1-ac-dd-a3-6f</MAC>
<IPv4Address>192.168.1.64</IPv4Address>
<IPv4SubnetMask>255.255.255.0</IPv4SubnetMask>
<IPv4Gateway>192.168.1.1</IPv4Gateway>
<DHCP>false</DHCP>
<CommandPort>8000</CommandPort>
<HttpPort>80</HttpPort>
<Activated>true</Activated>
```

Также встречаются capability/state-поля:

```xml
<Encrypt>true</Encrypt>
<SupportSecurityQuestion>true</SupportSecurityQuestion>
<SupportHCPlatform>true</SupportHCPlatform>
<HCPlatformEnable>true</HCPlatformEnable>
<PasswordResetAbility>true</PasswordResetAbility>
<DeviceLock>true</DeviceLock>
<Salt>...</Salt>
```

Во всех проверенных XML-ответах поле `<Activated>` присутствует и содержит
булево значение `true` или `false`. Различие поколений устройств по наличию
или отсутствию этого поля не выявлено; состояние активации следует
извлекать из него независимо от модели и поколения firmware.

`Salt` является полем объявления устройства. В проверенных трассах он имеет
вид 64 hex-символов, повторяется для одного устройства и различается между
устройствами. В исходящих запросах `Salt` не передаётся, поэтому его связь
с `<Password>` или `<Code>` не установлена. Salt, зашифрованные поля и
ответы на контрольные вопросы не должны попадать в журналы.

## 5. Процедура A — обнаружение устройства

### 5.1. Подготовка и отправка inquiry

Из сетевого перехвата подтверждён запрос вида:

```xml
<Probe>
  <Uuid>...</Uuid>
  <Types>inquiry</Types>
  <ResponseMode>2</ResponseMode>
</Probe>
```

Он отправляется на IPv4 multicast `239.255.255.250:37020`.
SDK отправляет inquiry через каждый пригодный канал, поэтому в логах
может быть несколько частичных успехов и отказов.

### 5.2. Ответ устройства

Устройство отвечает:

```xml
<ProbeMatch>
  <Uuid>...</Uuid>
  <Types>inquiry</Types>
  <DeviceType>142601</DeviceType>
  <DeviceDescription>DS-2CD1021-I</DeviceDescription>
  <DeviceSN>...</DeviceSN>
  <CommandPort>8000</CommandPort>
  <HttpPort>80</HttpPort>
  <MAC>94-e1-ac-dd-a3-6f</MAC>
  <IPv4Address>192.168.1.64</IPv4Address>
  <IPv4SubnetMask>255.255.255.0</IPv4SubnetMask>
  <IPv4Gateway>192.168.1.1</IPv4Gateway>
  <DHCP>false</DHCP>
  <Activated>true</Activated>
  ...
</ProbeMatch>
```

### 5.3. Критерии успешного обнаружения

Минимум:

1. пакет не пустой и не превышает лимит;
2. XML корректен;
3. корень — `ProbeMatch` или подтверждённый `Hello`;
4. `Types` соответствует ожидаемому типу;
5. MAC корректен и является EUI-48;
6. IPv4 корректен либо разрешено промежуточное DHCP-состояние;
7. sender сохранён отдельно от адреса, указанного внутри XML.

### 5.4. Workflow

```mermaid
sequenceDiagram
    participant App as Наша программа
    participant Net as SADP multicast
    participant Cam as Камера

    App->>Net: Bind выбранного интерфейса
    App->>Net: Probe(Uuid, Types=inquiry)
    Net->>Cam: UDP 239.255.255.250:37020
    Cam-->>Net: ProbeMatch(Types=inquiry)
    Net-->>App: payload + фактический sender
    App->>App: Проверка XML, MAC, IP, Types
    App->>App: Дедупликация по MAC/UUID/состоянию
    App-->>App: DeviceFound
```

## 6. Процедура B — запрос информации и capability

В raw SADP отдельный универсальный тип `capabilities` в исследованном
сеансе не подтверждён. Информация о возможностях приходит в обычном
`inquiry`/`update`-ответе.

Подтверждённые признаки:

```xml
<SupportSecurityQuestion>true</SupportSecurityQuestion>
<SupportHCPlatform>true</SupportHCPlatform>
<Encrypt>true</Encrypt>
<PasswordResetAbility>true</PasswordResetAbility>
```

В бинарной части SDK также сохраняет capability-массивы:

```text
bySupport
bySupport1
bySupport2
```

Значения отдельных битов пока не расшифрованы.

### Workflow

```mermaid
flowchart LR
    A[Имеется MAC устройства] --> B[Отправить inquiry]
    B --> C[Получить ProbeMatch]
    C --> D[Проверить MAC и UUID]
    D --> E[Извлечь network state]
    E --> F[Извлечь capability/state flags]
    F --> G[Сохранить Device profile]
    G --> H{Операция поддерживается?}
    H -- нет --> X[Явная ошибка unsupported]
    H -- да --> I[Разрешить соответствующий workflow]
```

## 7. Процедура C — активация

### 7.1. Подтверждённые сообщения

Для успешных операций в новых трассах подтверждена последовательность:

```text
exchangecode -> activate
```

Исходящий запрос `exchangecode` имеет форму:

```xml
<Probe>
  <Uuid>...</Uuid>
  <MAC>...</MAC>
  <Types>exchangecode</Types>
  <Code>RSA-public-key</Code>
</Probe>
```

Значение `<Code>` в запросе имеет вид Base64 DER RSA public key. Ответ
устройства имеет форму:

```xml
<ProbeMatch>
  <Uuid>...</Uuid>
  <Types>exchangecode</Types>
  <Result>success</Result>
  <Code>...</Code>
</ProbeMatch>
```

После успешного обмена SDK отправляет защищённый запрос активации:

```xml
<Probe>
  <Uuid>...</Uuid>
  <MAC>...</MAC>
  <Types>activate</Types>
  <Password>encrypted-value</Password>
</Probe>
```

Ответ активации:

```xml
<ProbeMatch>
  <Uuid>...</Uuid>
  <Types>activate</Types>
  <Result>success</Result>
</ProbeMatch>
```

Проверка итогового состояния выполняется отдельным `inquiry` по MAC и
временному окну после операции:

```xml
<Activated>true</Activated>
```

`inquiry` не следует считать прямым ответом на `activate`: SDK параллельно
публикует периодические объявления и дубликаты через локальный IPC.

### 7.2. Логика завершённой процедуры

```mermaid
sequenceDiagram
    participant App as Наша программа
    participant Cam as Камера

    App->>Cam: Probe(Uuid, MAC, Types=exchangecode, RSA public key)
    Cam-->>App: ProbeMatch(exchangecode, Result=success, encrypted Code)
    App->>App: Расшифровать Code локальным ключом
    App->>Cam: Probe(Uuid, Types=activate, protected password)
    Cam-->>App: ProbeMatch(activate, Result=success)
    App->>Cam: Probe(Uuid, Types=inquiry)
    Cam-->>App: ProbeMatch(inquiry, Activated=true)
    App-->>App: ActivationAccepted
```

### 7.3. Что неизвестно

Не восстановлены с достаточной уверенностью:

- полный формат RSA-обмена и расшифрованного содержимого response `<Code>`;
- точная форма защищённого поля активации;
- связь между `Salt` из объявления и любым защищённым полем;
- обязательность контрольных вопросов для всех прошивок — не подтверждена и
  опровергается практикой на устройствах разных поколений; вопросы могут
  вообще не устанавливаться;
- точная форма ответа для других моделей и firmware.

### 7.4. Новая успешная активация (MAC `08-3b-c1-21-15-16`)

В трассе от 12.11.2026 исходное объявление устройства содержит
`Activated=false`, IP `192.168.1.64`, `DHCP=false` и постоянный
64-символьный `Salt`. Полная успешная последовательность:

```text
inquiry (Activated=false)
  -> exchangecode
  -> activate (Result=success)
  -> inquiry (Activated=true)
```

Для `exchangecode` SDK отправляет `<Code>` в виде Base64 DER RSA public
key. В этом сеансе его длина составляет 188 символов (140 байт после
декодирования). Ответный `<Code>` отличается от запроса и имеет 128 байт
после Base64-декодирования. Это подтверждает обмен ключевым материалом,
но не раскрывает полный протокол RSA.

Запрос `activate` содержит отдельный `<Password>` длиной 44 Base64-символа
(32 байта после декодирования), без атрибутов `bSalt` и `PWErrorParse`.
Ответ с тем же UUID возвращает `Result=success`, после чего `inquiry`
показывает `Activated=true`.

Последующее включение DHCP выполняется отдельной командой `update`, без
нового `exchangecode`. Поэтому профиль активации и профиль изменения сети
нельзя объединять в один формат шифротекста.

ISAPI-активация является отдельной процедурой и описана в
[ISAPI_ENCRYPTED_USER_FIELDS.md](ISAPI_ENCRYPTED_USER_FIELDS.md). Нельзя
подставлять ISAPI challenge/RSA/AES-механику в raw SADP без отдельной
wire-fixture.

## 8. Процедура D — контрольные вопросы

### 8.1. Чтение списка вопросов

После активации камера отвечает:

```xml
<ProbeMatch>
  <Uuid>...</Uuid>
  <MAC>94-e1-ac-dd-a3-6f</MAC>
  <Types>getsecurityquestion</Types>
  <Result>success</Result>
  <QuestionList>
    <Question><ID>1</ID><mark>false</mark></Question>
    ...
    <Question><ID>15</ID><mark>false</mark></Question>
  </QuestionList>
</ProbeMatch>
```

В наблюдаемом сеансе подтверждено наличие ID `1..15`.

Рекомендуемая модель:

```go
type SecurityQuestion struct {
    ID       int
    Selected bool
}
```

### 8.2. Установка и отличие от новых трасс

В исходном материале отчёта описывалась последовательность:

```text
getsecurityquestion
  -> exchangecode
  -> setsecurityquestion
  -> update/inquiry
```

Однако в новых трассах операция записи вопросов в точном виде
`setsecurityquestion` не подтверждена. В них наблюдается отдельная команда
`SetVerificationCode`. По одному имени и содержимому этой команды нельзя
считать её операцией записи контрольных вопросов:

```xml
<Probe>
  <Uuid>...</Uuid>
  <MAC>...</MAC>
  <Types>SetVerificationCode</Types>
  <VerificationCode>encrypted-value</VerificationCode>
  <Password>encrypted-value</Password>
</Probe>
```

Ответ команды имеет вид:

```xml
<ProbeMatch>
  <Uuid>...</Uuid>
  <MAC>94-e1-ac-dd-a3-6f</MAC>
  <Types>SetVerificationCode</Types>
  <Result>success</Result>
</ProbeMatch>
```

### 8.3. Workflow

```mermaid
sequenceDiagram
    participant App as Наша программа
    participant Cam as Камера

    App->>Cam: Probe(Types=getsecurityquestion)
    Cam-->>App: QuestionList(ID 1..15, mark)
    App->>App: Проверить поддержку и выбрать вопросы
    App->>Cam: Probe(Types=exchangecode)
    Cam-->>App: Code
    App->>App: Сформировать защищённый payload
    App->>Cam: Probe(Types=SetVerificationCode, protected fields)
    Cam-->>App: Result=success
    App->>Cam: Probe(Types=inquiry)
    Cam-->>App: Обновлённое состояние
    App-->>App: ConfigurationAccepted
```

Настройка контрольных вопросов является отдельной необязательной
процедурой. Если пользователь её не запросил или устройство не объявляет
соответствующую capability, активация не должна включать
`getsecurityquestion`, дополнительный `exchangecode` или
`SetVerificationCode`.

Типичный поддерживаемый вариант — отдельная команда с тремя парами
«вопрос-ответ». Количество, идентификаторы вопросов и точный формат
команды зависят от модели и прошивки; значение «три» нельзя считать
универсальным wire-требованием.

### 8.4. Неизвестные данные

По имеющимся материалам не установлены:

- точные имена полей исходного запроса;
- как передаются ответы `11`, `22`, `33`;
- wire-поле, в котором передаются ответы;
- RSA-derived key и алгоритм защиты ответов;
- является ли `exchangecode` общим для нескольких конфигурационных команд;
- требуется ли отдельная проверка `mark=true` после записи.

Нельзя реализовывать эту процедуру по одному только успешному ответу:
нужна проверка повторным `getsecurityquestion` либо подтверждённым
`inquiry`-состоянием. Если пользователь не выбирал настройку вопросов,
их отсутствие не является ошибкой и не должно блокировать завершение
активации.

### 8.5. Другие защищённые команды, наблюдавшиеся в новых трассах

Помимо `activate` и `SetVerificationCode`, SDK выполняет отдельный обмен
`exchangecode` перед другими конфигурационными командами:

```xml
<Probe>
  <Uuid>...</Uuid>
  <MAC>...</MAC>
  <Types>SetHCPlatform</Types>
  <HCPlatformEnable>true</HCPlatformEnable>
  <Password>encrypted-value</Password>
</Probe>
```

и:

```xml
<Probe>
  <Uuid>...</Uuid>
  <MAC>...</MAC>
  <Types>SetMailBox</Types>
  <MailBox>encoded-value</MailBox>
  <Password>encrypted-value</Password>
</Probe>
```

Для каждой команды создаются собственные UUID и `exchangecode`. Один
полученный `Code` нельзя считать универсальным для всего сеанса.

## 9. Процедура E — изменение статического IP

### 9.1. Подтверждённый SDK-вызов

Логирует:

```text
SADP_ModifyDeviceNetParam_V40
byDhcpEnable=0
IPv4Address=192.168.1.64
IPv4Gateway=192.168.1.1
IPv4SubNetMask=255.255.255.0
IPv6Address=::
IPv6Gateway=::
IPv6MaskLen=64
CommandPort=8000
HttpPort=80
```

Исходный XML использует:

```xml
<Types>update</Types>
<MAC>...</MAC>
<Password>...</Password>
<IPv4Address>...</IPv4Address>
<CommandPort>8000</CommandPort>
<IPv4SubnetMask>...</IPv4SubnetMask>
<IPv4Gateway>...</IPv4Gateway>
<IPv6Address>::</IPv6Address>
<IPv6Gateway>::</IPv6Gateway>
<IPv6MaskLen>64</IPv6MaskLen>
<DHCP>false</DHCP>
<HttpPort>80</HttpPort>
```

В трассе для MAC `08-3b-c1-21-15-16` подтверждены `PWErrorParse=true` и
`bSalt="true"` именно для `update`. Защищённое поле имеет Base64-длину
88 символов (64 байта после декодирования). Перед включением DHCP в
запросе передаётся текущий IP `192.168.1.64`, а фактический новый адрес
проверяется последующим `inquiry` (в этом сеансе `192.168.10.198`).

### 9.2. Завершение

Непосредственный ответ `success` не является единственной проверкой.
SDK получает несколько `update`, затем запускает новый `inquiry`.

Переходы IP в эксперименте:

```text
192.168.1.64 -> 192.168.1.65 -> 192.168.1.64
```

При этом MAC сохраняется.

### 9.3. Workflow

```mermaid
sequenceDiagram
    participant App as Наша программа
    participant Cam as Камера

    App->>App: Проверить MAC и static IPv4 config
    App->>Cam: Probe(Types=update, target MAC, protected password)
    Cam-->>App: ProbeMatch(update, Result=success)
    Note over Cam: Камера применяет новый адрес
    Cam-->>App: update с новым IP
    App->>Cam: Probe(Types=inquiry)
    Cam-->>App: ProbeMatch(inquiry, MAC + ожидаемый IP)
    App->>App: Проверить IP/mask/gateway/DHCP=false
    App-->>App: NetworkConfigAccepted
```

### 9.4. Корреляция

Ответ может прийти:

- со старого IP;
- с нового IP;
- через `127.0.0.1` как локальная копия SDK;
- несколько раз.

Ключ корреляции должен включать MAC и UUID, но не требовать совпадения
sender IP.

## 10. Процедура F — включение и отключение DHCP

### 10.1. Включение DHCP

Подтверждённый SDK-вызов:

```text
byDhcpEnable=1
IPv4Address=169.254.30.51
IPv4Gateway=0.0.0.0
IPv4SubNetMask=255.255.0.0
DHCP=true
```

В сетевом перехвате также наблюдалось состояние:

```xml
<IPv4Address>0.0.0.0</IPv4Address>
<IPv4SubnetMask>0.0.0.0</IPv4SubnetMask>
<IPv4Gateway>0.0.0.0</IPv4Gateway>
<DHCP>true</DHCP>
```

Поэтому допустимы временные состояния:

```text
DHCP=true + 0.0.0.0
DHCP=true + 169.254.x.x
DHCP=true + выданный DHCP-адрес
```

`0.0.0.0` нельзя использовать как destination, но нельзя отвергать
такое объявление как повреждённое.

### 10.2. Workflow

```mermaid
sequenceDiagram
    participant App as Наша программа
    participant Cam as Камера
    participant DHCP as DHCP-сервер

    App->>Cam: Probe(Types=update, DHCP=true, protected password)
    Cam-->>App: ProbeMatch(update, Result=success)
    Cam->>DHCP: DHCP discovery/request
    Cam-->>App: update(DHCP=true, IP=0.0.0.0 или 169.254.x.x)
    DHCP-->>Cam: Новый IPv4-адрес
    Cam-->>App: inquiry/update с назначенным IP
    App->>App: Проверить MAC и DHCP=true
    App-->>App: DHCPAccepted
```

### 10.3. Отключение DHCP

Процедура аналогична статическому изменению IP:

```text
DHCP=true
  -> Probe(update, DHCP=false, address/mask/gateway)
  -> update
  -> inquiry
  -> проверка DHCP=false и статических полей
```

### 10.4. Несоответствие текущего парсера

Текущий [ParseAdvertisement](../../../../internal/parser/parser.go)
отвергает `0.0.0.0`. Это нужно изменить с учётом `DHCP=true`, сохранив
строгую проверку для обычного `DHCP=false`.

## 11. Процедура G — Hik-Connect

### 11.1. Подтверждённая последовательность

В перехвате видны:

```text
exchangecode
SetHCPlatform
inquiry
```

Ответ изменения:

```xml
<ProbeMatch>
  <Uuid>...</Uuid>
  <MAC>94-e1-ac-dd-a3-6f</MAC>
  <Types>SetHCPlatform</Types>
  <HCPlatformEnable>true</HCPlatformEnable>
  <Result>success</Result>
</ProbeMatch>
```

Последующее объявление содержит:

```xml
<SupportHCPlatform>true</SupportHCPlatform>
<HCPlatformEnable>true</HCPlatformEnable>
```

### 11.2. Workflow

```mermaid
sequenceDiagram
    participant App as Наша программа
    participant Cam as Камера

    App->>Cam: Probe(Types=exchangecode)
    Cam-->>App: ProbeMatch(exchangecode, Code)
    App->>App: Проверить SupportHCPlatform
    App->>Cam: Probe(Types=SetHCPlatform, HCPlatformEnable=true)
    Cam-->>App: ProbeMatch(SetHCPlatform, Result=success)
    App->>Cam: Probe(Types=inquiry)
    Cam-->>App: ProbeMatch(HCPlatformEnable=true)
    App-->>App: HikConnectAccepted
```

Для отключения ожидается та же схема с
`HCPlatformEnable=false`, но отдельный успешный wire-fixture отключения
в материалах не представлен.

### 11.3. Ошибка firmware

В ранних ответах наблюдалось значение:

```xml
<HCPlatformEnable>flase</HCPlatformEnable>
```

Это не следует автоматически трактовать как `false`. Допустимые состояния
в модели должны включать `true`, `false` и `unknown`.

## 12. Защита команды изменения сети

В новых трассах SDK исходящие защищённые команды имеют форму:

```xml
<Password>Base64(ciphertext)</Password>
```

В новом сеансе для MAC `08-3b-c1-21-15-16` подтверждены:

```xml
<PWErrorParse>true</PWErrorParse>
<Password bSalt="true">Base64(ciphertext)</Password>
```

Это относится к сетевому `update`, а не к `activate`. В `activate` атрибуты
`bSalt` и `PWErrorParse` отсутствуют, а защищённое поле имеет другой размер:
32 байта после Base64-декодирования против 64 байт для `update`.

В текущей Go-реализации [BuildNetworkXML](../../../../internal/logic/build_activation_xml.go)
используется:

```xml
<SecurityResponse>sha256(password + challenge)</SecurityResponse>
```

Это нельзя считать универсально совместимым с исследованным SDK. Для конкретных устройств из новых трасс подтверждены следующие факты:

```text
Probe + Types=exchangecode + Code=RSA-public-key
Probe + Types=activate + Password (32 bytes after Base64 decoding)
Probe + Types=update + PWErrorParse=true + Password bSalt=true (64 bytes)
Probe + Types=update/конфигурационная команда + Password=Base64-value
```

Live x86-инструментирование установило для `activate` и password-reset
следующую внутреннюю структуру: `RSA_private_decrypt` с 128-байтным
входом и PKCS#1 padding, затем AES-128-ECB по отдельным 16-байтным
блокам. При reset отдельными блоками обрабатываются новый пароль и три
ответа на контрольные вопросы. Для `update` подтверждено аналогичное
AES-128-ECB преобразование ключевого блока и UUID-блоков.

Это не даёт готового production-кодека: точный вывод AES-ключа из RSA
результата, связь с `Salt` и wire-размещение reset-полей ещё не доказаны.
До получения публично пригодного, редактированного test vector нельзя
реализовывать криптографию по догадке.

Перед активацией SDK получает `exchangecode`. Для последующих `update` в
исследованном сеансе новый `exchangecode` не запрашивается. Нельзя
предполагать, что один обменный код или один формат ciphertext универсален
для `activate`, `SetVerificationCode`, `SetHCPlatform`, `SetMailBox` и
изменения сети.

## 13. Требования к корреляции и приёму

Текущий транспорт [SADPMutationTransport](../../../../internal/network/mutation_transport.go)
читает один первый ответ. Для исследованного поведения этого недостаточно.

Нужен цикл:

```text
send command
  -> receive until deadline
  -> decode message type
  -> verify MAC
  -> verify operation UUID where available
  -> ignore duplicate/local copy
  -> accept only expected Types
  -> wait for post-condition inquiry/update when the operation changes state
```

Нельзя требовать:

```text
response sender IP == original target IP
```

Нужно разрешать:

```text
старый IP -> новый IP
169.254.x.x -> DHCP IP
device sender -> 127.0.0.1 duplicate
```

## 14. Безопасность и журналирование

Нельзя логировать:

- пароль;
- `<Password>` и его ciphertext;
- `<Code>` из `exchangecode` (как запросный public key, так и ответный ciphertext);
- `Salt`;
- ответы на контрольные вопросы;
- полные исходящие XML с чувствительными полями.

Разрешённый диагностический минимум:

```text
operation type
operation UUID
MAC
observed sender
old/new IP
DHCP state
result code
elapsed time
error stage
```

Ошибки загрузки необязательных библиотек или интерфейсов должны быть
привязаны к конкретному каналу. Наличие `ERR` в общем SDK-логе само по себе
не означает провал SADP-операции: в исследованном сеансе после нескольких
ошибок интерфейсов `ParseDevResponse` завершился успешно.

## 15. Что уже можно реализовать уверенно

| Процедура | Состояние знаний | Реализация |
|---|---|---|
| Инициализация выбранного IPv4-интерфейса | Подтверждено | Можно реализовать |
| `inquiry` обнаружение | Подтверждено | Уже частично реализовано |
| Разбор `ProbeMatch` | Подтверждено | Расширить поля DHCP/network/capability |
| Изменение статического IP | Форма XML и `bSalt`/`PWErrorParse` подтверждены; live trace показывает AES-128-ECB для 64-байтного payload | Нужны key derivation и wire regression tests |
| Включение DHCP | Последовательность подтверждена | Нужны промежуточные IP-состояния |
| Отключение DHCP | Практически подтверждено | Нужна отдельная проверка post-condition |
| Активация | Типы, порядок и успешный post-condition подтверждены | Наблюдаются RSA private decrypt и AES-128-ECB; key derivation и wire-correlation требуют отдельного fixture |
| Чтение вопросов | Подтверждено для поддерживающих устройств | Можно реализовать как отдельную опциональную операцию |
| Установка вопросов | `getsecurityquestion` подтверждён; `setsecurityquestion` не подтверждён новыми трассами | Не смешивать с `SetVerificationCode`; AES-защита ответов наблюдается, wire-поле требует capture |
| Hik-Connect enable | Типы и порядок подтверждены | Payload protection не восстановлена |
| Hik-Connect disable | Предполагается симметрично | Нужен отдельный capture |

## 16. Что остаётся неизвестным

Отдельно зафиксированы следующие неопределённости:

1. алгоритм и ключевывод для `<Password>`;
2. полный формат RSA-обмена `exchangecode`;
3. связь `Salt` объявления с защищённым payload и точный KDF;
4. точный формат активационного пароля в raw SADP;
5. исходный XML установки контрольных вопросов;
6. wire-формат и RSA-derived key для ответов на контрольные вопросы;
7. точная семантика трёх массивов `bySupport`;
8. универсальность формата для других моделей и firmware;
9. обязательность `ResponseMode=2` для всех устройств;
10. различия между multicast, subnet и pcap-режимами на разных Windows-сетях.

Эти пункты нельзя закрывать предположениями или переносом механики ISAPI.

## 17. Новые live-данные: activation и password reset

В контролируемой серии на сброшенной камере подтверждены следующие
криптографические факты; секреты, ответы, ключи и ciphertext в публичный
отчёт не включаются:

1. `activate` вызывает `RSA_private_decrypt` для 128-байтного входа с
   PKCS#1 padding.
2. После RSA выполняется AES-128-ECB по отдельным 16-байтным блокам.
3. В activation-потоке шифруются ключевой блок и пароль; наблюдаемый
   результат имеет размер 32 байта после Base64-декодирования.
4. В `Security Question Mode` reset-поток дополнительно обрабатывает
   отдельными AES-блоками новый пароль и три ответа на контрольные вопросы.
5. Для `update` ранее полностью сопоставлены четыре AES-128-ECB блока с
   64-байтным wire-полем `<Password bSalt="true">`; plaintext включает
   UUID операции.
6. На исследованных `activate`, reset и `update` путях не обнаружены
   `security`, URI-параметр `iv` или вызов AES-CBC.

Эти результаты уточняют криптографический профиль, но не являются
спецификацией production-кодека: не установлены KDF/вывод AES-ключа,
plaintext RSA-операции и точное XML-размещение reset payload. Поэтому
текущая реализация обязана сохранять fail-closed поведение для
непроверенного `salted-aes` и не должна подменять наблюдаемый raw SADP
профиль схемой ISAPI `security=1/2`.

## 18. Рекомендуемый следующий порядок работ

### P0 — корректность транспорта

1. Ввести корреляцию по MAC/UUID/Types.
2. Перейти от чтения одного пакета к приёму до deadline.
3. Добавить дедупликацию loopback-копий.
4. Реализовать post-condition verification.
5. Разрешить DHCP-состояния `0.0.0.0` и `169.254.x.x`.

### P1 — модели и wire profiles

1. Расширить `Device` сетевыми и capability-полями.
2. Добавить типизированные операции `exchangecode`, `getsecurityquestion`,
   `SetVerificationCode`, `SetHCPlatform`, `SetMailBox`.
3. Сохранять два наблюдаемых признака отдельно: `Salt` из объявления и
   wire-атрибут `bSalt`; наличие последнего подтверждено только для
   сетевого `update` в отдельных SDK/firmware-профилях.
4. Сохранить raw capability bytes без преждевременной расшифровки.

### P2 — криптография и неизвестные операции

1. Получить контролируемые SDK-логи с известным паролем.
2. Сопоставить plaintext и ciphertext для нескольких повторов.
3. Получить исходные payload `activate`, `SetVerificationCode`,
   `SetHCPlatform`, `SetMailBox` и изменения сети.
4. Добавить wire-fixtures и только после этого реализовывать
   соответствующие криптографические билдеры.
