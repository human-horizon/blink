# Lenient обработка встроенных struct Bevy

## Контекст

`EntityLocation` и `ArchetypeEntity` — типы из bevy_ecs, которые не разбираются (исходники не загружаются), но используются как struct-literals (`EntityLocation { .. }`), в struct patterns (`&ArchetypeEntity { entity, table_row }`) и field access (`loc.archetype_id`). Чекер выдавал `unknown struct` / `unknown type`.

## Цель

Struct-literal, struct-pattern и field-access для зарегистрированных встроенных типов не должны давать ошибки, а должны возвращать допустимый placeholder-тип.

## Что изменится

1. `internal/checker/checker.go` — `checkStructLit` / `checkStructLitWithAnnotation`: для встроенного имени возвращать `Named{name}` без валидации полей.
2. `internal/checker/checker.go` — `checkStructPattern`: для встроенного имени биндить поля по имени (`BindName`/`Field`) в generic-заглушку.
3. `internal/checker/checker.go` — `fieldType`: для встроенного имени возвращать `Generic{_}`.
4. `internal/checker/builtins.go` — helper `isBuiltinTypeName`.
5. `internal/checker/checker_test.go` — `TestBuiltinStructLiteral`.

## Критерии приёмки

- [ ] `TestBuiltinStructLiteral` проходит.
- [ ] `GOMEMLIMIT=1GiB go test -timeout=60s ./...` проходит.
- [ ] `go vet ./...` и `go build ./...` проходят.
- [ ] `blink check benchmarks/archetype-test` сокращает число ошибок (целевое: 87 против 89).
