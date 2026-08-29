# Непрозрачные macro_rules

## Контекст

`bevy_ecs` содержит declarative macros с metavariables, repetitions и несколькими правилами. Текущий parser поддерживает только одну простую rule без metavariables.

## Цель

Разбирать сложные `macro_rules!` как непрозрачные объявления, сохраняя полноценную поддержку существующих простых макросов.

## Что изменится

1. `internal/lexer/token.go` и `internal/lexer/lexer.go` — токен `$`.
2. `internal/parser/parser.go` — распознавание сложного тела macro_rules и пропуск сбалансированных delimiters.
3. `internal/checker/checker.go` — unit-тип для вызова непрозрачного макроса без тела AST.
4. `internal/parser/parser_test.go` — регрессионные тесты.

## Критерии приёмки

- [ ] Простая `macro_rules!` с выражением сохраняет прежнее поведение.
- [ ] Макрос с metavariable разбирается без зависания.
- [ ] Несколько правил разбираются как opaque macro.
- [ ] Незавершённое тело отклоняется.
- [ ] Все тесты, `go vet` и `go build` проходят.
