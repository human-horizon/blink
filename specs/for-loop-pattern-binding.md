# Биндинг паттернов for-loop для совместимости с Bevy

## Контекст

Чекер `ForStmt` не связывал переменные паттерна цикла с областью видимости. В результате обращения к переменным цикла (`component_id`, `idx`, `id`, `component`) давали ложную ошибку `cannot find value ... in this scope`. Причиной было отсутствие вызова `checkPattern` для паттерна `for ... in`.

## Цель

Связать переменные цикла с областью видимости и типом элемента итерации, чтобы снять первичные ошибки `cannot find value` при проверке Bevy.

## Что изменится

1. `internal/checker/checker.go` — в `CheckStmt` для `ForStmt` вызывать `checkPattern` c типом элемента итератора; добавить helper `iteratorElem`, который извлекает тип элемента из `Ref`, `Applied`, `Array`, `Tuple` или возвращает generic-заглушку.
2. `internal/checker/checker.go` — `checkTuplePattern` для generic-заглушки связывает элементы кортежа той же заглушкой без ошибки `expected tuple`.
3. `internal/checker/builtins.go` — `enumerate()` для `Iterator` и `array` возвращает `(i32, _)` — кортеж индекс/элемент, чтобы `for (idx, component_id)` деструктурировалось корректно.
4. `internal/checker/checker_test.go` — регрессионный `TestForLoopPatternBinding` для for-in и tuple-деструктуризации.

## Критерии приёмки

- [ ] `TestForLoopPatternBinding` проходит.
- [ ] `GOMEMLIMIT=1GiB go test -timeout=60s ./...` проходит.
- [ ] `go vet ./...` и `go build ./...` проходят.
- [ ] `blink check benchmarks/archetype-test` уменьшает число ошибок (целевое: 96 против 104).
- [ ] Никаких новых panic.
