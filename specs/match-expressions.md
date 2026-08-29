# FatArrow и Match expressions

## Проблема

В Bevy используются:

- `=>` как FatArrow (отдельный токен, не `=` + `>`)
- `match` keyword для pattern matching в expression и statement позициях
- `if let` statements с binding patterns

## Решение

### Lexer changes

- Добавлен новый токен `lexer.FatArrow` (`=>`) в `internal/lexer/token.go`.
- В `internal/lexer/lexer.go` после `=` если следующий символ `>`, возвращается FatArrow.
- Arrow (`->`) остался как было.
- Ключевое слово `match` → `lexer.Match`.

### Parser changes

- `parseIfExpr` распознаёт `if let Pat = expr { }` и обрабатывает отдельно от обычного `if cond { }`.
- `parseMatchExpr` — паттерн-матчинг как expression (используется в `let x = match y { ... };` и в return-выражениях).
- `parseMatchStmt` обёртка над `parseMatchExpr`, возвращает ExprStmt.
- `parseStatement` для `lexer.Match` возвращает ExprStmt от `parseMatchExpr`.
- `parsePrimary` для `lexer.Match` вызывает `parseMatchExpr`.

### Match arms syntax

```
match scrutinee {
    pattern1 => body1,
    pattern2 | pattern3 => body2,
    WildcardPattern => body3,
}
```

Парсинг arms:
- pattern через `parsePattern`
- optional `|` для or-patterns (следующие паттерны пропускаются)
- `=>` (FatArrow)
- body: либо `LBrace block`, либо expression

Expression body с balanced delimiters: учитываются `()`, `[]`, `{}` для правильного нахождения `,` или следующего `=>`.

## Regression тесты

- `TestParseFatArrow`: match expressions и `=>` в macro_rules.
- `TestParseMatchExpr`: match с int patterns и enum variant patterns.
- `TestParseMethodChainRange`: chain с range.

## Связанные изменения

- Pattern parsing расширен для литералов (`IntLit`, `StringLit`, `True`, `False`), enum variant tuple patterns `Some(x)`, path patterns `Entry::Variant(x)`.
- `parseType` теперь обрабатывает `Self::Output` и path в типах.
