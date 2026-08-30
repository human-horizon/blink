# Восстановление iterator chaining для enumerate

## Контекст

Ранее `enumerate()` возвращал кортеж `(i32, _)`, чтобы деструктурировать `for (idx, x)`. Это ломало method chaining: `.enumerate().map(...)` давал `no method map for tuple`. Bevy использует такие цепочки (`entities.iter().enumerate().map(|(row, &e)| ...)`).

## Цель

`enumerate()` должен возвращать iterator (цепочки `.map/.filter/.collect` сохраняются), а деструктуризация `for (idx, x)` опирается на generic-заглушку элемента в `checkTuplePattern`.

## Что изменится

1. `internal/checker/builtins.go` — `enumerate()` для `Iterator` возвращает `Named{Iterator}` (не кортеж); убрать специфичный `array.enumerate`.
2. `internal/checker/checker.go` — `iteratorElem` для `Applied`/`Array`/`Tuple` возвращает элемент; иначе generic-заглушка; `checkTuplePattern` связывает generic-элементы.
3. `internal/checker/checker_test.go` — `TestForLoopPatternBinding` на массиве кортежей.

## Критерии приёмки

- [ ] `no method map for type tuple` исчезает из `archetype-test`.
- [ ] `GOMEMLIMIT=1GiB go test -timeout=60s ./...` проходит.
- [ ] `go vet ./...` и `go build ./...` проходят.
- [ ] Количество ошибок в `archetype-test` не увеличивается относительно 88 (допустим рост на 1 из-за вскрытия реальных EntityLocation/closure ошибок).
