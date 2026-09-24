# Generic inference (unification + return-type)

## Статус
Реализовано (базовая bidirectional inference) в рамках этапа 1b. Полный
Hindley–Milner (generalization, однозначность, let-polymorphism) — отдельная
крупная работа, не входит в данную итерацию.

## Что работает
1. **Вывод из аргументов** (unification): `identity(42)` → `T=i32`;
   `pair(1, true)` → `(i32, bool)`.
2. **Вывод из ожидаемого типа возврата** (no-arg generic-call):
   ```rust
   fn make<T>() -> Option<T> { None }
   let x: Option<i32> = make();   // T=i32 из expected
   ```

## Реализация

### `types.Unify` (`internal/types/types.go`)
- Добавлено связывание generic-переменной в **got**-позиции: прежде `Unify`
  просто возвращал `true` для любого Generic в got (wildcard-заглушка), не
  записывая mapping. Теперь для `Generic{Name != "_"}` записывает
  `mapping[Name] = want` (bidirectional unification). Для `Generic{"_"}` —
  прежнее поведение (placeholder).

### `checkExprExpected` / `inferGenericFromReturn` (`internal/checker/checker.go`)
- Для no-arg generic-вызова с известным expected-типом унифицирует expected с
  declared return type функции; если все generic-параметры выведены — возвращает
  подставленный return type.
- Ограничено вызовами **без аргументов**, чтобы не переопределять вывод из
  аргументов (иначе `identity(42)` в `-> bool` ошибочно вывел `T=bool`).

## Критерии
- `go test/vet/build` зелёные.
- `git diff --check` чистый.
- Регрессионные тесты: `TestGenericInferFromReturnType` (новый),
  `TestGenericMismatch` (остался зелёным после ограничения no-arg).
