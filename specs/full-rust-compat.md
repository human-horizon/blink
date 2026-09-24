# Полная совместимость с Rust (без костылей)

## Статус и честная оценка

Текущий `blink` — Rust-подобный subset-checker (~8200 строк Go), а не полноценный
Rust compiler. `builtins.go` содержит Bevy-специфичные заглушки и hardcoded
таблицы методов — это «костыли», которые требуется удалить.

Полная совместимость с Rust stable + edition 2024 (parsing, type checking,
borrow checking, macros, unsafe, const generics, traits, diagnostics) — это
многолетний проект уровня команды. Данный документ фиксирует направление и
этапы, а не обещание завершить за одну сессию.

## Целевая архитектура

```
source → lexer → parser → AST → [macro expansion] → type checker
      → borrow checker → const evaluator → C99 codegen → TinyCC
```

Ключевое отличие от текущего состояния: **никаких Bevy-специфичных типов и
wildcard-правил**. Стандартная библиотека моделируется обобщённо и корректно.

## Этапы

### Этап 0 — Фундамент: обобщённая модель std-библиотеки
- Generic-типы с настоящими параметрами: `Vec<T>`, `Option<T>`, `Result<T,E>`,
  `HashMap<K,V>`, `Box<T>`, `String`, `Iterator<Item=T>`.
- Trait-based method resolution вместо hardcoded `builtinMethods`.
- Удаление Bevy-специфичных типов (`ComponentId`, `ArchetypeId`, `Entry`,
  `OccupiedEntry`, `SparseSet`, `ComponentIndex` и т.д.).
- Критерий: `go test/vet/build` зелёные; Bevy-специфичные имена отсутствуют.

### Этап 1 — Полный type system
- Traits: bounds, associated types, associated consts, supertraits, coherence.
- Generics: inference (Hindley–Milner), variance, higher-ranked lifetimes.
- Const generics: `[T; N]`, `const N: usize`.
- Критерий: корректная проверка типов без wildcard-релаксаций.

### Этап 2 — Полный borrow checker
- NLL (non-lexical lifetimes), regions, variance.
- Move semantics, partial moves, closures capturing.
- Критерий: корректные borrow-ошибки и их отсутствие на валидном коде.

### Этап 3 — Полный macro system
- `macro_rules!` с metavariables, repetitions, hygiene.
- Процедурные макросы (derive, attribute, function-like).
- Критерий: `vec![...]`, `println!`, derive-макросы работают.

### Этап 4 — Полный const evaluation
- `const fn`, const generics, const blocks.
- Критерий: константные выражения вычисляются на этапе компиляции.

### Этап 5 — Полный pattern matching
- Exhaustiveness, or-patterns, binding modes, slice/range patterns.
- Критерий: неисчерпывающие match диагностируются.

### Этап 6 — Полный unsafe
- Raw pointers, deref, unsafe fn/impl, invariants.
- Критерий: unsafe-операции проверяются по правилам Rust.

### Этап 7 — Diagnostics
- Корректные позиции (spans), сообщения об ошибках по образцу rustc.
- Критерий: ошибки указывают точное место и причину.

### Этап 8 — C99/TinyCC backend
- Полная кодогенерация для всех конструкций.
- Критерий: `blink build` компилирует валидный Rust в C99 и запускает TinyCC.

## Принципы
- Никаких Bevy-специфичных типов и wildcard-правил.
- Каждое правило — по спецификации Rust, с регрессионным тестом.
- Каждый этап завершается зелёным CI и обновлённой спецификацией.
