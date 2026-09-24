# Обобщённая модель стандартной библиотеки

## Проблема

Текущий `builtins.go` и `types.go` содержат Bevy-специфичные заглушки и
wildcard-релаксации:
- hardcoded таблица `builtinMethods` по имени типа;
- Bevy-типы (`ComponentId`, `ArchetypeId`, `Entry`, `OccupiedEntry`,
  `SparseSet`, `ComponentIndex` и т.д.);
- `isUsizeStub`/`isOpaqueIntStub` — приравнивание usize/u32 и Bevy-newtype к i32;
- `Generic{"_"}` wildcard, `Self` wildcard, Option→bool, Iterator→Vec/Slice,
  Vec↔Array, Named↔Applied.

Это не соответствует спецификации Rust. Требуется обобщённая, корректная модель
стандартной библиотеки.

## Целевая модель

### Generic-типы с настоящими параметрами

Каждый std-тип — это `Applied` с параметрами, а не `Named`-заглушка:

| Тип | Параметры |
|-----|-----------|
| `Vec<T>` | `T` |
| `Option<T>` | `T` |
| `Result<T,E>` | `T`, `E` |
| `HashMap<K,V>` | `K`, `V` |
| `Box<T>` | `T` |
| `String` | — |
| `Iterator<Item=T>` | `Item` (associated type) |

### Trait-based method resolution

Методы std-типов определяются через traits, а не hardcoded таблицу:

- `Vec<T>: Deref<Target=[T]>` — `&Vec<T>` → `&[T]`.
- `Vec<T>: IntoIterator<Item=T>` — `for x in vec`.
- `Option<T>`/`Result<T,E>` — методы через inherent impl.
- `Iterator<Item=T>` — методы через trait `Iterator`.

### Удаление Bevy-специфичных типов

Из `builtins.go` удаляются: `ComponentId`, `BundleId`, `Entity`,
`EntityLocation`, `ArchetypeId`, `TableId`, `TableRow`, `ArchetypeRow`,
`ArchetypeFlags`, `ComponentInfo`, `NonMaxU32`, `SparseSet`,
`ImmutableSparseSet`, `ComponentIndex`, `Edges`, `Entry`, `OccupiedEntry`,
`VacantEntry`, `Archetype`, `Archetypes`, `Bundle`, `Component`, `Components`,
`Observer`, `Observers`, `SparseArray`, `Event`, `EventKey`, `StorageType`,
`ComponentStatus`, `World`, `Query`, `QueryState`.

### Удаление wildcard-релаксаций из types.go

- `isUsizeStub`/`isOpaqueIntStub` — удалить; usize/u32 — отдельные типы.
- `Generic{"_"}` wildcard — удалить; placeholder должен быть типизирован.
- `Self` wildcard — удалить; `Self` резолвится через associated types.
- Option→bool, Iterator→Vec/Slice, Vec↔Array, Named↔Applied — удалить.

## Реализация (этап 0)

### Тип-конструкторы

В `types.go` добавляется `TypeConstructor` — отдельный тип для generic-конструктора
(`Vec`, `Option`, `Result`, `HashMap`, `Box`). Инстанциация — `Applied{Base: TypeConstructor, Args: [...]}`.

```go
type TypeConstructor struct {
    Name   string
    Params []string // имена generic-параметров
}
```

### Trait-based method resolution

В `internal/checker/stdlib.go`:

```go
type stdTrait struct {
    name    string
    methods map[string]*stdMethod
}

type stdType struct {
    name   string
    params []string
    traits []string // имена реализованных traits
}
```

Метод резолвится по receiver-типу: имя конструктора → traits → метод. Generic-параметры
подставляются из аргументов receiver (`Vec<i32>` → `T=i32`).

### Ключевые traits

- `Deref<Target=[T]>` для `Vec<T>` и `Box<T>` — `&Vec<T>` → `&[T]`.
- `IntoIterator<Item=T>` для `Vec<T>` — `for x in vec`.
- `Iterator<Item=T>` — методы `next`, `map`, `filter`, `collect` и т.д.
- `Default` — `Default::default()`.

## Приёмка

- [ ] `Vec<T>`, `Option<T>`, `Result<T,E>`, `HashMap<K,V>`, `Box<T>` — настоящие
      generic-типы с параметрами.
- [ ] Method resolution через traits, а не hardcoded таблицу.
- [ ] Bevy-специфичные типы отсутствуют в коде.
- [ ] Wildcard-релаксации удалены из `types.go`.
- [ ] `go test/vet/build` зелёные.
