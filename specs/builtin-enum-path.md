# Разрешение builtin enum-вариантов и констант

## Контекст

`archetype-test` использует enum-варианты и константы встроенных Bevy-типов как значения: `StorageType::Table`, `StorageType::SparseSet`, `ComponentStatus::Added`, `ArchetypeFlags::ON_ADD_HOOK` и др. `checkPathExpr` не распознавал их и выдавал `unresolved path` — до 12 первичных ошибок.

## Цель

Пути вида `BevyType::VariantOrConst`, где `BevyType` уже зарегистрирован во встроенных типах, не должны давать `unresolved path`.

## Что изменится

1. `internal/checker/checker.go` — в `checkPathExpr` перед ошибкой `unresolved path` проверять `builtinPath(e.Segments)`; для встроенного типа возвращать `i32AnyType()`.
2. `internal/checker/checker_test.go` — `TestBuiltinEnumVariantPath`.

## Критерии приёмки

- [ ] `TestBuiltinEnumVariantPath` проходит.
- [ ] `GOMEMLIMIT=1GiB go test -timeout=60s ./...` проходит.
- [ ] `go vet ./...` и `go build ./...` проходят.
- [ ] `blink check benchmarks/archetype-test` уменьшает число ошибок (целевое: 88 против 90).
