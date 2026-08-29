# Type aliases

## Контекст

Bevy использует `pub type ComponentIndex = HashMap<ComponentId, HashMap<ArchetypeId, ArchetypeRecord>>;`. Parser не поддерживает `type` aliases.

## Цель

Разбирать `type` aliases как declaration.

## Что изменится

1. `internal/ast/ast.go` — новый `TypeAliasDecl`.
2. `internal/parser/parser.go` — обработка `type` в top level.
3. `internal/checker/checker.go` — добавить в collect.

## Пример

```rust
type ComponentIndex = HashMap<ComponentId, HashMap<ArchetypeId, ArchetypeRecord>>;
```

## Критерии приёмки

- [ ] `type X = Y;` разбирается.
- [ ] Полный CI проходит.
