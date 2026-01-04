# REPORT

## Архитектура тем
- Root темы: класс темы на `body` и `#app` (`light|dark|lucifer`) через `syncThemeClasses`.
- Ultra-dark: активируется только для `dark` через `html[data-theme='ultra-dark']`.
- Токены: базовые `--color-*` в `web/assets/css/theme-tokens.css`, значения тем из `--dark-color-*` и `--lucifer-color-*` в `web/assets/css/custom.min.css`.
- Подключение CSS: `web/html/common/page.html` (`antd.min.css` -> `custom.min.css` -> `theme-tokens.css`).
- Ant меню: `themeSwitcher.menuTheme` (`light|dark`) для `a-menu`/`a-layout-sider`.

### Lucifer цветовая палитра (wine-red)
Определена в `custom.min.css`:
```
--lucifer-color-background: #0d0208      (очень тёмный винный)
--lucifer-color-surface-100: #1a0f14     (тёмный винный)
--lucifer-color-surface-200: #2b1319     (винный)
--lucifer-color-surface-300: #3d1822     (винно-красный)
--lucifer-color-surface-400: rgba(230..) (полупрозрачный)
--lucifer-color-surface-500: #4a1e28     (светлый винный)
--lucifer-color-btn-danger: #b91924      (красный)
--lucifer-color-tag-red-color: #ff495c   (ярко-красный)
```
Primary accent: `#e63946` (определён в theme-tokens.css)

### CSS переменные для порталов
Overlay-компоненты (modal, tooltip, dropdown) рендерятся вне `#app` через Ant Design portals.
Они получают класс темы через `overlay-class-name`/`wrap-class-name`, но НЕ наследуют CSS переменные от `body.dark`/`body.lucifer`.

Решение: CSS переменные дублируются на `.dark` и `.lucifer` классах (без `body.`) в theme-tokens.css.

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

### Наследование CSS переменных в порталах

#### Симптомы
- Primary кнопки в модалках оставались зелёными (#008771) в Lucifer теме вместо красных (#e63946)
- Некоторые компоненты в overlay-порталах не наследовали цвета темы

#### Корневая причина
**CSS переменные не наследуются от body в portal-контейнерах**

CSS переменные определены на `body.dark` и `body.lucifer`:
```css
body.lucifer {
  --color-primary-100: #e63946;  /* red */
  --color-primary: var(--color-primary-100);
}
```

Но portal-компоненты (modal, tooltip, dropdown) рендерятся напрямую в body, а не как дети элемента с классом темы. Они получают класс темы через `overlay-class-name`/`wrap-class-name`, но НЕ наследуют CSS переменные от `body.lucifer`.

В результате:
- Портал получает класс `.lucifer`
- Но `--color-primary` берётся из `:root` (зелёный #008771), а не из `body.lucifer`

#### Что сделано

**В `web/assets/css/theme-tokens.css` добавлены CSS переменные для `.dark` и `.lucifer` классов:**

```css
/* Для overlay порталов */
.dark {
  --color-bg: var(--dark-color-background);
  --color-surface: var(--dark-color-surface-100);
  --color-primary: var(--color-primary-100);
  /* ... */
}

.lucifer {
  --color-bg: var(--lucifer-color-background);
  --color-surface: var(--lucifer-color-surface-100);
  --color-primary-100: #e63946;
  --color-primary: var(--color-primary-100);
  /* ... */
}
```

**Добавлены стили для primary/danger кнопок в порталах:**
```css
.lucifer .ant-btn-primary {
  background-color: #e63946;
  border-color: #e63946;
}
.lucifer .ant-btn-danger {
  background-color: var(--lucifer-color-btn-danger);
}
```

---

### Исправление модалов (Session 2)

#### Проблема
Все модалы в проекте использовали `:class="themeSwitcher.currentTheme"` вместо `:wrap-class-name`. Атрибут `:class` на `<a-modal>` не работает для portal-контейнеров - нужен `:wrap-class-name`.

#### Что сделано

**Исправлено 21 модал:**

`web/html/modals/`:
- client_modal.html
- text_modal.html
- client_bulk_modal.html
- inbound_info_modal.html
- qrcode_modal.html
- prompt_modal.html
- warp_modal.html
- dns_presets_modal.html
- xray_dns_modal.html
- two_factor_modal.html
- xray_balancer_modal.html
- xray_rule_modal.html
- inbound_modal.html
- xray_fakedns_modal.html
- xray_outbound_modal.html
- xray_reverse_modal.html

`web/html/index.html`:
- version-modal
- log-modal
- xraylog-modal
- backup-modal
- cpu-history-modal

**Изменение:**
```html
<!-- До -->
<a-modal :class="themeSwitcher.currentTheme">

<!-- После -->
<a-modal :wrap-class-name="themeSwitcher.currentTheme">
```

---

### Структурное сравнение servers.html vs inbounds.html

| Аспект | servers.html | inbounds.html |
|--------|--------------|---------------|
| Layout header | CSS класс `.servers-page__header` | Inline style |
| Card padding | CSS класс `.servers-page__card` | Inline `:style` |
| Statistics | Нативный `a-statistic` | `a-custom-statistic` |
| Select theming | `:dropdown-class-name` ✓ | Частично |
| Tooltip theming | `:overlay-class-name` ✓ | `:overlay-class-name` ✓ |
| Modal theming | `:wrap-class-name` ✓ | `:wrap-class-name` ✓ |

**Вывод**: servers.html использует более правильный подход с CSS классами вместо inline styles. Стили определены в theme-tokens.css.

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

## Чеклист верификации

### Основные страницы
- [ ] `/panel/` (index): light/dark/lucifer переключается с первого клика
- [ ] `/panel/inbounds`: light/dark/lucifer переключается с первого клика
- [ ] `/panel/servers`: light/dark/lucifer переключается с первого клика
- [ ] `/panel/settings`: light/dark/lucifer переключается с первого клика
- [ ] `/panel/xray`: light/dark/lucifer переключается с первого клика
- [ ] `login`: light/dark переключается

### Lucifer тема - визуал
- [ ] Фон страницы винно-тёмный (#0d0208)
- [ ] Primary кнопки красные (#e63946)
- [ ] Danger кнопки тёмно-красные (#b91924)
- [ ] Текст белый/светлый
- [ ] Sidebar меню с красным accent при selected

### Overlay компоненты в Lucifer
- [ ] Tooltip на /panel/servers (hover на кнопки действий) - винный фон
- [ ] Select dropdown (фильтр статуса) - винный фон
- [ ] Modal "Add Server" - винный фон, красная primary кнопка
- [ ] Confirm dialog - винный фон, красная/тёмно-красная кнопки

### Таблицы
- [ ] Fixed columns (actions) без белых пластин
- [ ] Header таблицы в цвете темы
- [ ] Hover строк в цвете темы

### Inputs/Forms
- [ ] Input border при hover/focus - красный в lucifer
- [ ] Placeholder читаемый (серый текст)
- [ ] Select dropdown items при hover - винный

### Login страница
- [ ] Popover настроек в правильной теме
- [ ] Language select dropdown в правильной теме
- [ ] Input fields в правильной теме
