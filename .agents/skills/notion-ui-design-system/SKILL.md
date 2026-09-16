---
name: notion-ui-design-system
description: >-
  Provides comprehensive guidelines, color palettes, typography tokens, component structures, 
  and UI/UX best practices for building clean, distraction-free interfaces in the style of Notion. 
  Use when designing or refactoring web or desktop apps to look minimalist, elegant, 
  highly functional, with Notion-like sidebars, tables, callouts, and pastel tags.
---

# Notion-Style UI/UX Design System

This skill guides the design and implementation of user interfaces inspired by **Notion**: minimalist, distraction-free, elegant, highly productive, and typography-driven.

---

## 🎨 1. Design Philosophy & Aesthetic Core

1. **Content First, Zero Visual Clutter:**
   - Chrome (borders, sidebars, buttons) must never compete with data.
   - Use whitespace generously instead of heavy borders and dividing lines.
   - Avoid aggressive drop-shadows, 3D skeuomorphism, or bright neon gradients.

2. **Monochrome Neutral Foundation with Pastel Accents:**
   - Backgrounds are warm whites or deep neutral blacks.
   - Text has clear hierarchical contrast: primary (near black), secondary (muted charcoal), tertiary (soft gray).
   - Accents and badges use Notion's signature **soft pastel background pills** with darkened text.

3. **Restrained Micro-Interactions:**
   - Hover effects should be subtle background color shifts (`rgba(55, 53, 47, 0.06)`).
   - Transitions are fast and crisp (`120ms - 150ms ease-in-out`), never slow, bouncy, or playful.

---

## 🎨 2. Color System & Design Tokens

### Light Theme (Default Notion)
```css
:root {
  /* Canvas & Surfaces */
  --notion-bg: #ffffff;
  --notion-bg-sidebar: #f7f6f5;
  --notion-bg-hover: rgba(55, 53, 47, 0.06);
  --notion-bg-active: rgba(55, 53, 47, 0.09);
  --notion-bg-card: #ffffff;
  --notion-bg-subtle: #fbfbfa;

  /* Borders & Dividers */
  --notion-border: rgba(55, 53, 47, 0.09);
  --notion-border-strong: rgba(55, 53, 47, 0.16);

  /* Typography */
  --notion-text: #37352f;
  --notion-text-muted: rgba(55, 53, 47, 0.65);
  --notion-text-subtle: rgba(55, 53, 47, 0.45);

  /* Elevation */
  --notion-shadow-sm: 0 1px 2px rgba(15, 15, 15, 0.05);
  --notion-shadow-popover: 0 4px 16px rgba(15, 15, 15, 0.08), 0 0 0 1px rgba(15, 15, 15, 0.05);
}
```

### Dark Theme (Notion Dark)
```css
[data-theme="dark"], .theme-dark {
  --notion-bg: #191919;
  --notion-bg-sidebar: #202020;
  --notion-bg-hover: rgba(255, 255, 255, 0.055);
  --notion-bg-active: rgba(255, 255, 255, 0.085);
  --notion-bg-card: #252525;
  --notion-bg-subtle: #1e1e1e;

  --notion-border: rgba(255, 255, 255, 0.094);
  --notion-border-strong: rgba(255, 255, 255, 0.16);

  --notion-text: #e6e6e6;
  --notion-text-muted: rgba(255, 255, 255, 0.65);
  --notion-text-subtle: rgba(255, 255, 255, 0.45);

  --notion-shadow-sm: 0 1px 2px rgba(0, 0, 0, 0.3);
  --notion-shadow-popover: 0 4px 16px rgba(0, 0, 0, 0.4), 0 0 0 1px rgba(255, 255, 255, 0.1);
}
```

### Signature Notion Tag Colors (Pill Badges)
Always use these exact pastel pairings for status, types, and tags:
- **Gray:** Light `bg: #f1f1ef; color: #5a5a58;` | Dark `bg: #2f2f2f; color: #9b9a97;`
- **Brown:** Light `bg: #f4eeee; color: #785e4e;` | Dark `bg: #43332c; color: #bca08d;`
- **Orange:** Light `bg: #faece3; color: #ac5c29;` | Dark `bg: #492f24; color: #d9730d;`
- **Yellow:** Light `bg: #fbf3db; color: #8f6b00;` | Dark `bg: #56431f; color: #cb912f;`
- **Green:** Light `bg: #edf3ec; color: #3b6e4c;` | Dark `bg: #243d30; color: #448361;`
- **Blue:** Light `bg: #e7f3f8; color: #2b5d84;` | Dark `bg: #203c4b; color: #337ea9;`
- **Purple:** Light `bg: #f4f0f7; color: #694085;` | Dark `bg: #3c2d49; color: #9065b0;`
- **Pink:** Light `bg: #f9edf3; color: #9d3d68;` | Dark `bg: #4e2c3c; color: #c14c8a;`
- **Red:** Light `bg: #fdebec; color: #a93c3c;` | Dark `bg: #522e2a; color: #d44c47;`

