# Const generics (`const N: usize`, `[T; N]`)

## Статус
Реализовано в рамках этапа 1b (полный type system). Парсер ранее падал на
`const N: usize` (parse error) и на `[T; N]` с const-параметром длины.

## Синтаксис
```rust
struct Arr<const N: usize> {
    data: [i32; N],
}
let _: Arr<3> = Arr { data: [1, 2, 3] };
```

## Реализация

### AST (`internal/ast/ast.go`)
- `ArrayType.LenName string` — непусто, когда длина — const-параметр (`[T; N]`).
- `ConstIntLitType{Pos, Val}` — целочисленный литерал в позиции const-аргумента
  (`Arr<3>`).

### Types (`internal/types/types.go`)
- `Array.LenName string` — метка const-параметра длины.
  - `Array.String()`: `[i32; N]` при LenName, иначе `[i32; 3]`.
  - `Array.Equals()`: const-параметр сравнивается по имени; конкретная длина —
    по значению; параметр с конкретной длиной всегда false.
- `ConstInt{Val}` — литерал-const-аргумент; участвует в подстановке длины.
- `Substitute()`: для `Array` с LenName заменяет длину на значение `ConstInt`
  из mapping (Len=Val, LenName="").

### Парсер (`internal/parser/parser.go`)
- `parseParams()`: обработка `const N: usize` (имя в generics).
- `[T; N]`: если длина — Ident, парсится в `LenName`; иначе IntLit → Len.
- `parseTypeArgs()`: IntLit в позиции аргумента → `ConstIntLitType`.

### Checker (`internal/checker/checker.go`)
- `resolveType(ArrayType)`: LenName валидируется (`const parameter is not in
  scope`).
- `resolveType(ConstIntLitType)` → `types.ConstInt`.
- `checkStructLitFields()`: подстановка args через `types.Substitute` заменяет
  `LenName` на конкретную длину. Несовпадение длины → `expected [i32; 3],
  found [i32; 2]`.

### Cgen (`internal/checker/cgen.go`)
- `Array` с LenName эмитится как `[0]` (длина неизвестна без monomorphization).

## Критерии
- `go test/vet/build` зелёные.
- `git diff --check` чистый.
- Регрессионные тесты: `TestConstGeneric` (валидный),
  `TestConstGenericLengthMismatch`.
