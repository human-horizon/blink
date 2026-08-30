# Упрощение `impl Trait` возвратов в Bevy

## Контекст

Функции, возвращающие `impl Iterator<Item = X>`, разрешались в `Applied{Iterator, [Item = X]}`, тогда как конкретный iterator от `.iter()` возвращал builtin-заглушку `Named{Iterator}`. Типы не унифицировались, давая `expected Iterator<Item<X>>, found Iterator`. В `archetype-test` это были 3+ первичные ошибки.

## Цель

Функции, возвращающие `impl Trait<Associated = T>`, должны рассматриваться как возвращающие сам базовый trait, что позволяет им унифицироваться с builtin-заглушками iterator.

## Что изменится

1. `internal/checker/checker.go` — в `resolveType` для `*ast.ImplTraitType` вызывать `implTraitBase`, который сбрасывает associated type bound и возвращает базовый `Applied.Base` тип.
2. `internal/checker/checker.go` — добавлен helper `implTraitBase`.
3. `internal/checker/checker_test.go` — регрессионный тест на iterator-цепочку.

## Критерии приёмки

- [ ] checker-тесты проходят.
- [ ] `GOMEMLIMIT=1GiB go test -timeout=60s ./...` проходит.
- [ ] `go vet ./...` и `go build ./...` проходят.
- [ ] `blink check benchmarks/archetype-test` уменьшает число ошибок с 94 до 90 и убирает `expected Iterator<Item<..>>, found Iterator`.