---

## 📐 3. Typography & Spacing Rules

- **Font Family:**
  ```css
  font-family: ui-sans-serif, -apple-system, BlinkMacSystemFont, "Segoe UI", "Inter", Helvetica, "Apple Color Emoji", Arial, sans-serif;
  ```
- **Scale:**
  - Page Title (H1): `26px - 30px`, font-weight `700`, line-height `1.2`
  - Section Header (H2): `20px - 22px`, font-weight `600`, line-height `1.3`
  - Subheading (H3): `16px - 17px`, font-weight `600`, line-height `1.35`
  - Body Text: `14px`, font-weight `400`, line-height `1.5`
  - Small / Metadata / Captions: `12px`, font-weight `500`, line-height `1.4`
- **Border Radius Standards:**
  - Small (pills, badges, buttons, inputs): `4px` to `6px`
  - Cards & Modals: `8px`
  - Never use overly round pill shapes (e.g. `border-radius: 9999px`) for large cards or dialogs.

---

## 🧱 4. Key Component Patterns

### 1. The Collapsible Sidebar
- Flat background (`--notion-bg-sidebar`) with a subtle `1px solid var(--notion-border)` on the right.
- Top section: Workspace/Company switcher with company icon/avatar and caret.
- Navigation items:
  - Left icon (clean outline SVG or emoji) + label + optional pill counter on the right.
  - Hover background: `var(--notion-bg-hover)` with `border-radius: 4px`.
  - Active item: slightly darker background `var(--notion-bg-active)` and bold text.

### 2. The Page Header
- Optional top banner or clean minimalist breadcrumb (`AutoSire / Consultas / CPE`).
- Large icon (`36px - 40px` emoji or styled glyph).
- Clean, bold page title (`H1`).
- Optional property metadata row (e.g. `RUC: 20...`, `Estado: Activo`, `Periodo: 2026-09`).

### 3. Notion Callout Block
- Used for instructions, tips, status warnings, and SUNAT notices.
- Layout:
  ```html
  <div class="notion-callout notion-callout-blue">
    <div class="notion-callout-icon">💡</div>
    <div class="notion-callout-content">
      <strong>Información:</strong> Recuerda verificar las credenciales SOL antes de iniciar la sincronización.
    </div>
  </div>
  ```
- Styling: `display: flex; gap: 12px; padding: 12px 16px; border-radius: 6px; font-size: 14px;` with subtle pastel background and matching border.

### 4. Notion-Style Data Tables (Databases)
- Flat, dense, highly readable.
- Header row:
  - Subtle text (`12px`, font-weight `500`, color `var(--notion-text-muted)`).
  - Bottom border `1px solid var(--notion-border)`.
  - No vertical grid lines; space columns with padding (`8px 12px`).
- Data rows:
  - Height `36px - 42px`.
  - Subtle row hover (`var(--notion-bg-hover)`).
  - Status and document types rendered as pastel pills (e.g. `01 Factura`, `03 Boleta`, `07 NC`).
- Inline action buttons appear on row hover (Download PDF, XML, CDR).

### 5. Filter & Search Toolbar
- Located directly above tables.
- Left side: View tabs (e.g., "Todos", "Facturas", "Boletas", "Pendientes") styled as flat underline or pill tabs.
- Right side: Search input with magnifying glass icon + "Filtrar" and "Exportar" buttons.

---

## 🚀 5. Implementation Checklist for AutoSire
When styling or updating AutoSire:
1. Ensure the sidebar matches Notion's workspace layout with seamless company switching.
2. Replace generic alert boxes with Notion callout cards.
3. Transform CPE and SIRE tables into Notion database views with pastel tag pills.
4. Ensure full light and dark mode support with Notion's exact hex codes.
5. Keep buttons flat, with 4px border radius and 1px subtle border on secondary buttons.
