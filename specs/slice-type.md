# Slice-тип `[T]`

## Контекст

Bevy использует `Box<[ComponentStatus]>` — slice type. Parser поддерживал только `[T; N]`.

## Цель

Разбирать `[T]` как slice-тип в дополнение к `[T; N]`.

## Что изменится

1. `internal/ast/ast.go` — новый `SliceType`.
2. `internal/parser/parser.go` — обработка `[T]` без `;N`.
3. `internal/checker/checker.go` — pass-through для `SliceType`.

## Критерии приёмки

- [ ] `Box<[ComponentStatus]>` разбирается.
- [ ] `[T; N]` остаётся работать.
- [ ] Полный CI проходит.
