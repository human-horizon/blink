# blink: ультрабыстрый type checker для Rust

## Контекст

Существующий `rustc` мощный, но медленный для инкрементальной работы. Нужен компактный type checker, который проверяет репозиторий на Rust "моментально" — в духе TCC для C. Проект начинается с чистого листа, язык реализации — Go, CLI-инструмент.

Целевая скорость — type-check отдельного файла за наносекунды-микросекунды, небольшого репозитория — за миллисекунды. Это требует экстремальной оптимизации: single-pass/инкрементальная архитектура, предвыделенные буферы, минимум аллокаций, ленивые вычисления, параллельная обработка файлов.

## Цель

Построить `blink` — полноценный type checker для Rust (generics, lifetimes, traits, impl, модули, макросы, unsafe) с CLI `blink check <path>`, который проверяет репозиторий за миллисекунды. MVP этой сессии — фаза 1: core + парсер + базовый checker + CLI + инфраструктура тестов. Полный Rust — дорожная карта ниже.

## Что изменится

Новый репозиторий `~/Space/Projects/HumanHorizon/blink`:

```
blink/
├── cmd/blink/main.go          // точка входа CLI
├── internal/source/           // чтение файлов, FileSet, позиции
├── internal/lexer/              // лексер Rust
├── internal/parser/             // парсер и AST
├── internal/ast/                // узлы AST
├── internal/types/              // представление типов, traits, lifetimes
├── internal/checker/            // type & borrow checker
├── internal/resolve/            // имена, модули, use, crates
├── internal/macro/              // развёртывание declarative macros
├── internal/unsafe_/            // проверка unsafe-блоков
├── internal/diag/               // диагностики
├── internal/perf/               // бенчмарки и профилировщик
├── specs/SPEC.md              // эта спецификация
└── testdata/                  // примеры Rust-файлов
```

## Детали реализации

### Поддерживаемое подмножество Rust (дорожная карта)

#### Фаза 1 — Foundation (MVP этой сессии)
- Базовые типы: `i32`, `bool`, `()`.
- Выражения: литералы, переменные, бинарные операции, блоки, `let`, `if/else`, `while`, `return`, вызовы функций.
- Объявления: `fn` (с аргументами и возвращаемым типом), `struct`, `enum`.
- Single file или все `.rs` файлы в директории как одно пространство имён.
- CLI `blink check <path>` и инфраструктура тестов.

#### Фаза 2 — Generics
- Generic параметры для функций, структур, enum'ов.
- Ограничения `where`, простые bound'ы.
- Monomorphization-ready type checking.

#### Фаза 3 — Lifetimes & Borrow Checker
- Проверка ссылок `&T` и `&mut T`.
- Lifetime elision.
- Правила ownership/borrow/move для локальных переменных.

#### Фаза 4 — Traits & Impl
- Определения `trait`, `impl Trait for T`, inherent `impl`.
- Method resolution, associated types, associated functions.
- Орфан-правила и coherence (упрощённые).

#### Фаза 5 — Modules & Crates
- `mod`, `use`, `pub`, `crate`, `super`, `self`.
- Иерархия файлов: `foo.rs` ↔ `foo/mod.rs`.
- `extern crate` / `Cargo.toml` integration (опционально).

#### Фаза 6 — Macros
- Declarative macros `macro_rules!` с развёртыванием перед type check.
- Позиции ошибок маппятся на исходный код.

#### Фаза 7 — Unsafe
- Проверка `unsafe` блоков и функций.
- Raw pointers `*const T`, `*mut T`.
- `unsafe impl` / `unsafe trait`.

### Архитектура (целевая)

1. **source**: читает дерево `.rs` файлов, строит `FileSet`.
2. **lexer**: преобразует файл в поток токенов с позициями.
3. **macro expander**: разворачивает `macro_rules!` (фаза 6).
4. **resolve**: строит карту модулей, импортов, видимости имён.
5. **parser**: строит AST.
6. **collect**: регистрирует типы, функции, traits, impls.
7. **checker**: type checking, lifetime checking, trait resolution.
8. **diag**: вывод ошибок в формате `file.rs:line:col: error: ...`.

### CLI

```bash
blink check <path>
```

- `<path>` — директория с `.rs` файлами.
- Exit code `0`, если ошибок нет.
- Exit code `1`, если есть хотя бы одна ошибка.
- Вывод диагностик в `stderr`.

### Производительность

- Предвыделенные буферы под токены и AST-узлы.
- Позиции — компактные offset'ы, строки вычисляются лениво.
- Type checking — однопроходный по возможности.
- Параллельная обработка файлов через `sync.Pool` и горутины (фаза 5+).
- Целевые метрики:
  - один маленький файл (< 100 строк) — < 1 мс;
  - репозиторий 100 файлов по ~100 строк — < 10 мс;
  - полноценный crate — стремимся к < 100 мс.

## Риски и ограничения

Полный Rust type checker — это масштаб задачи, сопоставимый с `rustc`. Даже фаза 1 требует корректной реализации лексера, парсера и базового checker'а. Сроки каждой фазы зависят от сложности фич. Фаза 1 — реалистична за одну сессию; фазы 2–7 — последующие итерации.

## Критерии приёмки (Фаза 1)

- [ ] `go build ./...` собирает проект без ошибок.
- [ ] `go vet ./...` проходит без предупреждений.
- [ ] `go test ./...` проходит все unit и интеграционные тесты.
- [ ] `blink check` на `testdata/phase1/valid/` возвращает exit code `0`.
- [ ] `blink check` на `testdata/phase1/invalid/` возвращает exit code `1` и печатает диагностики с позициями.
- [ ] Бенчмарк `BenchmarkCheckSmallRepo` показывает среднее время < 10 мс для 100 синтетических файлов.
