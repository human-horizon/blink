# Associated types в traits

## Статус
Реализовано в рамках этапа 1 (полный type system). Парсер ранее **пропускал**
`type Item;` внутри trait/impl — associated types не моделировались. Теперь
поддерживаются.

## Синтаксис
```rust
trait Iterator {
    type Item;              // без дефолта
    type Output = i32;      // с дефолтом
    fn next(&mut self) -> Option<Self::Item>;
}

impl Iterator for Counter {
    type Item = i32;        // конкретный тип в impl
    fn next(&mut self) -> Option<i32> { ... }
}
```

## Реализация

### AST (`internal/ast/ast.go`)
- `AssocTypeDecl{Pos, Name, Ty, GenParams}` — узел associated type.
  `Ty == nil` означает отсутствие дефолта (`type Item;`).
- `TraitDecl.AssocTypes []*AssocTypeDecl` и `ImplDecl.AssocTypes []*AssocTypeDecl`.

### Парсер (`internal/parser/parser.go`)
- `parseAssocType()` — парсит `type Name;` / `type Name = Ty;` (опционально
  с generic-параметрами). Вызывается из `parseTraitDecl` и `parseImplDecl`
  вместо прежнего «skip type aliases».

### Checker (`internal/checker/checker.go`)
- `traitInfo.assocTypes map[string]types.Type` — имя → дефолтный тип (nil если нет).
- `implInfo.assocTypes map[string]types.Type` — имя → конкретный тип.
- В `collect()`: регистрируются assoc types trait'а (резолв дефолта).
- В `collectImpl()`:
  - резолв assoc types impl'а;
  - проверка: отсутствующий assoc type (если нет дефолта) → ошибка
    `missing associated type`;
  - несовпадение с дефолтом trait'а → `does not match trait default`;
  - лишний assoc type → `is not part of trait`;
  - подстановка `Self::Item` в сигнатурах trait-методов через
    `substituteAssoc`/`substituteAssocFn` перед `fnSigMatches`.
- `substituteAssoc(t, assoc)` — заменяет `Named{Name: "Self::Item"}` на
  конкретный тип из карты impl'а (рекурсивно по Applied/Ref/Tuple/Array/Slice).

### checkBlock (исправление инференса конструкторов)
Tail-выражение блока теперь проверяется через `checkExprExpected` с ожидаемым
типом возврата (если он известен и не error). Это позволяет `Some(x)`/`Ok(x)`
инферировать payload из типа возврата функции — правильная Rust-семантика.
Раньше `Some(x)` без ожидаемого типа возвращал `i32` (builtinPath).

## Критерии
- `go test/vet/build` зелёные.
- `git diff --check` чистый.
- Регрессионные тесты: `TestTraitAssociatedType` (валидный),
  `TestTraitAssociatedTypeMissing`, `TestTraitAssociatedTypeMismatch`.
