# Паттерны match (этап 5)

## Статус
Реализованы: exhaustiveness, range patterns (`1..=5`), slice/array patterns
(`[a, b]`), binding modes (`ref x`, `ref mut x`, `mut x`), or-patterns
(верхнего уровня и вложенные `(1 | 2, 3)`). Этап 5 закрыт.

## 1. Исчерпывающий match
```rust
enum Direction { North, South, East, West }

fn name(d: Direction) -> i32 {
    match d {
        Direction::North => 0,
        Direction::South => 1,
        Direction::East => 2,
        // ошибка: West не покрыт
    }
}
```

## Реализация (`internal/checker/checker.go`)
- `checkMatch` вызывает `checkMatchExhaustive(e, scrutineeTy)`.
- `checkMatchExhaustive`:
  - определяет тип scrutinee (`typeName`);
  - если это объявленный enum — собирает покрытые имена вариантов из паттернов
    (`PatPath` → последний сегмент пути);
  - паттерн `PatWildcard` (`_`) или `PatIdent` (binding) покрывает всё → return;
  - если есть непокрытые варианты → `non-exhaustive `match`: variant(s) X not covered`.

## Критерии
- `go test/vet/build` зелёные. `git diff --check` чистый.
- Регрессионные тесты: `TestMatchExhaustive` (валидный), `TestMatchNonExhaustive`
  (неисчерпывающий), `TestMatchWildcardCoversAll` (wildcard),
  `TestMatchRangePattern`.

## 2. Range patterns
```rust
fn class(n: i32) -> i32 {
    match n {
        1..=5 => 1,   // inclusive
        6..=10 => 2,  // inclusive
        _ => 3,
    }
}
```
### Реализация
- **AST**: `PatRange{Pos, Low, High, Inclusive}`.
- **Парсер**: в `parsePattern`, после IntLit при токене `Dot` парсит `..`/`..=`
  (две точки + опц. `Eq`) и верхнюю границу.
- **Checker**: `checkPattern` обрабатывает `PatRange` как значение-паттерн
  без привязок.

## 3. Slice/array patterns
```rust
fn first_two(arr: [i32; 2]) -> i32 {
    match arr {
        [a, b] => a + b,
        _ => 0,
    }
}
```
### Реализация
- **AST**: `PatSlice{Pos, Elements}`.
- **Парсер**: `parsePattern` для `[` возвращает `PatSlice` (ранее `PatTuple`).
- **Checker**: `checkSlicePattern` привязывает каждый элемент-паттерн к типу
  элемента scrutinee (`sliceElemType`: Array/Slice/Vec/Ref).
- Тест: `TestMatchSlicePattern`.

## 4. Binding modes
```rust
match x {
    ref r => *r,        // r: &T
    mut y => y + 1,     // y: T, изменяемая привязка
    ref mut rm => ...,  // rm: &mut T
}
```
- **AST**: `PatIdent{IsRef, IsMut}`.
- **Парсер**: `parsePatternBase` распознаёт `ref`/`ref mut`/`mut` + ident.
- **Checker**: `checkPattern`: при `IsRef` биндит `&T`/`&mut T`; иначе `T` с
  mut-флагом.
- Тесты: `TestBindingModeRef`, `TestBindingModeMut`, `TestBindingModeRefMut`.
- Ограничение: `ref`-привязка не регистрирует loan в borrow-checker (лениво).

## 5. Or-patterns
```rust
match t {
    (1 | 2, 3) => 10,   // вложенный or внутри tuple-паттерна
    _ => 20,
}
```
- **AST**: `PatOr{Alternatives}`.
- **Парсер**: `parsePattern` — обёртка над `parsePatternBase`, жадно собирает
  `p1 | p2 | ...` в `PatOr` (везде, включая вложенные позиции).
- **Checker**: `checkPattern` проверяет каждую альтернативу; exhaustiveness
  (`collectCovered`) объединяет покрытые варианты по альтернативам.
- Тест: `TestNestedOrPattern`.
- Ограничение: не проверяется равенство наборов привязок между альтернативами
  (rustc требует одинаковые bindings).
