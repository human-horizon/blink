# Codegen: match и enum с payload (этап 8 — старт)

## Сделано
`blink build/run` теперь кодогенерирует `match`-выражения и enum-variants с
payload (internal/checker/cgen_match.go, интеграция в cgen.go).

## Представления
- **Unit-enum** (все варианты без полей): как раньше — `int` +
  `#define E__Variant <idx>`.
- **Enum с payload**: tagged union:
  ```c
  struct Status {
      int32_t tag;
      union Status__payload {
          struct Status__Blocked { int32_t f0; } Blocked;
      } payload;
  };
  ```
  - конструктор: `((struct Status){ .tag = 1, .payload = { .Blocked = { .f0 = 20 } } })`
  - unit-вариант того же enum: `((struct Status){ .tag = 0 })`
  - имена полей: struct-вариант → по именам, tuple → `fN`.
  - `#define`-ы вариантов для payload-enum НЕ эмитятся (конфликт имён).
  - определения payload-enum идут ПЕРЕД forward-декларациями (TinyCC не
    принимает прототипы с incomplete struct by-value).

## Паттерны → C
`match` → statement-выражение TinyCC: материализация scrutinee `__m`, цепочка
`if (cond) { binds; __r = body; } else ...`, затем `__r`:
- wildcard/ident → `1` + binding-объявление (`ref x` → `T x = &(__m)`);
- literal `5`/`-3`/`true` → `__m == 5` (новый узел **PatLit** — раньше
  литералы были молча wildcards!);
- range `10..=20` → `(__m >= 10 && __m <= 20)`;
- or `1 | 2` → дизъюнкция условий;
- tuple `(3, y)` → конъюнкция по `.fN` + bindings;
- `Enum::V(sub...)` user-payload → `__m.tag == V` + рекурсивный биндинг
  `__m.payload.V.fN`; unit → `__m == E__V`;
- struct-pattern `Point { x, y: _ }` → tag + binding по именам (enum) или
  `1` + bindings (обычный struct).
- Не поддержаны: builtin-enum паттерны (`Some(x)`/`Ok(x)`), slice-patterns,
  строковые литералы → константа `0` с комментарием (проверку типов это не
  ломает, ветка просто не срабатывает).

## Проверка
- Фикстура `testdata/run/valid/match42/main.rs`: payload enum + or + range +
  tuple + unit — **exit 42** через TinyCC ✓.
- Тест `TestBuildRunMatchExit42` + прежние (exit42, phase1–11) зелёные.
- `go test/vet/build` + `git diff --check` — ALL GREEN.

## Остаток этапа 8
- match guard'ы (`if ...` в arm — AST-узла нет), `Some/Ok` runtime
  представление (Option/Result в std currently no C-repr),
- for-циклы в cgen (нет ForStmt case), slice/Vec runtime,
- struct↔enum взаимные циклы, monomorphization generic-enum.

## Бонус: restriction semantics для условий (parser)
`match Color::Red { ... }` / `if Flag { ... }` раньше ломались: `{` после
capitalized-пути съедался как struct literal. Добавлен `Parser.condDepth`:
- `parseCondExpr()` (match scrutinee, if/if-let RHS, while cond, for iter)
  инкрементит глубину;
- в postfix-цикле `LBrace` при `condDepth > 0` делает break;
- глубина обнуляется на границах скобок (call args, индексы, tuple/paren,
  array literal), как в Rust — `foo(Bar { .. })` внутри условия разрешён.
Тесты: `TestMatchUnitEnumScrutinee`, `TestMatchBoolLiteralPattern`.

