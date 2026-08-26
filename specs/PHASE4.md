# blink — Фаза 4: Traits & Impl

## Контекст

Фаза 3 добавила borrow checker. Теперь нужен ещё один столп Rust — traits и impl-блоки. Без них невозможно полноценное ООП-полиморфное программирование в Rust.

## Цель

Добавить поддержку trait-ов, `impl Trait for Type` и inherent `impl Type` с методами. Поддержать вызов методов через `value.method(args)` и `Type::assoc(args)`.

## Что изменится

1. `internal/ast/ast.go` — добавить `TraitDecl`, `ImplDecl`.
2. `internal/parser/parser.go` — парсить `trait Name { fn ... }`, `impl Name for Type { ... }`, `impl Type { ... }`.
3. `internal/checker/checker.go` — коллекция trait/impl, method resolution, проверка тел методов.
4. Тесты + `testdata/phase4/`.

## Детали реализации

### Поддерживаемый синтаксис

```rust
trait Printable {
    fn print(&self) -> i32;
}

struct Point { x: i32, y: i32 }

impl Printable for Point {
    fn print(&self) -> i32 {
        self.x
    }
}

impl Point {
    fn new(x: i32, y: i32) -> Point {
        Point { x: x, y: y }
    }
}

fn main() -> i32 {
    let p = Point::new(1, 2);
    p.print()
}
```

### Правила

- Trait содержит сигнатуры методов без тел.
- `impl Trait for Type` должен реализовать все методы trait.
- `impl Type` — inherent methods, доступные через `Type::method` или `value.method`.
- Method resolution: сначала inherent, затем trait-методы.
- `&self` — первый параметр получает тип `&Type`.
- Без associated types, без default method bodies, без trait bounds.

### Что НЕ входит

- Trait bounds (`T: Trait`).
- Associated types.
- Generic traits.
- Supertraits.
- Orphan rules/coherence (MVP — простая проверка дубликатов).

## Критерии приёмки

- [ ] `go build ./...` и `go vet ./...` без ошибок.
- [ ] `go test -timeout=30s -run=. ./...` с `GOMEMLIMIT=1GiB` проходит.
- [ ] Unit тест: trait method call.
- [ ] Unit тест: inherent method call.
- [ ] Unit тест: ошибка при неполной реализации trait.
- [ ] `testdata/phase4/valid/` → exit 0.
- [ ] `testdata/phase4/invalid/` → exit 1.
