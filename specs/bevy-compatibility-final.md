# Bevy compatibility — Phase 4 (final parser pass)

## Статус

🟢 **Bevy archetype-test полностью парсится** — parser проходит весь `bevy_ecs/src/archetype.rs` (37000+ chars) без parse errors.

Остались только **checker errors** (semantic-level), что ожидаемо — Bevy интенсивно использует std lib, который не загружается в blink.

## Compatibility fixes (этот проход)

| Что | Где |
|-----|-----|
| `..` range (open-ended) detection в postfix loop | parsePrimary |
| `pub unsafe fn` / `pub const fn` в trait/impl methods | parseTraitDecl, parseImplDecl |
| Unsafe keyword (lexer.Unsafe, не Ident) | parseTraitDecl, parseImplDecl |
| Path macros `bitflags::bitflags!` (item-level) | parseDecl |
| Struct shorthand `Field { field, other }` | parseStructLitFromName |
| skipAttributes на struct fields | parseStructDecl |
| Associated type bounds в type args `<Item = ComponentId>` | parseTypeArgs |
| Slice pattern `[a, b]` | parsePattern |
| Postfix `?` operator | parseUnary |
| Path patterns `Enum::Variant(x)` | parsePattern |
| Enum variant tuple pattern `Some(x)` | parsePattern |
| `match` keyword + match expression/statement | parseMatchExpr + parseStmt/parsePrimary |
| FatArrow `=>` (отдельный токен от `->`) | lexer/parser |
| `if let` statement | parseIfExpr |
| `Self::Output` path types | parseType |
| `type` alias inside impl/trait | parseImplDecl, parseTraitDecl |

## Метрики

- Парсинг Bevy archetype.rs: парсится без parse errors.
- CI тестов: 30+ parser regression tests + все phase1–11 fixtures.

## Командный вызов

```
BLINK_MEMLIMIT=4GiB /tmp/blink-debug check benchmarks/archetype-test
```

## Следующие шаги

1. Расширить checker для Bevy-specific фич (Vec/Box/Iterator APIs).
2. Загрузить std/prelude заглушки.
3. Тысячи checker errors исчезнут когда добавим базовые impl для встроенных типов.
