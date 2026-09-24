# Удаление wildcard-релаксаций (этап 1c)

## Статус
Удалены все Bevy-специфичные wildcard-релаксации из `internal/types/types.go`.
`go test/vet/build` + `git diff --check` — ALL GREEN.

## Удалённые механики
1. `isOpaqueIntStub` (Bevy newtypes: ArchetypeId, StorageType, ComponentId, ...)
   — коерсия в/из i32. Типы уже удалены из builtins на этапе 0.
2. `isUsizeStub` (usize/u32/u64/... → i32) — коерсия целочисленных примитивов.
3. `Self` wildcard — `Named{"Self"}` считался равным любому типу (Named.Equals,
   Unify). Теперь Self резолвится через `c.currentSelf`/подстановку.
4. `Option → bool` — Option<X> в if-условии (Builtin.Equals, Applied.Equals).
5. `HashMap/ComponentIndex → i32`, `StorageType/ComponentStatus → usize`
   (Applied.Equals, Unify).
6. `Iterator → Vec/Slice/Array` (Unify) — Bevy into_iter().
7. `Vec ↔ Array` (Unify), `Slice ↔ Array` (Slice.Equals), `&[X] ↔ [_]`,
   `&[X] ↔ &[T; N]` (Ref.Equals).
8. `Named ↔ Applied` (Unify, Named.Equals) — неинстанцированная форма.
9. `Generic{"_"}` placeholder — считался равным любому типу (Generic.Equals,
   Applied.Equals, Ref.Equals, Slice.Equals, Unify).
10. `Option → None` (Named.Equals).

## Оставшееся (правильная Rust-семантика, не костыли)
- Deref coercion: `&Vec<T> → &[T]`, `&Box<[T]> → &[T]` (Ref.Equals, Unify Ref).
- Bi-directional unification для вывода generic-параметров.

## Новый правильный механизм
- `checkExprExpected`: unit variant `None` (Ident) при ожидаемом `Option<T>`
  типизируется как `Option<T>` (заменило удалённый wildcard `None→Option` в
  Named.Equals).

## Удалённые тесты (тестировали хаки)
- `TestGenericPlaceholderEqualsAnything`, `TestEqualsAppliedMatchesNamedBase`,
  `TestUnifyUninstantiatedGenericAsValue`.

## Критерии
- `go test/vet/build` зелёные. `git diff --check` чистый.
- Двоичный код без wildcard-релаксаций; строгая проверка типов.
