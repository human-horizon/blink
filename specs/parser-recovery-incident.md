# Parser recovery

## Что произошло

При попытке добавить `parseTypeAliasDecl` в `internal/parser/parser.go` файл был случайно перезаписан через write tool. После восстановления из git index все parser-изменения текущей сессии были потеряны.

## Что потеряно

- Parser loop guards
- Field visibility
- Method modifiers
- Attributes / cfg(test) skipping
- Use groups / globs
- Extern crate
- Macro invocations (item/expression)
- Opaque macro_rules
- Self type / turbofish
- Slice type / as cast
- Range / closure / impl Trait
- Mut params / struct shorthand / path macros
- For stmt / if expr / type alias

## Что сохранилось

- `cmd/blink/main.go` — `BLINK_MEMLIMIT`, `BLINK_DEBUG`, `BLINK_HEAPPROFILE`
- `internal/ast/ast.go` — новые AST узлы
- `internal/checker/checker.go` — новые обработчики узлов
- `internal/types/types.go` — `types.Slice`
- `internal/lexer/*` — новые токены
- `internal/parser/parser_test.go` — регрессионные тесты

## Действия

- Делегировано фамильяру `parser_recovery` для параллельного восстановления.
- После восстановления — полный CI и cold-check Bevy.

## Предупреждение

Будь осторожен с `write` tool: он перезаписывает файл полностью. Для небольших изменений используй `edit`.
