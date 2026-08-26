# blink — Фаза 3: Lifetimes & Borrow Checker

## Контекст

Фаза 2 реализовала generics. Теперь нужно добавить одну из ключевых фич Rust — borrow checker, который гарантирует безопасность памяти без GC. Это критично для полноценного type checker.

## Цель

Добавить базовый borrow checker для локальных переменных с правилами ownership, shared/mutable borrow и move. Поддержать `&T` и `&mut T` без явных lifetime-имён (elision). Добавить оператор присваивания `=`.

## Что изменится

1. `internal/ast/ast.go` — добавить `AssignStmt`.
2. `internal/lexer/token.go` / `lexer.go` — уже есть `&`, `mut`.
3. `internal/parser/parser.go` — парсить `x = expr;` как присваивание.
4. `internal/types/types.go` — `Ref` уже есть; добавить `IsCopy` для примитивов.
5. `internal/checker/` — новый модуль `internal/checker/borrow.go` для отслеживания loans.
6. `internal/checker/checker.go` — интегрировать borrow checking в проверку тел функций.
7. Тесты: unit + `testdata/phase3/`.

## Детали реализации

### Поддерживаемый синтаксис

```rust
fn swap(a: &mut i32, b: &mut i32) {
    let tmp = *a;
    *a = *b;
    *b = tmp;
}

fn main() {
    let mut x: i32 = 1;
    let y: &i32 = &x;
    let z: i32 = *y;
    x = x + 1; // ошибка: x borrowed immutably
}
```

### Правила borrow checker

- **Copy types**: `i32`, `bool` копируются, не move'ятся.
- **Shared borrow** `&T`: можно неограниченное число, но пока активны, owner нельзя move или mutate.
- **Mutable borrow** `&mut T`: только один, и никаких других borrow (shared/mutable) одновременно.
- **Move**: присваивание/передача в функцию по значению non-Copy типа передаёт ownership.
- **Use after move**: использование переменной после move — ошибка.
- **Dereference**: `*r` требует, чтобы `r` была ссылкой.

### Что НЕ входит

- Именованные lifetimes (`'a`).
- Lifetime elision для сложных сигнатур (возврат ссылки из функции).
- Struct fields ссылок.
- `static`, `Box`, raw pointers.

### Интеграция

- После type checking функции запускать borrow checking её тела.
- Borrow checker работает statement-by-statement, обновляя состояние scope.

## Критерии приёмки

- [ ] `go build ./...` и `go vet ./...` без ошибок.
- [ ] `go test -timeout=30s -run=. ./...` с `GOMEMLIMIT=1GiB` проходит.
- [ ] Unit тест: shared borrow работает.
- [ ] Unit тест: mutable borrow блокирует shared.
- [ ] Unit тест: use after move — ошибка.
- [ ] Unit тест: assignment `=` работает.
- [ ] `testdata/phase3/valid/` → exit 0.
- [ ] `testdata/phase3/invalid/` → exit 1.
