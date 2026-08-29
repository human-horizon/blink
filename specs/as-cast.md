# `as`-cast в выражениях

## Контекст

Bevy использует `value as usize` и подобные числовые приведения. Текущий parser не поддерживает `as`.

## Цель

Разбирать цепочку `expr as Type` с правильным приоритетом — между unary и multiplicative.

## Что изменится

1. `internal/ast/ast.go` — новый `CastExpr`.
2. `internal/parser/parser.go` — `parseCast`, вызываемый из `parseMultiplicative`.
3. `internal/parser/parser_test.go` — регрессионный тест.
4. `internal/checker/checker.go` — обработка `CastExpr` (no-op для type checking).

## Критерии приёмки

- [ ] `1 as i32`, `self.0.get() as usize` разбираются.
- [ ] Приоритет: `1 + 2 as i32` → `(1) + ((2) as i32)`.
- [ ] Полный CI проходит.
