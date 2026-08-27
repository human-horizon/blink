# Phase 9 — Trait Bounds

## Цель
Добавить trait bounds для обобщённых параметров.

## Синтаксис

```rust
trait Clone {
    fn clone(&self) -> Self;
}

fn duplicate<T: Clone>(x: &T) -> (T, T) {
    (x.clone(), x.clone())
}
```

## Поддержка

- Bounds указываются в угловых скобках: `<T: Trait>`.
- Можно несколько bounds: `<T: Clone + Debug>` (парсится, но проверяется каждый).
- Проверка bound при вызове обобщённой функции.
- Проверка bound в `impl<T: Clone> ...`.

## Ограничения Phase 9
- Нет `where`-clause.
- Bounds только на именованные параметры.
- Associated types и default methods не входят.

## Критерии приёма
- Валидные generic вызовы с bounds проходят.
- Вызов с типом, не реализующим bound, отклоняется.
- Все тесты и `go vet` проходят.
