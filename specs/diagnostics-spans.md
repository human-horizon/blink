# Диагностика: реальные spans (этап 7 — ядро)

## Проблема
Все диагностики черекера печатались как `path:1:1`, потому что:
1. `Checker.errorf` жёстко писал `1, 1`, игнорируя аргумент `pos`;
2. `PosOf(n)` полагался на интерфейс `GetPos()`, который не был реализован
   ни для одного AST-узла → всегда 0;
3. у черекера не было содержимого файлов для перевода offset→line:col.

## Реализация
- **`internal/ast/ast.go`**: метод `GetPos() Pos` добавлен всем узлам
  (decl/expr/stmt/pattern/type); `ExprStmt.GetPos` делегирует вложенному
  выражению. Новый `ast.PosOf(Node) Pos`.
- **`internal/checker/checker.go`**:
  - поля `sources [][]byte` + `lineCache map[int][]int`;
  - `SetSources([][]byte)` — содержимое файлов, выровненное с `Files/Paths`;
  - `errorf(pos, ...)` теперь вычисляет `(line, col)` через
    `lineCol(currentIdx, int(pos))`: бинарный поиск по_starts_строк
      (ленивый кэш на файл); offset — 0-based байтовый (как в лексере),
      line/col — 1-based.
  - Без `SetSources` (юнит-тесты) — фолбэк 1:1, обратная совместимость.
- **`cmd/blink/main.go`**: `loadFile/loadModules` возвращают `sources`,
  `checkPath/buildPath` вызывают `chk.SetSources(sources)`.

## Эффект
```
/tmp/orpat/main.rs:19:18: error: expected `i32`, found `bool`
```
(раньше всегда 1:1). Тест `TestDiagnosticSpanPosition` проверяет точные
line:3 col:19.

## Не сделано (остаток этапа 7)
- Сообщения форматом 1-в-1 как rustc (стиль, подсветка, code frames с
  тирелками под токеном).
- Многоточечные spans (primary + secondary labels), `note`/`help`-подсказки.
- Пул ошибок: часть вызовов `errorf` передаёт `PosOf(чего-то)` = 0 для
  синтетических узлов (builtin-выражения) — остаётся 1:1 по существу.

## Критерии
- `go test/vet/build` зелёные; `git diff --check` чистый.
- CLI-проверка: ошибка в строке 19 → `:19:18` ✓ (эмуляция /tmp/orpat).
