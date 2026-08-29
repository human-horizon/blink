# Mutable self-параметр

## Контекст

После поддержки `Self` проверка Bevy останавливается на `&mut self`. Текущий parser принимает только `&self`.

## Цель

Распознавать `&mut self` как методный receiver и сохранять признак mutability в AST.

## Что изменится

1. `internal/ast/ast.go` — признак mutable receiver в `Param`.
2. `internal/parser/parser.go` — разбор `&mut self`.
3. `internal/parser/parser_test.go` — регрессионный тест.

## Критерии приёмки

- [ ] `fn method(&mut self);` разбирается.
- [ ] `&self` продолжает разбираться.
- [ ] `&mut value` не принимается как receiver без `self`.
- [ ] Все тесты, `go vet` и `go build` проходят.
