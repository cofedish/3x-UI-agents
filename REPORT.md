# REPORT

## Архитектура тем
- Root темы: класс темы на `body` и `#app` (`light|dark|lucifer`) через `syncThemeClasses`.
- Ultra-dark: активируется только для `dark` через `html[data-theme='ultra-dark']`.
- Токены: базовые `--color-*` в `web/assets/css/theme-tokens.css`, значения тем из `--dark-color-*` и `--lucifer-color-*` в `web/assets/css/custom.min.css`.
- Подключение CSS: `web/html/common/page.html` (`antd.min.css` -> `custom.min.css` -> `theme-tokens.css`).
- Ant меню: `themeSwitcher.menuTheme` (`light|dark`) для `a-menu`/`a-layout-sider`.

## Theme switch bug

### Баг "двойного клика"

#### Симптомы
- На ВСЕХ страницах (кроме `/panel/servers`) при первом переключении темы вид "ломался".
- Требовался повторный клик на ту же тему, чтобы она применилась корректно.
- `/panel/servers` - единственная страница где тема переключалась с первого раза.

#### Корневая причина
**Конфликт двойной реактивности Vue.observable + data**

Объект `themeSwitcher` создаётся как `Vue.observable(createThemeSwitcher())` - это делает его реактивным глобально.

Но на страницах `index.html`, `inbounds.html`, `settings.html`, `xray.html`, `login.html` этот же объект добавлялся в `data` Vue instance:

```javascript
const app = new Vue({
  data: {
    themeSwitcher,  // <-- ПРОБЛЕМА: Vue пытается обернуть уже-реактивный объект
    ...
  }
});
```

Когда Vue встречает объект в `data`, он пытается сделать его реактивным через свой Observer. Но `themeSwitcher` уже `Vue.observable`. Это создаёт конфликт, из-за которого getter-ы (`currentTheme`, `menuTheme`) теряют реактивную связь с базовым свойством `theme`.

**Доказательство**: На `servers.html` где `themeSwitcher` НЕ БЫЛ добавлен в `data`, тема переключалась с первого раза.

#### Что сделано

**Удалён `themeSwitcher` из `data` на всех страницах:**
- `web/html/index.html` - убрана строка `themeSwitcher,` из data
- `web/html/inbounds.html` - убрана строка `themeSwitcher,` из data
- `web/html/settings.html` - убрана строка `themeSwitcher,` из data
- `web/html/xray.html` - убраны строки `themeSwitcher,` и `isDarkTheme: themeSwitcher.isDarkTheme,` из data
- `web/html/login.html` - убрана строка `themeSwitcher,` из data

Vue instance всё равно имеет доступ к глобальной переменной `themeSwitcher` в шаблоне. Реактивность обеспечивается через `Vue.observable`, а не через `data`.

---

### Залипшие overlays на /panel/servers

#### Симптомы
- Tooltips, modals, dropdowns на `/panel/servers` оставались в старых цветах после смены темы.
- "Белые" области в таблице и модальных окнах.

#### Причина
1. **Отсутствие `overlay-class-name` на tooltips**: Ant Design рендерит tooltip/popover/modal в отдельный portal-контейнер вне `#app`. Без атрибута `:overlay-class-name="themeSwitcher.currentTheme"` эти порталы не получают класс темы.
2. **Modal с `:class` вместо `:wrap-class-name`**: Для modal нужен `:wrap-class-name` чтобы класс применился к wrapper-элементу портала.
3. **Отсутствие CSS для overlay-селекторов**: В `theme-tokens.css` не было стилей для `.dark` и `.lucifer` (без `body.`) - эти классы добавляются к порталам через `overlay-class-name`.

#### Что сделано

**1. `web/html/servers.html`**
- Добавлен `:overlay-class-name="themeSwitcher.currentTheme"` ко всем `<a-tooltip>` (4 шт)
- Заменён `:class` на `:wrap-class-name` в `<a-modal>`

**2. `web/assets/css/theme-tokens.css`**
Добавлен блок "OVERLAY PORTAL THEME STYLES" (~200 строк) с полным набором стилей для:
- `.dark .ant-tooltip-*`, `.dark .ant-popover-*`, `.dark .ant-modal-*`, `.dark .ant-select-dropdown`, `.dark .ant-dropdown-menu`, `.dark .ant-input/*`
- `.lucifer .ant-tooltip-*`, `.lucifer .ant-popover-*`, `.lucifer .ant-modal-*`, `.lucifer .ant-select-dropdown`, `.lucifer .ant-dropdown-menu`, `.lucifer .ant-input/*`

---

### Проверка

1. Открыть любую страницу (`/panel/`, `/panel/inbounds`, `/panel/servers`, `/panel/settings`)
2. Переключить тему: **light → dark** (ОДИН клик должен сразу применить тему)
3. Переключить: **dark → lucifer** (ОДИН клик)
4. Переключить: **lucifer → light** (ОДИН клик)
5. На `/panel/servers` проверить:
   - Tooltips на кнопках действий (hover на heart/edit/delete)
   - Dropdown фильтра статуса
   - Modal "Add Server" / "Edit Server"
   - Confirm dialog удаления
6. В Lucifer убедиться:
   - Фон винно-тёмный
   - Кнопки primary красные (#e63946)
   - Белый текст

## Чеклист
- /panel/inbounds: light/dark/lucifer
- /panel/servers: light/dark/lucifer
- dropdown/select/popover/confirm в lucifer не белые и не зелёные
- table fixed column без белых пластин
- primary button в lucifer красная
- читаемость текста и placeholder
