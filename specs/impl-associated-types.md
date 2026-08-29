# Impl и Trait type aliases

## Проблема

В Bevy используются:

- `impl Trait for Type { type Output = [Archetype]; }` — associated type aliases внутри `impl` блоков.
- `trait Index { type Output; fn index(...) -> &Self::Output; }` — associated type в trait.
- Self::Output в типах возврата.
- `&Self::Output` как reference на ассоциированный тип.

## Решение

### Parser changes

В `parseImplDecl` после проверки `p.tok.Kind != lexer.Fn` добавлена обработка:

```go
if p.tok.Kind == lexer.Ident && p.tok.Text == "type" {
    p.next() // type
    _ = p.expect(lexer.Ident)
    if p.tok.Kind == lexer.Lt {
        _, _, _ = p.parseParams()
    }
    if p.tok.Kind == lexer.Eq {
        p.next()
        _ = p.parseType()
    }
    if p.tok.Kind == lexer.Semi {
        p.next()
    }
    continue
}
```

Аналогично для `parseTraitDecl`.

### Self path types

`parseType` для `Ident`/`Self` теперь обрабатывает `::`:

```go
for p.tok.Kind == lexer.ColonColon {
    p.next()
    name = name + "::" + p.tok.Text
    p.next()
    if p.tok.Kind == lexer.Lt {
        args = p.parseTypeArgs()
    }
}
```

Это позволяет `Self::Output`, `std::vec::Vec<T>` и другие path-типы.

## Регрессия

Phase1 CI тесты + новые match tests проходят.

## Пример

```rust
impl Index<RangeFrom<ArchetypeGeneration>> for Archetypes {
    type Output = [Archetype];

    fn index(&self, index: RangeFrom<ArchetypeGeneration>) -> &Self::Output {
        &self.archetypes[index.start.0.index()..]
    }
}
```
