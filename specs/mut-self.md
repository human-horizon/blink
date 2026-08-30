# Распознавание `&mut self` для корректной мутации

## Контекст

Методы, объявленные с получателем `&mut self`, всё равно помечались как immutable: парсер не выставлял `IsMut` для self-параметра, отчего любые мутации `self.field` внутри таких методов давали ложную ошибку `cannot borrow self as mutable`. Это было первичной причиной 4 ошибок в `archetype-test`.

## Цель

Метод с `&mut self` должен позволять мутировать поля `self` внутри тела метода и при вызове других `&mut`-методов.

## Что изменится

1. `internal/parser/parser.go` — при разборе `&mut self` выставлять `IsMut: true` в `ast.Param`.
2. `internal/checker/checker.go` — в `checkImpl` биндить `self` с `IsMut` ресивера и флагом мутабельности.
3. `internal/parser/parser_test.go` — `TestParseMutSelfParam`.
4. `internal/checker/checker_test.go` — `TestMutSelfBorrow` (мутация поля self в `&mut self`).

## Критерии приёмки

- [ ] `TestParseMutSelfParam` и `TestMutSelfBorrow` проходят.
- [ ] `GOMEMLIMIT=1GiB go test -timeout=60s ./...` проходит.
- [ ] `go vet ./...` и `go build ./...` проходят.
- [ ] `blink check benchmarks/archetype-test` устраняет все `cannot borrow self as mutable` (0) и уменьшает общее число ошибок с 96 до 94.
