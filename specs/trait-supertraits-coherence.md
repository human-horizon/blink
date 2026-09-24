# Supertraits, coherence и associated consts

## Статус
Реализовано в рамках этапа 1b (полный type system). Парсер ранее **не** умел
парсить `trait A: B` (была parse error) и пропускал associated consts.

## Supertraits

### Синтаксис
```rust
trait Base { fn base(&self) -> i32; }
trait Derived: Base { fn derived(&self) -> i32; }

struct S;
impl Base for S { fn base(&self) -> i32 { 1 } }
impl Derived for S { fn derived(&self) -> i32 { 2 } }
```

### Реализация
- **AST** (`internal/ast/ast.go`): `TraitDecl.Supertraits []string`.
- **Парсер** (`internal/parser/parser.go`): после `parseParams()` парсит
  `: Super1 + Super2` (с `::`-путями).
- **Checker** (`internal/checker/checker.go`):
  - `traitInfo.supertraits []string`; регистрация в `collect()`.
  - `collectImpl()`: при `impl Derived for S` проверяет, что `S` реализует все
    supertraits `Derived` → `does not implement supertrait X of trait Y`.
  - `hasTraitImpl()` теперь транзитивная: тип `T` реализует `Derived` только
    если реализует и все supertraits (закрытие). Это распространяется на
    `checkBounds` (проверка bounds на call-site).

## Coherence (overlap detection)
- `collectImpl()`: если для пары (trait, typeName) уже есть impl → ошибка
  `conflicting implementations of trait T for type S`. Возврат до перезаписи,
  чтобы избежать каскадных ошибок.
- Inherent-методы уже имели проверку `duplicate inherent method`.

## Associated consts
```rust
trait Shape {
    const SIDES: i32;
    fn area(&self) -> i32;
}
struct Square;
impl Shape for Square {
    const SIDES: i32 = 4;   // конкретное значение в impl
    fn area(&self) -> i32 { 16 }
}
// доступ: Square::SIDES
```

### Реализация
- **AST**: `AssocConstDecl{Pos,Name,Ty,Value}`; `TraitDecl.AssocConsts` и
  `ImplDecl.AssocConsts` (Value nil в trait-сигнатуре).
- **Парсер**: `parseAssocConst(inTrait)`; различает `const fn` (метод) от
  `const Y: i32;` (associated const) через `peekNext()==fn`.
- **Checker**:
  - `traitInfo.assocConsts map[string]*AssocConstDecl` (имя → сигнатура),
    `implInfo.assocConsts map[string]types.Type` (имя → тип значения).
  - `collectImpl`: missing associated const → `missing associated const X`;
    несовпадение типа → `assoc const X has type Y, expected Z`;
    лишний → `is not part of trait`.
  - `checkPathExpr`: `Type::CONST` резолвится через
    `findAssociatedConst(typeName, name)` (сканирует trait impls типа) —
    возвращает тип константы.

## Критерии
- `go test/vet/build` зелёные.
- `git diff --check` чистый.
- Регрессионные тесты: `TestTraitSupertrait` (валидный),
  `TestTraitSupertraitMissing`, `TestTraitImplConflict`,
  `TestTraitAssociatedConst`, `TestTraitAssociatedConstMissing`.
