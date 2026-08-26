# blink — Фаза 2: Generics

## Контекст

Фаза 1 реализовала foundation: lexer, parser, AST, базовый type checker и CLI для ограниченного подмножества Rust. Теперь нужно добавить поддержку обобщённых типов (generics) — один из столпов Rust, без которого невозможно приблизиться к полноценному type checker.

## Цель

Добавить в `blink` поддержку generic параметров для функций, структур и enum'ов с базовым выводом типов при вызове/использовании. Trait bounds (`T: Trait`) не входят в эту фазу.

## Что изменится

1. `internal/ast/ast.go` — добавить generic parameters к `FnDecl`, `StructDecl`, `EnumDecl`; добавить generic application к `NamedType`.
2. `internal/lexer/token.go` / `lexer.go` — добавить токены `<` и `>` (уже частично есть как `Lt`/`Gt`, но generic params требуют корректного парсинга).
3. `internal/parser/parser.go` — парсить `fn foo<T, U>(...)`, `struct Pair<T> { ... }`, `enum Option<T> { ... }`, `Pair<i32>`.
4. `internal/types/types.go` — добавить `Generic` и `AppliedType`; поддержать generic type equality.
5. `internal/checker/checker.go` — реализовать substitution при вызове generic-функций и создании generic-структур/enum'ов; вывод типов по аргументах.
6. `internal/checker/checker_test.go` — unit тесты generics.
7. `testdata/phase2/` — интеграционные примеры.

## Детали реализации

### Поддерживаемый синтаксис

```rust
fn identity<T>(x: T) -> T {
    x
}

fn make_pair<T, U>(a: T, b: U) -> Pair<T, U> {
    Pair { first: a, second: b }
}

struct Pair<T, U> {
    first: T,
    second: U,
}

enum Option<T> {
    Some(T),
    None,
}

fn main() -> i32 {
    let p: Pair<i32, bool> = Pair { first: 1, second: true };
    identity(p.first)
}
```

### Что НЕ входит

- Trait bounds (`where T: Clone`).
- Higher-kinded types.
- Associated types.
- Lifetime parameters.
- Default generic parameters.

### Type checker

- При `collect` сохранять generic parameters для функций/структур/enum'ов.
- При вызове `identity(42)` вывести `T = i32` по типу аргумента.
- При создании `Pair<i32, bool> { ... }` применить substitution к типам полей.
- При обращении к полю generic структуры возвращать подставленный тип.
- Type equality: `Pair<i32, bool>` != `Pair<bool, i32>`; `Pair<T, T>` != `Pair<T, U>`.

### Производительность

- Substitution через неглубокое копирование типов (MVP).
- Избегать экспоненциального взрыва — не мономорфизировать, а проверять полиморфно.

## Критерии приёмки

- [ ] `go build ./...` и `go vet ./...` без ошибок.
- [ ] `go test -timeout=10s -run=. ./...` проходит.
- [ ] Unit тест: generic function call с выводом типа.
- [ ] Unit тест: generic struct с полями разных типов.
- [ ] Unit тест: generic enum с параметризованным вариантом.
- [ ] `testdata/phase2/valid/` → `blink check` exit 0.
- [ ] `testdata/phase2/invalid/` → `blink check` exit 1.
