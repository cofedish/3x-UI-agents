# REPORT

## Архитектура тем
- Root темы: класс темы на `body` и `#app` (`light|dark|lucifer`) через `syncThemeClasses`.
- Ultra-dark: активируется только для `dark` через `html[data-theme='ultra-dark']`.
- Токены: базовые `--color-*` в `web/assets/css/theme-tokens.css`, значения тем из `--dark-color-*` и `--lucifer-color-*` в `web/assets/css/custom.min.css`.
- Подключение CSS: `web/html/common/page.html` (`antd.min.css` -> `custom.min.css` -> `theme-tokens.css`).
- Ant меню: `themeSwitcher.menuTheme` (`light|dark`) для `a-menu`/`a-layout-sider`.

## Theme switch bug

### Залипшие элементы
- Modals/confirm/popover/dropdown, особенно на `/panel/servers`.
- Табличные хедеры/инпуты в карточке фильтров после смены темы.

### Причина
- `themeSwitcher` был не реактивен для большинства страниц, поэтому классы темы на `#app` и overlay-компонентах оставались от предыдущей темы.
- Перезапись `body.className` стирала сторонние классы и не синхронизировала `#app`/`#message`.
- В lucifer оставались зелёные hardcoded hover/active из `custom.min.css`.

### Что сделано
- Сделал `themeSwitcher` реактивным через `Vue.observable` и централизовал `syncThemeClasses` для `body/#app/#message`.
- Обновил lucifer overrides в `web/assets/css/theme-tokens.css`: красный primary, винные hover/active, меню без зелёных градиентов, нормализованный selection.
- Локальные правки `/panel/servers` (key + классы) теперь реально реагируют на смену темы.

### Проверка
1. Открыть `/panel/servers`.
2. Переключить темы: light -> dark -> lucifer -> dark -> light.
3. Проверить: inputs/select/table header, dropdown/select/popover/confirm, modal.
4. Открыть `/panel/inbounds`, повторить переключения.

## Чеклист
- /panel/inbounds: light/dark/lucifer
- /panel/servers: light/dark/lucifer
- dropdown/select/popover/confirm в lucifer не белые и не зелёные
- table fixed column без белых пластин
- primary button в lucifer красная
- читаемость текста и placeholder
