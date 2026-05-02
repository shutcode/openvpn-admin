# Design System Specification: The Technical Architect

## 1. Overview & Creative North Star
**Creative North Star: The Precision Observatory**
In the world of network infrastructure, data is often chaotic. This design system rejects the cluttered, "dashboard-as-a-spreadsheet" trope in favor of a **Precision Observatory**. The aesthetic moves beyond "clean" into the realm of **High-End Editorial Engineering**. 

We achieve this through a "Mechanical Sophistication" approach: utilizing extreme whitespace, intentional asymmetry in data visualization, and a tonal-first depth model. Instead of a rigid, boxed-in grid, the layout should feel like a series of floating, high-fidelity instruments resting on a bed of soft light. We are not just building an admin panel; we are building a mission control center that feels lightweight, breathable, and authoritative.

---

## 2. Colors & Surface Architecture
The palette is rooted in deep professional stability, using a Material-inspired tonal logic to define hierarchy without visual noise.

### The "No-Line" Rule
**Explicit Instruction:** Designers are prohibited from using 1px solid borders to section off the UI. Containers must be defined solely through background color shifts.
*   **Context:** A `surface-container-low` component sitting on a `surface` background provides enough contrast for the human eye to perceive a boundary without the "boxed-in" feel of a stroke.

### Surface Hierarchy & Nesting
Treat the UI as physical layers of "frosted glass" and "fine paper."
*   **Layer 0 (Base):** `surface` (#f7f9fb) – The canvas.
*   **Layer 1 (Sections):** `surface-container-low` (#f2f4f6) – Use for large secondary areas.
*   **Layer 2 (Interactive Elements):** `surface-container-lowest` (#ffffff) – Use for primary data cards to provide a "pop" against the base.
*   **The Glass & Gradient Rule:** For floating modals or navigation overlays, use a background of `surface_variant` at 80% opacity with a `backdrop-blur` of 12px.

### Signature Accents
*   **Primary Action:** `primary` (#0058be) transitioning to `primary_container` (#2170e4) in a 135-degree linear gradient. This adds a "soul" to buttons that flat hex codes lack.
*   **Status Indicators:** Use `tertiary` (#006947) for "Online" and `error` (#ba1a1a) for "Offline." These should be paired with their "On" counterparts for high-contrast legibility.

---

## 3. Typography
We utilize **Inter** for its mathematical precision and neutral "voice."

*   **Display (lg/md/sm):** Used for high-level network stats (e.g., Total Throughput). Tighten letter spacing by -0.02em for a premium, "Swiss" feel.
*   **Headline & Title:** Use for page headers. These should be `on_surface` to command authority.
*   **Body (lg/md/sm):** All functional data and descriptions. Body-md (0.875rem) is our workhorse for network logs.
*   **Label (md/sm):** Reserved for metadata and technical specs. These should use `on_surface_variant` to recede in the visual hierarchy.

---

## 4. Elevation & Depth
Depth is a functional tool, not a decoration. We use **Tonal Layering** to convey importance.

*   **The Layering Principle:** To lift a card, do not reach for a shadow first. Move from `surface` to `surface-container-lowest`. The shift from off-white to pure white creates a "natural lift."
*   **Ambient Shadows:** For elements that truly float (e.g., dropdowns), use a multi-layered shadow:
    *   `box-shadow: 0 4px 6px -1px rgba(0, 0, 0, 0.04), 0 10px 15px -3px rgba(0, 0, 0, 0.08);`
    *   The shadow color must be a tinted version of `on_surface`, never pure black.
*   **The "Ghost Border" Fallback:** If accessibility requires a border (e.g., high-contrast mode), use `outline_variant` (#c2c6d6) at 20% opacity.

---

## 5. Components

### Navigation Sidebar
*   **Background:** `inverse_surface` (#2d3133).
*   **Active State:** Avoid a simple color change. Use a "pill" shape (`rounded-full`) in `primary_fixed` with `on_primary_fixed` text.
*   **Aesthetic:** Icon-based with `label-sm` text. Icons should be 2px weight for a lightweight look.

### Buttons
*   **Primary:** Gradient of `primary` to `primary_container`. `rounded-DEFAULT` (8px). 
*   **Secondary:** `surface_container_high` background with `on_surface` text. No border.
*   **Tertiary (Ghost):** No background, `primary` text. Use for low-emphasis actions like "Cancel."

### Data Tables (The Network Log)
*   **Rule:** Forbid divider lines. 
*   **Alternative:** Use `spacing-4` (1rem) vertical padding. Highlight every other row with `surface_container_low` for readability.
*   **Headers:** Use `label-md` in uppercase with 0.05em tracking for an editorial "column" feel.

### Input Fields
*   **State:** Unfocused inputs use `surface_container_highest`. Focused inputs transition to `surface_container_lowest` with a 2px "Ghost Border" of `primary`.
*   **Corners:** Strict `8px` (rounded-DEFAULT) to maintain the modern architectural vibe.

### Status Chips
*   **Online:** `tertiary_container` background with `on_tertiary_fixed_variant` text.
*   **Offline:** `error_container` background with `on_error_container` text.

---

## 6. Do's and Don'ts

### Do
*   **Do** use `spacing-12` (3rem) or `spacing-16` (4rem) between major dashboard modules. Breathing room is a sign of a premium experience.
*   **Do** use `backdrop-blur` on the top navigation bar to let content scroll elegantly beneath it.
*   **Do** use `surface-tint` for subtle highlights on active toggles.

### Don't
*   **Don't** use pure black (#000000) for text. Use `on_surface` (#191c1e) to maintain a sophisticated softness.
*   **Don't** use 100% opaque borders. They create "visual friction" and make the dashboard feel heavy.
*   **Don't** mix roundedness. If the system is `8px`, do not use `4px` or `12px` for secondary elements; stick to the Scale.