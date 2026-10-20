#!/usr/bin/env python3
"""
Aneta Reports Static Site Generator
Builds a Google News styled portal from markdown and HTML reports in report-summaries/
"""

import os
import re
import sys
import json
import shutil
from pathlib import Path
from datetime import datetime

BASE_DIR = Path(__file__).resolve().parent
REPORTS_DIR = BASE_DIR / "report-summaries"
OUTPUT_INDEX = BASE_DIR / "index.html"

def parse_frontmatter(content):
    meta = {}
    body = content
    if content.startswith("---"):
        parts = content.split("---", 2)
        if len(parts) >= 3:
            fm = parts[1]
            body = parts[2].strip()
            current_key = None
            for line in fm.splitlines():
                line_str = line.strip()
                if not line_str or line_str.startswith("#"):
                    continue
                if ":" in line_str and not line_str.startswith("-"):
                    k, v = line_str.split(":", 1)
                    current_key = k.strip()
                    val = v.strip()
                    if val.startswith("[") and val.endswith("]"):
                        meta[current_key] = [x.strip().strip("'\"") for x in val[1:-1].split(",") if x.strip()]
                    elif val:
                        meta[current_key] = val.strip("'\"")
                    else:
                        meta[current_key] = []
                elif line_str.startswith("-") and current_key:
                    item = line_str[1:].strip().strip("'\"")
                    if isinstance(meta.get(current_key), list):
                        meta[current_key].append(item)
                    else:
                        meta[current_key] = [item]
    return meta, body

def extract_html_metadata(html_content, fallback_slug):
    title = fallback_slug.replace("-", " ").title()
    title_match = re.search(r"<title>(.*?)</title>", html_content, re.IGNORECASE | re.DOTALL)
    if title_match:
        title = title_match.group(1).strip()
    return {
        "title": title,
        "slug": fallback_slug,
        "summary": "Detailed research and benchmark analysis report.",
        "category": "AI Models",
        "author": "Aneta Research",
        "source": "Aneta Reports",
        "date": datetime.now().strftime("%Y-%m-%d"),
        "tags": ["Report"]
    }

def collect_reports():
    reports = {}
    
    if not REPORTS_DIR.exists():
        REPORTS_DIR.mkdir(parents=True, exist_ok=True)
        return []

    # First discover .md files
    for md_file in sorted(REPORTS_DIR.glob("*.md")):
        slug = md_file.stem
        try:
            content = md_file.read_text(encoding="utf-8")
            meta, body = parse_frontmatter(content)
            report_data = {
                "slug": meta.get("slug", slug),
                "title": meta.get("title", slug.replace("-", " ").title()),
                "date": str(meta.get("date", datetime.now().strftime("%Y-%m-%d"))),
                "category": meta.get("category", "General"),
                "author": meta.get("author", "Dora Research Engine"),
                "source": meta.get("source", "Aneta Reports"),
                "summary": meta.get("summary", ""),
                "tags": meta.get("tags", []),
                "has_html": False,
                "html_path": None,
                "body": body
            }
            reports[slug] = report_data
        except Exception as e:
            print(f"Error reading {md_file}: {e}", file=sys.stderr)

    # Discover .html files
    for html_file in sorted(REPORTS_DIR.glob("*.html")):
        slug = html_file.stem
        html_rel = f"report-summaries/{html_file.name}"
        if slug in reports:
            reports[slug]["has_html"] = True
            reports[slug]["html_path"] = html_rel
        else:
            try:
                content = html_file.read_text(encoding="utf-8")
                extracted = extract_html_metadata(content, slug)
                extracted["has_html"] = True
                extracted["html_path"] = html_rel
                extracted["body"] = ""
                reports[slug] = extracted
            except Exception as e:
                print(f"Error reading {html_file}: {e}", file=sys.stderr)

    report_list = list(reports.values())
    # Sort by date descending
    report_list.sort(key=lambda r: r.get("date", ""), reverse=True)
    return report_list

