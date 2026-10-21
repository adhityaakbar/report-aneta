---
name: Notion Style Reference
url: https://notion.so/
colors:
  paper-warmth: '#f6f5f4'
  pure-white: '#ffffff'
  notion-blue: '#0075de'
  notion-blue-hover: '#0062bd'
  notion-blue-ghost-bg: '#e6f3fe'
  hairline-border: 'rgba(0, 0, 0, 0.08)'
  hairline-border-dark: 'rgba(255, 255, 255, 0.08)'
  text-black-100: '#000000'
  text-muted-60: '#757575'
  text-warm-graphite: '#615d59'
  marigold: '#ffb110'
  coral: '#f64932'
  signal-blue: '#097fe8'
  midnight-ink: '#02093a'
  canvas-dark: '#141414'
  card-dark: '#1e1e1e'
typography:
  display:
    family: 'Inter, -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif'
    tracking: '-0.048em'
    weight: 700
  editorial:
    family: '"Source Serif Pro", "Lyon Text", Georgia, serif'
    size: 18px
    line-height: 1.5
  body:
    family: 'Inter, -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif'
    size: 14px
    line-height: 1.6
radius:
  pill: '9999px'
  card: '12px'
  btn: '8px'
  input: '8px'
components:
  card:
    bg: '#ffffff'
    border: '1px solid rgba(0, 0, 0, 0.08)'
    radius: '12px'
    shadow: 'none'
  button-primary:
    bg: '#0075de'
    text: '#ffffff'
    radius: '8px'
    weight: 500
  pill-tag:
    radius: '9999px'
    padding: '4px 12px'
---

# Notion Design System: Aneta Reports Intelligence Desk

Tactile analog warmth ("warm paper notebook under afternoon sun") coupled with high-signal executive analytics.

## 1. Canvas & Core Surfaces
- **Canvas / Page Background:** `#f6f5f4` (`--color-paper-warmth`). Warm off-white tactile paper analog feel.
- **Card Surface:** `#ffffff` (`--color-pure-white`) with 1px hairline border `rgba(0, 0, 0, 0.08)`, 12px border radius (`rounded-xl`), zero drop-shadow.
- **Dark Mode Surface:** `#141414` canvas, `#1e1e1e` card surface, 1px border `rgba(255, 255, 255, 0.08)`.

## 2. Color Palette & Chromatic Commitment
- **Primary CTA / Action:** `#0075de` (`--color-notion-blue`) — 8px radius, white text 500 weight. Single chromatic commitment.
- **Secondary Ghost Action:** `#e6f3fe` background with `#0075de` text.
- **Text & Hierarchy:**
  - 100% / 95%: `#000000` (Headings & Primary labels)
  - 60%: `#757575` (Secondary nav, helper text, muted labels)
  - Warm Graphite: `#615d59` (Body copy harmonizing with paper canvas)
- **Accent Panels & Sticky Cards:**
  - Marigold: `#ffb110` (Agent / highlight cards)
  - Coral: `#f64932` (Alerts & critical deltas)
  - Signal Blue: `#097fe8` (Metrics & KPI indicators)
  - Midnight Ink: `#02093a` (Dark contrast panels on light canvas)

## 3. Typography Rules
- **Display Headings:** Inter / NotionInter with negative tracking (`-0.048em` display, `-0.028em` subheadings).
- **Editorial Accent:** Source Serif Pro / Lyon Text for editorial summaries and section lead-ins.
- **Pills & Badges:** `rounded-full` (9999px) with tight padding for tags and status indicators.

## 5. Thematic Infographics & UI/UX Pro Max Standard
- **Thematic Visual Adaptation:** Each report dynamically adapts visual motifs, badges, and inline SVG illustrations to its subject (e.g., peatland cross-sections, canal block diagrams, water table gauges for environmental topics; network topologies, token throughput sparklines for tech topics).
- **Infographic Dashboard Structure:**
  1. **Visual Hero:** Editorial lead with high-impact key takeaway and quick status meter.
  2. **Bento KPI Matrix:** Visual metric cards with bold numbers, percentage deltas, and progress bars/sparklines.
  3. **Visual Concept Explainer:** An interactive or inline SVG schematic (process flow, 3R framework, before/after comparison).
  4. **Comparison Matrix:** Clean table with inline contextual icons and color-coded status badges.
  5. **Structured Action Items:** 3-column recommendation cards with priority badges.
- **Micro-Interactions:** Smooth hover transitions (`150ms ease`), pill filter buttons, and instant search feedback without layout shift.
