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
- Tooltips в таблице серверов оставались в старых цветах.

### Причина
1. **Отсутствие `overlay-class-name` на tooltips**: В `servers.html` tooltips не имели атрибута `:overlay-class-name="themeSwitcher.currentTheme"`, поэтому портал-элементы (рендерятся вне `#app`) не получали класс текущей темы.
2. **Modal с `:class` вместо `:wrap-class-name`**: Модалка использовала `:class` который не применяется к wrapper-элементу портала.
3. **Отсутствие CSS для overlay-порталов**: В `theme-tokens.css` не было стилей для селекторов `.dark` и `.lucifer` (без `body.`) - эти классы добавляются к порталам через `overlay-class-name`.
4. **Lucifer тема не стилизована для overlays**: Хотя токены `--lucifer-color-*` были определены правильно в `custom.min.css`, в `theme-tokens.css` не было соответствующих overrides для overlay-компонентов.

### Что сделано

#### 1. `web/html/servers.html`
- Добавлен `:overlay-class-name="themeSwitcher.currentTheme"` ко всем `<a-tooltip>` в таблице серверов (4 tooltip'а в actions column).
- Заменён `:class` на `:wrap-class-name` в `<a-modal>` для правильного применения темы к wrapper-элементу.

#### 2. `web/assets/css/theme-tokens.css`
Добавлен блок "OVERLAY PORTAL THEME STYLES" с полным набором стилей для overlay-компонентов:

**Dark theme overlays** (селектор `.dark`):
- `.ant-tooltip-inner`, `.ant-tooltip-arrow::before`
- `.ant-popover-inner`, `.ant-popover-title`, `.ant-popover-arrow`
- `.ant-select-dropdown`, `.ant-select-dropdown-menu-item`
- `.ant-modal-content/header/body/footer/close-x`
- `.ant-confirm-body .ant-confirm-title/content`
- `.ant-form-item-label>label`
- `.ant-input`, `.ant-input-number`, `.ant-select-selection`
- `.ant-dropdown-menu`, `.ant-dropdown-menu-item`

**Lucifer theme overlays** (селектор `.lucifer`):
- Все те же компоненты с винно-красными цветами
- Красный primary (#e63946) для hover/focus states
- Правильные фоновые цвета из `--lucifer-color-surface-*`
- Белый текст для высокой читаемости

### Проверка
1. Открыть `/panel/servers`.
2. Переключить темы: light -> dark -> lucifer -> dark -> light.
3. Проверить:
   - Tooltips на кнопках действий в таблице (heart, reload, edit, delete)
   - Select dropdown для фильтра по статусу
   - Modal добавления/редактирования сервера (inputs, selects, кнопки)
   - Confirm dialog при удалении сервера
4. Открыть `/panel/inbounds`, повторить переключения.
5. В Lucifer теме убедиться:
   - Фон тёмно-винный (#0d0208 -> #1a0f14 -> #2b1319)
   - Primary кнопки красные (#e63946)
   - Inputs при hover/focus получают красную границу
   - Текст белый и читаемый

## Чеклист
- /panel/inbounds: light/dark/lucifer
- /panel/servers: light/dark/lucifer
- dropdown/select/popover/confirm в lucifer не белые и не зелёные
- table fixed column без белых пластин
- primary button в lucifer красная
- читаемость текста и placeholder