def generate_routes(reports):
    """
    Ensure direct routing on Nginx:
    /<slug> (via directory <slug>/index.html)
    /<slug>.html
    """
    for r in reports:
        slug = r["slug"]
        src_html = REPORTS_DIR / f"{slug}.html"
        if src_html.exists():
            # Create <slug>.html in root
            root_html = BASE_DIR / f"{slug}.html"
            shutil.copy2(src_html, root_html)
            
            # Create <slug>/index.html
            slug_dir = BASE_DIR / slug
            slug_dir.mkdir(exist_ok=True)
            shutil.copy2(src_html, slug_dir / "index.html")
            print(f"[route] Created direct route for /{slug} and /{slug}.html")

def build_index_html(reports):
    top_story = reports[0] if reports else None
    other_stories = reports[1:] if len(reports) > 1 else []
    
    categories = ["Semua", "AI Models", "Telkomsel", "Cloud & Infra", "FinTech"]
    
    # Pre-render stories for JS data
    json_reports = json.dumps([{
        "slug": r["slug"],
        "title": r["title"],
        "date": r["date"],
        "category": r["category"],
        "author": r["author"],
        "source": r["source"],
        "summary": r["summary"],
        "tags": r["tags"],
        "url": f"/{r['slug']}" if r["has_html"] else f"report-summaries/{r['slug']}.html"
    } for r in reports])

    html = f"""<!DOCTYPE html>
<html lang="id" class="light">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>Aneta Reports — Intelligence, Benchmarks & Analytics</title>
  <meta name="description" content="Portal laporan analitik komparasi teknologi, pricing AI model, infrastruktur cloud, dan telekomunikasi.">
  <link rel="preconnect" href="https://fonts.googleapis.com">
  <link rel="preconnect" href="https://fonts.gstatic.com" crossorigin>
  <link href="https://fonts.googleapis.com/css2?family=Plus+Jakarta+Sans:wght@400;500;600;700;800&family=Google+Sans:wght@400;500;700&display=swap" rel="stylesheet">
  <script src="https://cdn.tailwindcss.com"></script>
  <script>
    tailwind.config = {{
      darkMode: 'class',
      theme: {{
        extend: {{
          fontFamily: {{
            sans: ['"Google Sans"', '"Plus Jakarta Sans"', 'system-ui', 'sans-serif'],
          }},
          colors: {{
            gblue: {{
              50: '#E8F0FE',
              100: '#D2E3FC',
              500: '#1A73E8',
              600: '#1557B0',
              700: '#174EA6',
            }},
            gred: '#EA4335',
            gyellow: '#FBBC05',
            ggreen: '#34A853',
            darkbg: '#0F172A',
            darkcard: '#1E293B',
            darkborder: '#334155'
          }}
        }}
      }}
    }}
  </script>
  <style>
    body {{
      font-family: 'Plus Jakarta Sans', -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif;
      transition: background-color 0.2s ease, color 0.2s ease;
    }}
    .headline-font {{
      font-family: 'Google Sans', 'Plus Jakarta Sans', sans-serif;
      letter-spacing: -0.015em;
    }}
    .chip-active {{
      background-color: #E8F0FE !important;
      color: #1A73E8 !important;
      border-color: #1A73E8 !important;
    }}
    .dark .chip-active {{
      background-color: rgba(26, 115, 232, 0.2) !important;
      color: #8AB4F8 !important;
      border-color: #1A73E8 !important;
    }}
    .line-clamp-3 {{
      display: -webkit-box;
      -webkit-line-clamp: 3;
      -webkit-box-orient: vertical;
      overflow: hidden;
    }}
  </style>
</head>
<body class="bg-gray-50 text-gray-900 dark:bg-slate-950 dark:text-gray-100 min-h-screen flex flex-col">

  <!-- TOP HEADER (Google News Style) -->
  <header class="sticky top-0 z-50 bg-white/95 dark:bg-slate-900/95 backdrop-blur-md border-b border-gray-200 dark:border-slate-800">
    <div class="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8">
      <div class="flex items-center justify-between h-16 gap-4">
        
        <!-- Logo & Branding -->
        <div class="flex items-center gap-3 shrink-0">
          <a href="/" class="flex items-center gap-2.5 group">
            <div class="w-9 h-9 rounded-xl bg-gradient-to-tr from-blue-600 via-indigo-600 to-sky-500 flex items-center justify-center text-white font-black text-xl shadow-sm group-hover:scale-105 transition-transform">
              A
            </div>
            <div class="flex flex-col">
              <span class="text-xl font-bold tracking-tight headline-font flex items-center">
                <span class="text-blue-600 dark:text-blue-400">Aneta</span>
                <span class="text-gray-800 dark:text-gray-100 ml-1 font-semibold">Reports</span>
              </span>
              <span class="text-[10px] uppercase font-bold tracking-wider text-gray-400 dark:text-gray-500 -mt-1">
                Intelligence Desk
              </span>
            </div>
          </a>
        </div>

        <!-- Google-style Search Bar -->
        <div class="flex-1 max-w-2xl mx-2 sm:mx-6">
          <div class="relative">
            <div class="absolute inset-y-0 left-0 pl-3.5 flex items-center pointer-events-none text-gray-400 dark:text-gray-500">
              <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z"/>
              </svg>
            </div>
            <input 
              type="text" 
              id="searchInput"
              placeholder="Cari laporan, model AI, benchmark, topik..." 
              class="w-full pl-10 pr-4 py-2 text-sm bg-gray-100 hover:bg-gray-200/70 focus:bg-white dark:bg-slate-800 dark:hover:bg-slate-700/70 dark:focus:bg-slate-900 border border-transparent focus:border-blue-500 rounded-full transition-all duration-150 outline-none placeholder-gray-500 dark:placeholder-gray-400 text-gray-900 dark:text-gray-100 shadow-inner"
            />
          </div>
        </div>

        <!-- Right Side: Date Widget, Status, Theme Toggle -->
        <div class="flex items-center gap-3 shrink-0">
          <!-- Date / Status Widget -->
          <div class="hidden md:flex flex-col text-right">
            <div class="text-xs font-semibold text-gray-700 dark:text-gray-300" id="currentDateStr">
              Rabu, 21 Okt 2026
            </div>
            <div class="flex items-center justify-end gap-1.5 text-[11px] text-emerald-600 dark:text-emerald-400 font-medium">
              <span class="relative flex h-2 w-2">
                <span class="animate-ping absolute inline-flex h-full w-full rounded-full bg-emerald-400 opacity-75"></span>
                <span class="relative inline-flex rounded-full h-2 w-2 bg-emerald-500"></span>
              </span>
              <span>Live Updates</span>
            </div>
          </div>

          <div class="h-6 w-[1px] bg-gray-200 dark:bg-slate-800 hidden sm:block"></div>

          <!-- Theme Toggle Button -->
          <button 
            id="themeToggleBtn"
            title="Ganti Tema"
            class="p-2 rounded-full text-gray-500 hover:text-gray-700 dark:text-gray-400 dark:hover:text-gray-200 hover:bg-gray-100 dark:hover:bg-slate-800 transition-colors">
            <!-- Sun Icon (for dark mode) -->
            <svg id="sunIcon" class="w-5 h-5 hidden" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 3v1m0 16v1m9-9h-1M4 12H3m15.364 6.364l-.707-.707M6.343 6.343l-.707-.707m12.728 0l-.707.707M6.343 17.657l-.707.707M16 12a4 4 0 11-8 0 4 4 0 018 0z"/>
            </svg>
            <!-- Moon Icon (for light mode) -->
            <svg id="moonIcon" class="w-5 h-5 hidden" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M20.354 15.354A9 9 0 018.646 3.646 9.003 9.003 0 0012 21a9.003 9.003 0 008.354-5.646z"/>
            </svg>
          </button>
        </div>

      </div>
    </div>

    <!-- TOPIC FILTER CHIPS (Google News Horizontal Scroll) -->
    <div class="border-t border-gray-100 dark:border-slate-800/80 bg-white dark:bg-slate-900">
      <div class="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8">
        <div class="flex items-center gap-2 py-2.5 overflow-x-auto no-scrollbar scroll-smooth" id="topicChipsContainer">
          {''.join([f'''<button 
            onclick="filterCategory('{cat}')" 
            data-cat="{cat}"
            class="category-chip px-3.5 py-1.5 rounded-full text-xs font-medium border border-gray-200 dark:border-slate-700 hover:bg-gray-100 dark:hover:bg-slate-800 text-gray-700 dark:text-gray-300 transition-all shrink-0 {'chip-active font-semibold' if cat == 'Semua' else ''}">
            {cat}
          </button>''' for cat in categories])}
        </div>
      </div>
    </div>
  </header>

  <!-- MAIN CONTAINER (2-Column Google News Layout) -->
  <main class="flex-1 max-w-7xl w-full mx-auto px-4 sm:px-6 lg:px-8 py-6">
    <div class="grid grid-cols-1 lg:grid-cols-12 gap-6">

      <!-- LEFT COLUMN: Top Story & Articles (8 cols) -->
      <section class="lg:col-span-8 flex flex-col gap-6" id="storiesContainer">

        <!-- TOP STORY / FEATURED REPORT -->
        {f'''
        <article class="report-card bg-white dark:bg-slate-900 rounded-2xl border border-gray-200/90 dark:border-slate-800 p-6 sm:p-7 shadow-sm hover:shadow-md transition-all duration-200 flex flex-col justify-between"
          data-category="{top_story['category']}" 
          data-tags="{','.join(top_story['tags'])}"
          data-title="{top_story['title']}">
          <div>
            <!-- Top meta: Source badge & Date -->
            <div class="flex flex-wrap items-center justify-between gap-2 mb-3.5">
              <div class="flex items-center gap-2">
                <span class="inline-flex items-center gap-1.5 px-2.5 py-1 rounded-full text-xs font-semibold bg-red-50 text-red-600 dark:bg-red-950/60 dark:text-red-400 border border-red-200 dark:border-red-900/50">
                  <span class="w-1.5 h-1.5 rounded-full bg-red-600 dark:bg-red-400"></span>
                  {top_story['source'].upper()}
                </span>
                <span class="text-xs text-gray-400 dark:text-gray-500">•</span>
                <span class="text-xs font-medium text-gray-500 dark:text-gray-400">{top_story['category']}</span>
              </div>
              <div class="text-xs text-gray-400 dark:text-gray-500 font-mono">
                {top_story['date']}
              </div>
            </div>

            <!-- Big Headline -->
            <h1 class="headline-font text-2xl sm:text-3xl font-bold text-gray-900 dark:text-white leading-tight mb-3">
              <a href="/{top_story['slug']}" class="hover:text-blue-600 dark:hover:text-blue-400 transition-colors">
                {top_story['title']}
              </a>
            </h1>

            <!-- Executive Summary -->
            <p class="text-gray-600 dark:text-gray-300 text-sm sm:text-base leading-relaxed mb-5">
              {top_story['summary']}
            </p>

            <!-- Tags -->
            <div class="flex flex-wrap gap-1.5 mb-6">
              {''.join([f'<span class="px-2.5 py-0.5 rounded-md text-xs font-medium bg-gray-100 text-gray-700 dark:bg-slate-800 dark:text-gray-300">#{t}</span>' for t in top_story['tags']])}
            </div>
          </div>

          <!-- Action Footer -->
          <div class="pt-4 border-t border-gray-100 dark:border-slate-800/80 flex items-center justify-between">
            <div class="flex items-center gap-2 text-xs text-gray-500 dark:text-gray-400">
              <svg class="w-4 h-4 text-gray-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M16 7a4 4 0 11-8 0 4 4 0 018 0zM12 14a7 7 0 00-7 7h14a7 7 0 00-7-7z"/>
              </svg>
              <span>{top_story['author']}</span>
            </div>
            <a href="/{top_story['slug']}" 
               class="inline-flex items-center gap-1.5 text-sm font-semibold text-blue-600 dark:text-blue-400 hover:text-blue-700 dark:hover:text-blue-300 group">
              <span>Buka Laporan Lengkap</span>
              <svg class="w-4 h-4 group-hover:translate-x-1 transition-transform" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M14 5l7 7m0 0l-7 7m7-7H3"/>
              </svg>
            </a>
          </div>
        </article>
        ''' if top_story else '<p class="text-gray-500">Belum ada laporan tersedia.</p>'}

        <!-- Additional Stories -->
        {''.join([f'''
        <article class="report-card bg-white dark:bg-slate-900 rounded-xl border border-gray-200 dark:border-slate-800 p-5 shadow-sm hover:shadow-md transition-all flex flex-col justify-between"
          data-category="{st['category']}" 
          data-tags="{','.join(st['tags'])}"
          data-title="{st['title']}">
          <div>
            <div class="flex items-center justify-between gap-2 mb-2 text-xs">
              <span class="font-semibold text-blue-600 dark:text-blue-400">{st['source']}</span>
              <span class="text-gray-400">{st['date']}</span>
            </div>
            <h2 class="text-lg font-bold text-gray-900 dark:text-white mb-2 leading-snug">
              <a href="/{st['slug']}" class="hover:text-blue-600 dark:hover:text-blue-400">{st['title']}</a>
            </h2>
            <p class="text-gray-600 dark:text-gray-300 text-sm mb-3 line-clamp-3">{st['summary']}</p>
          </div>
          <div class="pt-3 border-t border-gray-100 dark:border-slate-800 flex items-center justify-between text-xs">
            <span class="text-gray-500">{st['author']}</span>
            <a href="/{st['slug']}" class="font-medium text-blue-600 dark:text-blue-400 hover:underline">Baca &rarr;</a>
          </div>
        </article>
        ''' for st in other_stories])}

        <div id="noResults" class="hidden text-center py-12 bg-white dark:bg-slate-900 rounded-2xl border border-dashed border-gray-300 dark:border-slate-800">
          <svg class="w-12 h-12 mx-auto text-gray-400 dark:text-gray-600 mb-3" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.5" d="M9.172 16.172a4 4 0 015.656 0M9 10h.01M15 10h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z"/>
          </svg>
          <p class="text-gray-600 dark:text-gray-400 font-medium">Tidak ada laporan yang sesuai dengan pencarian atau filter.</p>
          <button onclick="resetFilters()" class="mt-3 text-xs font-semibold text-blue-600 dark:text-blue-400 hover:underline">Reset filter</button>
        </div>

      </section>

      <!-- RIGHT COLUMN: Side Rail (4 cols) -->
      <aside class="lg:col-span-4 flex flex-col gap-6">

        <!-- CARD 1: Quick Stats -->
        <div class="bg-white dark:bg-slate-900 rounded-2xl border border-gray-200 dark:border-slate-800 p-5 shadow-sm">
          <div class="flex items-center gap-2 mb-4 pb-3 border-b border-gray-100 dark:border-slate-800">
            <div class="w-2.5 h-2.5 rounded-full bg-blue-500"></div>
            <h3 class="text-xs uppercase font-bold tracking-wider text-gray-500 dark:text-gray-400">
              Executive Quick Stats
            </h3>
          </div>
          <div class="space-y-4">
            <div class="p-3.5 rounded-xl bg-gray-50 dark:bg-slate-800/60 border border-gray-100 dark:border-slate-700/50">
              <div class="text-2xl font-black text-blue-600 dark:text-blue-400">35 Model</div>
              <div class="text-xs text-gray-600 dark:text-gray-300 mt-0.5">Model AI aktif terkomparasi di OCTA vs Provider Global</div>
            </div>
            <div class="p-3.5 rounded-xl bg-gray-50 dark:bg-slate-800/60 border border-gray-100 dark:border-slate-700/50">
              <div class="text-2xl font-black text-emerald-600 dark:text-emerald-400">11.9% - 20%</div>
              <div class="text-xs text-gray-600 dark:text-gray-300 mt-0.5">Rata-rata efisiensi tarif Rupiah (bebas kurs valas & fee bank)</div>
            </div>
            <div class="p-3.5 rounded-xl bg-gray-50 dark:bg-slate-800/60 border border-gray-100 dark:border-slate-700/50">
              <div class="text-2xl font-black text-purple-600 dark:text-purple-400">1 Kredit = Rp 1</div>
              <div class="text-xs text-gray-600 dark:text-gray-300 mt-0.5">Patokan fix rate Telkomsel OCTA tanpa fluktuasi USD</div>
            </div>
          </div>
        </div>

        <!-- CARD 2: Categories Breakdown -->
        <div class="bg-white dark:bg-slate-900 rounded-2xl border border-gray-200 dark:border-slate-800 p-5 shadow-sm">
          <div class="flex items-center gap-2 mb-4 pb-3 border-b border-gray-100 dark:border-slate-800">
            <div class="w-2.5 h-2.5 rounded-full bg-emerald-500"></div>
            <h3 class="text-xs uppercase font-bold tracking-wider text-gray-500 dark:text-gray-400">
              Topik & Kategori
            </h3>
          </div>
          <ul class="space-y-2 text-sm">
            <li class="flex items-center justify-between p-2 rounded-lg hover:bg-gray-50 dark:hover:bg-slate-800 transition-colors cursor-pointer" onclick="filterCategory('AI Models')">
              <span class="text-gray-700 dark:text-gray-300 font-medium">AI Models & LLM</span>
              <span class="text-xs font-semibold px-2 py-0.5 rounded-full bg-blue-50 text-blue-600 dark:bg-blue-900/30 dark:text-blue-400">1 Laporan</span>
            </li>
            <li class="flex items-center justify-between p-2 rounded-lg hover:bg-gray-50 dark:hover:bg-slate-800 transition-colors cursor-pointer" onclick="filterCategory('Telkomsel')">
              <span class="text-gray-700 dark:text-gray-300 font-medium">Telkomsel Ecosystem</span>
              <span class="text-xs font-semibold px-2 py-0.5 rounded-full bg-gray-100 text-gray-600 dark:bg-slate-800 dark:text-gray-400">1 Laporan</span>
            </li>
            <li class="flex items-center justify-between p-2 rounded-lg hover:bg-gray-50 dark:hover:bg-slate-800 transition-colors cursor-pointer" onclick="filterCategory('Cloud & Infra')">
              <span class="text-gray-700 dark:text-gray-300 font-medium">Cloud & Infrastructure</span>
              <span class="text-xs font-semibold px-2 py-0.5 rounded-full bg-gray-100 text-gray-600 dark:bg-slate-800 dark:text-gray-400">Segera</span>
            </li>
            <li class="flex items-center justify-between p-2 rounded-lg hover:bg-gray-50 dark:hover:bg-slate-800 transition-colors cursor-pointer" onclick="filterCategory('FinTech')">
              <span class="text-gray-700 dark:text-gray-300 font-medium">FinTech & Payment</span>
              <span class="text-xs font-semibold px-2 py-0.5 rounded-full bg-gray-100 text-gray-600 dark:bg-slate-800 dark:text-gray-400">Segera</span>
            </li>
          </ul>
        </div>

        <!-- CARD 3: System & Platform Status -->
        <div class="bg-white dark:bg-slate-900 rounded-2xl border border-gray-200 dark:border-slate-800 p-5 shadow-sm text-xs">
          <div class="flex items-center gap-2 mb-3 pb-2 border-b border-gray-100 dark:border-slate-800">
            <div class="w-2.5 h-2.5 rounded-full bg-indigo-500"></div>
            <h3 class="text-xs uppercase font-bold tracking-wider text-gray-500 dark:text-gray-400">
              Infrastruktur & Status
            </h3>
          </div>
          <div class="space-y-2.5 text-gray-600 dark:text-gray-300">
            <div class="flex justify-between items-center">
              <span class="text-gray-500 dark:text-gray-400">Domain</span>
              <span class="font-mono font-medium text-gray-800 dark:text-gray-200">report.aneta.my.id</span>
            </div>
            <div class="flex justify-between items-center">
              <span class="text-gray-500 dark:text-gray-400">Host Engine</span>
              <span class="font-mono font-medium text-gray-800 dark:text-gray-200">LXC g4-apps (192.168.100.192)</span>
            </div>
            <div class="flex justify-between items-center">
              <span class="text-gray-500 dark:text-gray-400">Web Server</span>
              <span class="font-mono font-medium text-gray-800 dark:text-gray-200">Nginx /var/www/report</span>
            </div>
            <div class="flex justify-between items-center">
              <span class="text-gray-500 dark:text-gray-400">Pipeline</span>
              <span class="inline-flex items-center gap-1 text-emerald-600 dark:text-emerald-400 font-semibold">
                <span class="w-1.5 h-1.5 rounded-full bg-emerald-500"></span>
                GitHub Actions (prod)
              </span>
            </div>
          </div>
        </div>

      </aside>

    </div>
  </main>

  <!-- FOOTER -->
  <footer class="mt-auto border-t border-gray-200 dark:border-slate-800 bg-white dark:bg-slate-900 py-6 text-xs text-gray-500 dark:text-gray-400">
    <div class="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 flex flex-col sm:flex-row items-center justify-between gap-4">
      <div class="flex items-center gap-2">
        <span class="font-bold text-gray-700 dark:text-gray-300">Aneta Reports</span>
        <span>•</span>
        <span>Research Engine & Benchmark Portal</span>
      </div>
      <div>
        <span>Diperbarui otomatis via CI/CD • 2026 Dora Research Desk</span>
      </div>
    </div>
  </footer>

  <!-- CLIENT-SIDE SCRIPT FOR FILTERING, SEARCH & DARK MODE -->
  <script>
    // Theme Switcher Logic
    const themeToggleBtn = document.getElementById('themeToggleBtn');
    const sunIcon = document.getElementById('sunIcon');
    const moonIcon = document.getElementById('moonIcon');

    function applyTheme(isDark) {{
      if (isDark) {{
        document.documentElement.classList.add('dark');
        document.documentElement.classList.remove('light');
        sunIcon.classList.remove('hidden');
        moonIcon.classList.add('hidden');
      }} else {{
        document.documentElement.classList.remove('dark');
        document.documentElement.classList.add('light');
        sunIcon.classList.add('hidden');
        moonIcon.classList.remove('hidden');
      }}
    }}

    // Initial theme check
    const savedTheme = localStorage.getItem('theme');
    const prefersDark = window.matchMedia('(prefers-color-scheme: dark)').matches;
    applyTheme(savedTheme === 'dark' || (!savedTheme && prefersDark));

    themeToggleBtn.addEventListener('click', () => {{
      const isCurrentlyDark = document.documentElement.classList.contains('dark');
      const newTheme = !isCurrentlyDark;
      applyTheme(newTheme);
      localStorage.setItem('theme', newTheme ? 'dark' : 'light');
    }});

    // Search and Category Filter Logic
    let currentCategory = 'Semua';
    const searchInput = document.getElementById('searchInput');
    const cards = document.querySelectorAll('.report-card');
    const noResults = document.getElementById('noResults');

    function filterReports() {{
      const query = (searchInput.value || '').toLowerCase().trim();
      let visibleCount = 0;

      cards.forEach(card => {{
        const cat = card.getAttribute('data-category') || '';
        const tags = card.getAttribute('data-tags') || '';
        const title = card.getAttribute('data-title') || '';
        const cardText = card.innerText.toLowerCase();

        const matchesCat = (currentCategory === 'Semua') || 
                           (cat.toLowerCase().includes(currentCategory.toLowerCase())) ||
                           (tags.toLowerCase().includes(currentCategory.toLowerCase()));
        const matchesQuery = !query || cardText.includes(query) || title.toLowerCase().includes(query);

        if (matchesCat && matchesQuery) {{
          card.style.display = '';
          visibleCount++;
        }} else {{
          card.style.display = 'none';
        }}
      }});

      if (visibleCount === 0) {{
        noResults.classList.remove('hidden');
      }} else {{
        noResults.classList.add('hidden');
      }}
    }}

    function filterCategory(cat) {{
      currentCategory = cat;
      document.querySelectorAll('.category-chip').forEach(btn => {{
        if (btn.getAttribute('data-cat') === cat) {{
          btn.classList.add('chip-active', 'font-semibold');
        }} else {{
          btn.classList.remove('chip-active', 'font-semibold');
        }}
      }});
      filterReports();
    }}

    function resetFilters() {{
      searchInput.value = '';
      filterCategory('Semua');
    }}

    searchInput.addEventListener('input', filterReports);

    // Format Date Widget
    try {{
      const now = new Date();
      const options = {{ weekday: 'long', year: 'numeric', month: 'short', day: 'numeric' }};
      document.getElementById('currentDateStr').innerText = now.toLocaleDateString('id-ID', options);
    }} catch (e) {{}}
  </script>
</body>
</html>
"""
    return html

def main():
    print("=== Building Aneta Reports Static Site ===")
    reports = collect_reports()
    print(f"Discovered {len(reports)} report(s):")
    for r in reports:
        print(f" - [{r.get('category')}] {r.get('title')} ({r.get('slug')})")

    # Generate routes (<slug>/index.html and <slug>.html)
    generate_routes(reports)

    # Generate index.html
    html_output = build_index_html(reports)
    OUTPUT_INDEX.write_text(html_output, encoding="utf-8")
    print(f"[build] Generated home page at {OUTPUT_INDEX} ({len(html_output)} bytes)")

if __name__ == "__main__":
    main()
