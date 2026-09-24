# Enum variants с payload (continuation этапа 1/5)

## Проблема
`enum Status { Blocked(i32) }` НЕ парсился (parse error на `(`). Варианты были
только unit — конструирование и биндинг значений в паттернах невозможен.

## Реализация
- **AST**: `Variant{Pos, Name, Fields []Type, FieldNames []string}`.
- **Парсер** (`parseEnumDecl`): `Name(T1, T2)` — tuple variant; `Name { f: T }`
  — struct variant (декларация парсится, имена полей сохраняются).
- **Checker** (`internal/checker/checker.go`):
  - `enumInfo.fields map[string][]types.Type` — ленивый кэш типов payload;
  - `variantFields(enumKey, ei, variant)` — резолвит типы полей в контексте
    файла объявления (currentIdx временно = itemFile[enumKey]);
  - `enumVariantPath(segments)` — резолв `Enum::Variant`;
  - конструирование `Enum::Variant(args)` — `checkCall` → 
    `checkVariantConstructor`: арность (`expected N arguments, found M`),
    типы аргументов через `types.Unify`;
  - bare-путь `Enum::Variant` с payload — `enum variant X::Y requires arguments`;
  - unit-вариант как путь → `Named{Enum}`;
  - **паттерны** `checkPathPattern`: при scrutinee=user enum — элементы
    биндятся на типы payload варианта; арность паттерна:
    `pattern has N subpatterns but variant X has M field(s)`.
- Вложенность: `Shape::Circle(n)` — `n: i32` настоящий (не `_`).

## Проверено
- CLI: /tmp/enum_pl rc=0; /tmp/enum_neg — 4 корректные диагностики с ТОЧНЫМИ
  позициями (7:19, 11:5, 15:5, 16:9) — synergy с этапом 7 spans.
- Тесты: `TestEnumTupleVariant`, `TestEnumVariantPayloadMismatch`,
  `TestEnumVariantArity`, `TestEnumStructVariantDecl`.
- `go test/vet/build` зелёные, `git diff --check` чистый.
- `blink build` на enum payload: cgen НЕ паникует, но C код не компилируется
  (генерация payload enum + match — этап 8).

## Не сделано (остаток)
- Struct-variant ПАТТЕРНЫ (`Shape::Point { x, y: _ }`) — парсер path+LBrace,
  биндинг по именам полей, `..` (PatRest).
- cgen: tag+union представление enum с payload (этап 8).
- `&str` vs `String` для строковых литералов (rustc говорит found `&str`).
