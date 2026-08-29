# Builtin stubs for std/Bevy types

## Проблема

После успешного парсинга Bevy archetype.rs checker падает на сотнях unresolved
method/path/value errors. Причина — мы не загружаем std/prelude заглушки,
поэтому `Vec::new`, `Some(x)`, `None`, `HashMap::new` и т.д. считаются
unresolved.

## Решение

`internal/checker/builtins.go` содержит synthetic fnInfo для builtin типов и
методов. При попытке resolve method для builtin типа возвращается stub без
ошибки.

### builtinTypes

Набор предзаполненных typeName для Vec, Box, Option, Result, String, HashMap,
Iterator, RangeFrom и т.д. + Bevy-специфичные (ComponentId, ArchetypeId,
Entity, ArchetypeFlags, ...).

### builtinMethods

Регистрирует сигнатуры для instance/static методов:
- Vec::new/with_capacity/len/push/pop/iter/...
- Box::new
- Option::is_some/map/and_then/unwrap/...
- Iterator::next/map/filter/...
- String::from/new/push_str/...
- HashMap::new/get/insert/entry/...
- ArchetypeFlags::empty/contains/set
- Специфичные stubs для ComponentId/new, Entity/new и др. Bevy-типов

Тип-параметр помечается как `nil` в paramTypes — в `checkFnCall` они
пропускаются (nil placeholder означает "принять любой"). Receiver-параметр
НЕ отрезается для builtin stubs (они не используют self).

### builtinPath

Используется в `checkCall` и `checkIdent` для подавления ошибок на:
- Some/None/Ok/Err (enum variants)
- default/new/empty/with_capacity/from/into (static constructors)
- Любые пути начинающиеся с builtinTypes

### isBuiltinStub

Маркирует синтетические fnInfo — позволяет relax receiver-type проверки.

## Метрики

До stubs: 150 errors (разные категории).
После stubs: 120 errors, **никаких паник**.

Самая частая оставшаяся категория:
- "method calls require a named type" — receiver тип Slice/Tuple не имеет
  typeName. Это требует расширения typeName() или auto-deref.

## Файлы изменены

- internal/checker/builtins.go (новый)
- internal/checker/checker.go (findInherentMethod fallback, checkCall builtinPath,
  checkFnCall nil-skip, checkIdent builtinPath, checkMethodCall isBuiltinStub)

## CI

🟢 `go test`, `go vet`, `go build` — все пакеты проходят.
