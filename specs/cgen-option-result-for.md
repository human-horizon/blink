# Codegen: for-циклы и Option/Result runtime (этап 8)

## For-циклы (`cgen_for.go`)
- `for (p, q) in arr` — материализация не нужна (ident-массив): 
  `for (size_t it = 0; it < N; it++) { <bindings через patCond(arr[it])>; body }`.
- `for i in 0..n` — `for (int32_t i = 0; i < n; i++) { body }` (из
  `parseCondExpr`-ограничения: `0..n` парсится корректно).
- Vec/slice/итераторы — `/* unsupported for iterable */` (ждут std runtime).
- Интеграция: `stmt()` case ForStmt, `collectBlock` обход.

## Option/Result runtime (`cgen_adt.go`)
Представление — tagged union, имя структуры по типам аргументов:
```c
struct option_int32_t {
    int32_t tag;  // None=0, Some=1
    union option_int32_t__payload { char none; int32_t some; } payload;
};
struct result_int32_t_int32_t { /* Ok=1 (ok), Err=2 (err) */ };
```
- регистрация: `collectAdts()` (обход exprTypes/сигнатур/полей) + ленивая в
  `cType`/`adtTypeName`; эмит `emitAdtDefs` до forward-деклараций.
- конструкторы `Some(x)/Ok(x)/Err(x)` (Ident и `Option::Some(x)`) →
  designated-initializer; `None` (Ident/Path) → `{ .tag = 0 }`.
- методы (интерцепт в `callExpr` до name-mangling): `unwrap` (abort при
  wrong tag), `is_some/is_none/is_ok/is_err` — statement-expression с temp.
- паттерны `Some(n) => n` / `Ok(v)` / `Err(_)` / `None` — tag-сравнение +
  биндинг payload-поля (checker: `checkPathPattern` ADT-ветка; cgen:
  `pathPatCond` ADT-ветка).

## Семантика заглавных идентификаторов-паттернов (checker + cgen)
`match o { None => ... }` — `None` больше НЕ binding: capitalized PatIdent,
именующий unit-вариант типа (`isVariantPattern`), — path-паттерн. То же для
unit-вариантов пользовательских enum. Exhaustiveness (`collectCovered`)
учитывает такие arm'ы как покрытие варианта.

## Ожидаемый тип в if/else
`checkExprExpected` проверяет обе ветки `if/else` с expected-типом —
`fn f(v) -> Option<i32> { if v > 10 { Some(v) } else { None } }` работает
(раньше ветки проверялись в Unit-контексте → каскад ошибок).

## Мелочи
- `declString`: массивы в `let` объявляются правильно (`T name[N] = {..}`,
  не `T[N] name`) — фиксит `let pairs = [(1,2),(3,4)];`.
- `#include <stdlib.h>` для abort().
- Осторожно с `go build | head` — pipe скрывает код выхода (старый бинарь
  маскировал ошибки компиляции).

## Проверка
- `testdata/run/valid/adt42` (Option+Result+unwrap+is_none+паттерны) — exit 42.
- `testdata/run/valid/match42` (enum payload + match + for-циклы) — exit 42.
- Unit: `TestOptionPatternBinding`, `TestIfElseInfersOptionTail`.
- Go: `TestBuildRunAdtExit42`, `TestBuildRunMatchExit42`, `TestBuildRunExit42`.
- ALL GREEN: test/vet/build + git diff --check.
