# Borrow checker: NLL-light (реестр держателей ссылок)

## Проблема (баг прошлой сессии)
Старая модель: `checkUnary` для `&`/`&mut` брал loan, затем `reapplyBorrow` в
let-стейтменте брал его **повторно** → ложная ошибка
`cannot borrow x as mutable more than once at a time` для любого
`let y = &mut x;`. При этом `releaseLoans()` после каждого стейтмента стирал
и re-применённый loan — механизм был сломан в обе стороны (ложные ошибки +
пропущенные реальные).

## Решение
Реестр **держателей ссылок** (`refHolder`) с NLL-подобным liveness:

- `borrowCtx.holders: map[имя переменной]→{target, mut}` — var держит ссылку
  на target.
- Loan берётся **один раз** в `checkUnary` (`&`/`&mut`). `reapplyBorrow`
  удалён.
- `LetStmt`: `registerRefHolder(loans, st.Name, st.Value)` — если value это
  `&`/`&mut` корневой переменной (не `self`).
- `checkBlock` после каждого стейтмента вызывает
  `endStatement(block.Stmts[i+1:], block.Result)`:
  1. сбрасывает statement-level loans;
  2. для каждого живого holder (имя упомянуто в оставшихся стейтментах или
     tail-выражении) **восстанавливает** loan target'а;
  3. мёртвый holder удаляется → заём заканчивается (NLL: borrow ends at last
     use).
- `AssignStmt` (`a = ...`): перед проверкой `removeHolder(a)` (старый заём
  умирает с переприсваиванием), после — `registerRefHolder(a, right)`.
- Liveness-сканер `stmtsMention/exprMentions/blockMentions` покрывает все
  узлы AST; неизвестный узел = консервативно «упоминается» (loan живёт
  дольше — меньше ложных освобождений).
- Вложенные блоки: holder наследуются в дочерний `borrowCtx` (копируются при
  обращении), умирают вместе с областью видимости.
- Параметры замыкания затеняют имя holder (`ClosureExpr.Args`).

## Семантика (что теперь ловится / пропускается)
- `let y = &mut x; *y = 6; let z = x;` — **OK** (NLL, заём y закончился).
- `let y = &mut x; let b = &x; *y = 2;` — ошибка `cannot borrow x as immutable
  because it is mutably borrowed` (y жив).
- `let y = &mut x; x = 5; *y = 2;` — ошибка `cannot assign to x because it is
  mutably borrowed`.
- `let a = &mut x; *a = 3; a = &mut z; ... let b = &x;` — OK (переприсваивание
  a завершило заём x).

## Не сделано (остаток этапа 2)
- Настоящие regions/граф заимствований, variance, lifetimes-в-заимах.
- Partial moves (`let px = p.x; p.y`), closures capturing (замыкания сейчас
  не захватывают borrow-state).
- Поля структур как отдельные заимодержатели (rootVar грубит до корня).

## Критерии / тесты
- `TestLetMutBorrowNoDoubleLoan`, `TestNLLBorrowEndsAfterLastUse`,
  `TestMutableBorrowBlocksSharedMessage`, `TestWriteToTargetWhileBorrowed`,
  `TestReassignmentEndsHolder` + прежние `TestSharedBorrow`,
  `TestSequentialMutBorrows`, `TestMutableBorrowBlocksShared`,
  `TestUseAfterMove`, `TestMutSelfBorrow`.
- `go test/vet/build` зелёные, `git diff --check` чистый.
