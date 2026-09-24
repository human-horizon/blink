# Контекст разработки

[2026-08-31] Проблема: предварительное разрешение generic type alias вызывало ошибку арности при разрешении определения без аргументов → Решение: разделить разрешение базового тела alias (`resolveAliasBase`) и применение аргументов (`resolveAlias`), чтобы generic-параметры оставались placeholder-параметрами до места использования.

[2026-08-31] Проблема: standalone `match` и `if let` теряли expression/pattern semantics → Решение: хранить `MatchExpr`, `PatPath` и `IfExpr.Pattern` в AST; checker проверяет arms и bindings отдельно.
