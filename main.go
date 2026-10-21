package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

const (
	SecretKey    = "guardian8"
	RouterURL    = "https://9router.mytsel.id/v1/chat/completions"
	RouterAPIKey = "sk-088ecd7360b12ce3-18ymc8-86634ab0"
	DefaultModel = "9combos-agy"
)

type ResearchRequest struct {
	Topic        string `json:"topic"`
	Mode         string `json:"mode"`          // "new" or "enhance"
	ExistingSlug string `json:"existing_slug"` // slug for enhance
	SecretKey    string `json:"secret_key"`
}

type ResearchResponse struct {
	Success bool        `json:"success"`
	Result  interface{} `json:"result,omitempty"`
	Error   string      `json:"error,omitempty"`
}

type ReportItem struct {
	Slug     string `json:"slug"`
	Title    string `json:"title"`
	Date     string `json:"date"`
	Category string `json:"category"`
	Summary  string `json:"summary"`
	HasHTML  bool   `json:"has_html"`
	URL      string `json:"url"`
}

func findLightpanda() string {
	if p, err := exec.LookPath("lightpanda"); err == nil {
		return p
	}
	home, _ := os.UserHomeDir()
	candidate := filepath.Join(home, ".local", "bin", "lightpanda")
	if info, err := os.Stat(candidate); err == nil && (info.Mode()&0111 != 0) {
		return candidate
	}
	return ""
}

func searchLightpandaMCP(query string) []map[string]string {
	binary := findLightpanda()
	if binary == "" {
		return []map[string]string{{"title": "Lightpanda unavailable", "snippet": "Binary not found in path"}}
	}

	cmd := exec.Command(binary, "mcp")
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil
	}
	if err := cmd.Start(); err != nil {
		return nil
	}
	defer cmd.Process.Kill()

	initReq := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "initialize",
		"params": map[string]interface{}{
			"protocolVersion": "2024-11-05",
			"capabilities":    map[string]interface{}{},
			"clientInfo":      map[string]string{"name": "aneta-researcher-go", "version": "1.0"},
		},
	}
	initBytes, _ := json.Marshal(initReq)
	stdin.Write(append(initBytes, '\n'))

	buf := make([]byte, 8192)
	stdout.Read(buf) // consume init response

	searchReq := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      2,
		"method":  "tools/call",
		"params": map[string]interface{}{
			"name":      "search",
			"arguments": map[string]string{"query": query},
		},
	}
	searchBytes, _ := json.Marshal(searchReq)
	stdin.Write(append(searchBytes, '\n'))

	decoder := json.NewDecoder(stdout)
	var resp map[string]interface{}
	if err := decoder.Decode(&resp); err == nil {
		if res, ok := resp["result"].(map[string]interface{}); ok {
			if content, ok := res["content"].([]interface{}); ok && len(content) > 0 {
				if item, ok := content[0].(map[string]interface{}); ok {
					text, _ := item["text"].(string)
					return parseMarkdownSearchResults(text)
				}
			}
		}
	}
	return nil
}

func parseMarkdownSearchResults(raw string) []map[string]string {
	var results []map[string]string
	lines := strings.Split(raw, "\n")
	var curTitle, curURL, curSnippet string

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "1. ") || strings.HasPrefix(trimmed, "2. ") || strings.HasPrefix(trimmed, "3. ") ||
			strings.HasPrefix(trimmed, "4. ") || strings.HasPrefix(trimmed, "5. ") {
			if curTitle != "" {
				results = append(results, map[string]string{
					"title":   curTitle,
					"url":     curURL,
					"snippet": curSnippet,
				})
			}
			parts := strings.Split(trimmed, " — ")
			curTitle = strings.TrimPrefix(parts[0], trimmed[:3])
			curTitle = strings.ReplaceAll(curTitle, "*", "")
			if len(parts) > 1 {
				curURL = parts[1]
			}
			curSnippet = ""
		} else if trimmed != "" {
			curSnippet += " " + trimmed
		}
	}
	if curTitle != "" {
		results = append(results, map[string]string{
			"title":   curTitle,
			"url":     curURL,
			"snippet": curSnippet,
		})
	}
	return results
}

func call9Router(systemPrompt, userPrompt string) (string, error) {
	reqBody := map[string]interface{}{
		"model": DefaultModel,
		"messages": []map[string]string{
			{"role": "system", "content": systemPrompt},
			{"role": "user", "content": userPrompt},
		},
		"stream":      false,
		"temperature": 0.3,
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequest("POST", RouterURL, bytes.NewBuffer(bodyBytes))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+RouterAPIKey)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 90 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("9router status %d: %s", resp.StatusCode, string(respBytes))
	}

	var parsed struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}

	if err := json.Unmarshal(respBytes, &parsed); err != nil {
		return "", err
	}

	if len(parsed.Choices) == 0 {
		return "", fmt.Errorf("empty choices from 9router")
	}

	return parsed.Choices[0].Message.Content, nil
}

func collectReports() []ReportItem {
	reportsDir := filepath.Join(".", "report-summaries")
	files, err := os.ReadDir(reportsDir)
	if err != nil {
		return nil
	}

	reportsMap := make(map[string]ReportItem)

	for _, f := range files {
		if strings.HasSuffix(f.Name(), ".md") {
			slug := strings.TrimSuffix(f.Name(), ".md")
			data, _ := os.ReadFile(filepath.Join(reportsDir, f.Name()))
			title := slug
			category := "AI Models"
			date := time.Now().Format("2006-01-02")
			summary := ""

			lines := strings.Split(string(data), "\n")
			for _, l := range lines {
				if strings.HasPrefix(l, "title:") {
					title = strings.TrimSpace(strings.TrimPrefix(l, "title:"))
				} else if strings.HasPrefix(l, "category:") {
					category = strings.TrimSpace(strings.TrimPrefix(l, "category:"))
				} else if strings.HasPrefix(l, "date:") {
					date = strings.TrimSpace(strings.TrimPrefix(l, "date:"))
				} else if strings.HasPrefix(l, "summary:") {
					summary = strings.TrimSpace(strings.TrimPrefix(l, "summary:"))
				}
			}

			reportsMap[slug] = ReportItem{
				Slug:     slug,
				Title:    title,
				Date:     date,
				Category: category,
				Summary:  summary,
				HasHTML:  false,
				URL:      "/" + slug,
			}
		}
	}

	for _, f := range files {
		if strings.HasSuffix(f.Name(), ".html") {
			slug := strings.TrimSuffix(f.Name(), ".html")
			item, exists := reportsMap[slug]
			if !exists {
				item = ReportItem{
					Slug:     slug,
					Title:    strings.Title(strings.ReplaceAll(slug, "-", " ")),
					Date:     time.Now().Format("2006-01-02"),
					Category: "AI Models",
					Summary:  "Detailed research benchmark analysis report.",
					URL:      "/" + slug,
				}
			}
			item.HasHTML = true
			reportsMap[slug] = item
		}
	}

	var list []ReportItem
	for _, v := range reportsMap {
		list = append(list, v)
	}
	return list
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	return err
}

func generateRoutes(reports []ReportItem) {
	reportsDir := filepath.Join(".", "report-summaries")
	for _, r := range reports {
		if !r.HasHTML {
			continue
		}
		srcHTML := filepath.Join(reportsDir, r.Slug+".html")
		if _, err := os.Stat(srcHTML); err == nil {
			rootHTML := filepath.Join(".", r.Slug+".html")
			copyFile(srcHTML, rootHTML)

			slugDir := filepath.Join(".", r.Slug)
			os.MkdirAll(slugDir, 0755)
			copyFile(srcHTML, filepath.Join(slugDir, "index.html"))
			log.Printf("[route] Generated direct route for /%s", r.Slug)
		}
	}
}

func buildIndexHTML(reports []ReportItem) {
	jsonReports, _ := json.Marshal(reports)

	var cardsHTML strings.Builder
	for _, r := range reports {
		url := "/" + r.Slug
		cardsHTML.WriteString(fmt.Sprintf(`
        <article class="report-card bg-white rounded-xl border border-black/[0.08] p-6 hover:border-black/20 transition-all flex flex-col justify-between"
                 data-category="%s" 
                 data-title="%s">
          <div>
            <div class="flex items-center justify-between gap-2 mb-3">
              <span class="text-[11px] font-semibold px-2.5 py-0.5 rounded-full bg-[#e6f3fe] text-[#0075de] border border-[#0075de]/20">
                %s
              </span>
              <span class="text-xs font-mono text-[#757575]">%s</span>
            </div>
            <h2 class="text-lg font-bold text-[#111111] mb-2 leading-snug tracking-tight">
              <a href="%s" class="hover:text-[#0075de] transition-colors">%s</a>
            </h2>
            <p class="text-xs text-[#615d59] leading-relaxed mb-4 line-clamp-3">
              %s
            </p>
          </div>
          
          <div class="pt-4 border-t border-black/[0.06] flex items-center justify-between text-xs">
            <span class="text-[#757575] truncate max-w-[140px]">Aneta Research</span>
            <div class="flex items-center gap-2">
              <button onclick="openEnhanceModal('%s', '%s')" 
                      class="px-2.5 py-1 rounded-lg bg-[#f6f5f4] hover:bg-[#e6f3fe] text-[#615d59] hover:text-[#0075de] font-medium transition flex items-center gap-1 border border-black/[0.06]">
                <svg class="w-3.5 h-3.5 text-[#ffb110]" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 4v16m8-8H4"/></svg>
                <span>Enhance</span>
              </button>
              <a href="%s" class="px-3 py-1 rounded-lg bg-[#0075de] hover:bg-[#0060b9] text-white font-medium transition shadow-sm">
                Lihat Laporan
              </a>
            </div>
          </div>
        </article>
		`, r.Category, r.Title, r.Category, r.Date, url, r.Title, r.Summary, r.Slug, r.Title, url))
	}

	html := fmt.Sprintf(`<!DOCTYPE html>
<html lang="id">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>Aneta Research — Intelligence, Benchmarks & Analytics Hub</title>
  <meta name="description" content="Portal dashboard intelligence, teknologi komparasi, dan autonomous AI research engine Aneta.">
  <link rel="preconnect" href="https://fonts.googleapis.com">
  <link rel="preconnect" href="https://fonts.gstatic.com" crossorigin>
  <link href="https://fonts.googleapis.com/css2?family=Inter:wght@400;500;600;700;800&family=Source+Serif+4:ital,wght@0,400;0,600;1,400&family=Fira+Code:wght@400;500;600&display=swap" rel="stylesheet">
  <script src="https://cdn.tailwindcss.com"></script>
  <script>
    tailwind.config = {
      theme: {
        extend: {
          fontFamily: {
            sans: ['"Inter"', 'system-ui', 'sans-serif'],
            serif: ['"Source Serif 4"', 'serif'],
            mono: ['"Fira Code"', 'monospace'],
          },
          colors: {
            notion: {
              blue: '#0075de',
              canvas: '#f6f5f4',
              white: '#ffffff',
              ink: '#000000',
              charcoal: '#111111',
              stone: '#757575',
              graphite: '#615d59',
              slate: '#696969',
              skytint: '#e6f3fe',
              marigold: '#ffb110',
              coral: '#f64932',
              midnight: '#02093a'
            }
          }
        }
      }
    }
  </script>
  <style>
    body {
      font-family: 'Inter', system-ui, -apple-system, sans-serif;
      background-color: #f6f5f4;
      color: #111111;
    }
    .hero-heading {
      letter-spacing: -0.04em;
    }
    .line-clamp-3 {
      display: -webkit-box;
      -webkit-line-clamp: 3;
      -webkit-box-orient: vertical;
      overflow: hidden;
    }
  </style>
</head>
<body class="min-h-screen antialiased flex flex-col bg-[#f6f5f4] text-[#111111]">

  <!-- Top Sticky Header -->
  <header class="border-b border-black/[0.08] bg-[#f6f5f4]/90 backdrop-blur-md sticky top-0 z-40">
    <div class="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 h-16 flex items-center justify-between gap-4">
      <div class="flex items-center gap-3">
        <a href="/" class="flex items-center gap-2.5">
          <div class="w-8 h-8 rounded-lg bg-[#0075de] text-white flex items-center justify-center font-bold text-lg shadow-sm">
            A
          </div>
          <div class="flex items-baseline gap-1.5">
            <span class="font-bold text-lg tracking-tight text-[#111111]">Aneta</span>
            <span class="font-medium text-sm text-[#757575]">Research</span>
          </div>
        </a>
        <span class="hidden sm:inline-flex text-[11px] font-medium px-2 py-0.5 rounded-full bg-[#ffb110]/20 text-[#8f6200] border border-[#ffb110]/30 ml-2">
          Go Engine Active
        </span>
      </div>

      <div class="flex items-center gap-3">
        <a href="/octa-telkomsel-model-comparison" class="hidden md:inline-flex text-xs font-medium text-[#615d59] hover:text-[#0075de] px-3 py-1.5 rounded-lg hover:bg-black/5 transition">
          OCTA Benchmark
        </a>
        <button id="openResearchBtn" class="flex items-center gap-2 px-3.5 py-1.5 rounded-lg bg-[#0075de] hover:bg-[#0060b9] text-white font-medium text-xs shadow-sm transition active:scale-95">
          <svg class="w-3.5 h-3.5 text-white" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13 10V3L4 14h7v7l9-11h-7z"/></svg>
          <span>AI Deep Research</span>
          <span class="text-[10px] px-1 rounded bg-black/20 text-white font-mono">Lock</span>
        </button>
      </div>
    </div>
  </header>

  <!-- Hero Section -->
  <main class="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-8 w-full flex-1">
    
    <div class="mb-8">
      <div class="inline-flex items-center gap-1.5 px-3 py-1 rounded-full bg-[#e6f3fe] text-[#0075de] text-xs font-medium mb-3 border border-[#0075de]/20">
        <span class="w-2 h-2 rounded-full bg-[#0075de] animate-pulse"></span>
        <span>Intelligence Desk & Autonomous Research</span>
      </div>
      <h1 class="text-3xl sm:text-4xl font-extrabold text-[#111111] hero-heading mb-2">
        Where intelligence and technology benchmarks meet.
      </h1>
      <p class="text-sm sm:text-base text-[#615d59] max-w-2xl font-serif">
        Analisis komprehensif teknologi, komparasi harga model AI, dan deep research mandiri berbasis Go backend API.
      </p>
    </div>

    <!-- 4 Notion Accent Cards (Metrics & Engine Specs) -->
    <div class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4 mb-8">
      
      <!-- Card 1: Engine -->
      <div class="bg-white rounded-xl border border-black/[0.08] p-5">
        <div class="text-[11px] font-semibold text-[#757575] uppercase tracking-wider mb-1">Inference Engine</div>
        <div class="text-xl font-bold text-[#111111]">9router LLM</div>
        <div class="text-xs text-[#0075de] mt-1 font-mono font-medium">9combos-agy (Active)</div>
      </div>

      <!-- Card 2: Web Search -->
      <div class="bg-white rounded-xl border border-black/[0.08] p-5">
        <div class="text-[11px] font-semibold text-[#757575] uppercase tracking-wider mb-1">Web Extraction</div>
        <div class="text-xl font-bold text-[#111111]">Lightpanda MCP</div>
        <div class="text-xs text-[#615d59] mt-1">Headless Browser Search</div>
      </div>

      <!-- Card 3: Reports Count -->
      <div class="bg-white rounded-xl border border-black/[0.08] p-5">
        <div class="text-[11px] font-semibold text-[#757575] uppercase tracking-wider mb-1">Published Intel</div>
        <div class="text-xl font-bold text-[#111111]">%d Report(s)</div>
        <div class="text-xs text-[#615d59] mt-1">Verified Benchmarks</div>
      </div>

      <!-- Card 4: Output Standard -->
      <div class="bg-white rounded-xl border border-black/[0.08] p-5">
        <div class="text-[11px] font-semibold text-[#757575] uppercase tracking-wider mb-1">Backend Runtime</div>
        <div class="text-xl font-bold text-[#111111]">Golang 1.25</div>
        <div class="text-xs text-[#615d59] mt-1">High Performance Native</div>
      </div>

    </div>

    <!-- Filter & Search Bar -->
    <div class="bg-white rounded-xl border border-black/[0.08] p-3 mb-6 flex flex-col sm:flex-row items-center justify-between gap-3">
      <div class="flex items-center gap-1.5 overflow-x-auto w-full sm:w-auto" id="categoryFilters">
        <button onclick="filterCategory('Semua')" class="cat-chip px-3 py-1 rounded-lg text-xs font-semibold bg-[#111111] text-white">Semua</button>
        <button onclick="filterCategory('AI Models')" class="cat-chip px-3 py-1 rounded-lg text-xs font-medium text-[#615d59] hover:bg-[#f6f5f4] transition">AI Models</button>
        <button onclick="filterCategory('Telkomsel')" class="cat-chip px-3 py-1 rounded-lg text-xs font-medium text-[#615d59] hover:bg-[#f6f5f4] transition">Telkomsel</button>
        <button onclick="filterCategory('Cloud & Infra')" class="cat-chip px-3 py-1 rounded-lg text-xs font-medium text-[#615d59] hover:bg-[#f6f5f4] transition">Cloud & Infra</button>
      </div>
      <div class="relative w-full sm:w-64">
        <input type="text" id="searchInput" placeholder="Cari laporan..." class="w-full bg-[#f6f5f4] border border-black/[0.08] rounded-lg px-3 py-1.5 text-xs text-[#111111] placeholder-[#757575] focus:outline-none focus:border-[#0075de]">
      </div>
    </div>

    <!-- Reports Grid -->
    <div class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6" id="reportsGrid">
      %s
    </div>

    <div id="noResults" class="hidden text-center py-12 bg-white rounded-xl border border-black/[0.08] mt-4">
      <p class="text-sm text-[#757575]">Tidak ditemukan laporan yang sesuai kriteria pencarian.</p>
    </div>

  </main>

  <!-- PASSWORD MODAL (guardian8) -->
  <div id="authModal" class="fixed inset-0 z-50 bg-black/40 backdrop-blur-sm flex items-center justify-center p-4 hidden">
    <div class="bg-white border border-black/[0.08] rounded-2xl max-w-sm w-full p-6 shadow-2xl relative">
      <div class="w-10 h-10 rounded-xl bg-[#e6f3fe] text-[#0075de] flex items-center justify-center mb-4 font-bold text-lg">
        <svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 15v2m-6 4h12a2 2 0 002-2v-6a2 2 0 00-2-2H6a2 2 0 00-2 2v6a2 2 0 002 2zm10-10V7a4 4 0 00-8 0v4h8z"/></svg>
      </div>
      <h3 class="text-base font-bold text-[#111111] mb-1">Akses AI Research Engine</h3>
      <p class="text-xs text-[#615d59] mb-4">Masukkan secret key autentikasi untuk mengakses konsol Autonomous Deep Research.</p>
      
      <form id="authForm" onsubmit="handleAuthSubmit(event)">
        <div class="mb-4">
          <input type="password" id="secretInput" placeholder="Masukkan secret key..." class="w-full bg-[#f6f5f4] border border-black/[0.08] rounded-lg px-3.5 py-2 text-xs text-[#111111] placeholder-[#757575] focus:outline-none focus:border-[#0075de] font-mono">
          <p id="authError" class="text-[11px] text-[#f64932] mt-1.5 hidden">Secret key salah. Akses ditolak.</p>
        </div>
        <div class="flex items-center justify-end gap-2">
          <button type="button" onclick="closeAuthModal()" class="px-3.5 py-1.5 rounded-lg text-xs text-[#757575] hover:text-[#111111] transition">Batal</button>
          <button type="submit" class="px-4 py-1.5 rounded-lg bg-[#0075de] hover:bg-[#0060b9] text-xs font-semibold text-white transition shadow-sm">Buka Konsol</button>
        </div>
      </form>
    </div>
  </div>

  <!-- AI RESEARCH CHAT DRAWER -->
  <div id="chatDrawer" class="fixed inset-0 z-50 bg-black/40 backdrop-blur-sm flex justify-end hidden">
    <div class="bg-white border-l border-black/[0.08] w-full max-w-xl h-full flex flex-col shadow-2xl">
      
      <!-- Drawer Header -->
      <div class="p-4 border-b border-black/[0.08] flex items-center justify-between bg-[#f6f5f4]">
        <div class="flex items-center gap-2.5">
          <div class="w-8 h-8 rounded-lg bg-[#0075de] text-white flex items-center justify-center font-bold">
            <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13 10V3L4 14h7v7l9-11h-7z"/></svg>
          </div>
          <div>
            <div class="flex items-center gap-2">
              <h3 class="text-sm font-bold text-[#111111]">Aneta Deep Research</h3>
              <span class="text-[10px] px-1.5 py-0.2 rounded bg-[#e6f3fe] text-[#0075de] font-mono font-medium">Golang + 9router</span>
            </div>
            <p class="text-[11px] text-[#757575]">Lightpanda MCP Search + Octa-style Interactive HTML Generator</p>
          </div>
        </div>
        <button onclick="closeChatDrawer()" class="p-1 rounded-lg text-[#757575] hover:text-[#111111] hover:bg-black/5 transition">
          <svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12"/></svg>
        </button>
      </div>

      <!-- Mode Selector -->
      <div class="px-4 py-3 bg-[#f6f5f4] border-b border-black/[0.08] flex flex-wrap items-center gap-2">
        <span class="text-xs font-medium text-[#757575]">Mode:</span>
        <button id="modeNewBtn" onclick="setResearchMode('new')" class="px-3 py-1 rounded-lg text-xs font-semibold bg-[#111111] text-white transition">
          New Research
        </button>
        <button id="modeEnhanceBtn" onclick="setResearchMode('enhance')" class="px-3 py-1 rounded-lg text-xs font-medium text-[#615d59] hover:bg-black/5 transition">
          Enhanced Existing Research
        </button>
        <div id="enhanceSelectContainer" class="hidden w-full mt-2 flex items-center gap-2">
          <label class="text-xs text-[#757575] whitespace-nowrap">Target Report:</label>
          <select id="targetResearchSelect" class="w-full bg-white border border-black/[0.08] rounded-lg px-2.5 py-1 text-xs text-[#111111]">
          </select>
        </div>
      </div>

      <!-- Messages Thread -->
      <div id="chatMessages" class="flex-1 overflow-y-auto p-4 space-y-4 bg-[#f6f5f4]/50">
        <div class="bg-white border border-black/[0.08] rounded-xl p-4 text-xs text-[#615d59] leading-relaxed shadow-sm">
          <div class="font-bold text-[#111111] mb-1 flex items-center gap-1.5">
            <span class="w-2 h-2 rounded-full bg-[#10b981]"></span>
            <span>Agent Siap Melakukan Riset</span>
          </div>
          Ketik topik riset teknologi, benchmark, atau data yang ingin dianalisis. AI akan melakukan live web retrieval melalui Lightpanda MCP, men-sintesis data ala Perplexity Pro, dan menyusun laporan interaktif berstandar OCTA.
        </div>
      </div>

      <!-- Chat Input Form -->
      <div class="p-4 border-t border-black/[0.08] bg-white">
        <form id="chatForm" onsubmit="handleSendResearch(event)" class="space-y-3">
          <div class="relative">
            <textarea id="chatInput" rows="3" placeholder="Contoh: 'Komparasi performa dan efisiensi biaya GPU NVIDIA B200 vs H100 di hyperscaler cloud 2026'..." class="w-full bg-[#f6f5f4] border border-black/[0.08] rounded-xl p-3 text-xs text-[#111111] placeholder-[#757575] focus:outline-none focus:border-[#0075de] resize-none"></textarea>
          </div>
          <div class="flex items-center justify-between">
            <div class="flex items-center gap-2 text-[11px] text-[#757575]">
              <span class="flex items-center gap-1">
                <span class="w-1.5 h-1.5 rounded-full bg-[#0075de]"></span>
                Lightpanda Search
              </span>
              <span>•</span>
              <span>Auto-save HTML</span>
            </div>
            <button type="submit" id="submitResearchBtn" class="px-4 py-2 rounded-lg bg-[#0075de] hover:bg-[#0060b9] text-xs font-semibold text-white shadow-sm flex items-center gap-1.5 transition">
              <span>Mulai Riset</span>
              <svg class="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M14 5l7 7m0 0l-7 7m7-7H3"/></svg>
            </button>
          </div>
        </form>
      </div>

    </div>
  </div>

  <footer class="border-t border-black/[0.08] bg-white py-6 mt-12">
    <div class="max-w-7xl mx-auto px-4 text-center text-xs text-[#757575]">
      Aneta Reports &copy; %d — PT Aneta Cyber System. Powered by Golang Backend, 9router & Lightpanda MCP.
    </div>
  </footer>

  <script>
    let currentAuthKey = sessionStorage.getItem('aneta_research_key') || '';
    let currentMode = 'new';
    const allReports = %s;

    const authModal = document.getElementById('authModal');
    const chatDrawer = document.getElementById('chatDrawer');
    const secretInput = document.getElementById('secretInput');
    const authError = document.getElementById('authError');
    const modeNewBtn = document.getElementById('modeNewBtn');
    const modeEnhanceBtn = document.getElementById('modeEnhanceBtn');
    const enhanceSelectContainer = document.getElementById('enhanceSelectContainer');
    const targetResearchSelect = document.getElementById('targetResearchSelect');
    const chatMessages = document.getElementById('chatMessages');
    const chatInput = document.getElementById('chatInput');
    const searchInput = document.getElementById('searchInput');
    const noResults = document.getElementById('noResults');

    function populateDropdown() {
      targetResearchSelect.innerHTML = allReports.map(r => 
        '<option value="' + r.slug + '">' + r.title + '</option>'
      ).join('');
    }
    populateDropdown();

    document.getElementById('openResearchBtn').addEventListener('click', () => {
      if (currentAuthKey === 'guardian8') {
        openChatDrawer();
      } else {
        openAuthModal();
      }
    });

    function openAuthModal() {
      authModal.classList.remove('hidden');
      secretInput.value = '';
      authError.classList.add('hidden');
      secretInput.focus();
    }

    function closeAuthModal() {
      authModal.classList.add('hidden');
    }

    function handleAuthSubmit(e) {
      e.preventDefault();
      const val = secretInput.value.trim();
      if (val === 'guardian8') {
        currentAuthKey = val;
        sessionStorage.setItem('aneta_research_key', val);
        closeAuthModal();
        openChatDrawer();
      } else {
        authError.classList.remove('hidden');
      }
    }

    function openChatDrawer() {
      chatDrawer.classList.remove('hidden');
    }

    function closeChatDrawer() {
      chatDrawer.classList.add('hidden');
    }

    function setResearchMode(mode) {
      currentMode = mode;
      if (mode === 'new') {
        modeNewBtn.className = "px-3 py-1 rounded-lg text-xs font-semibold bg-[#111111] text-white transition";
        modeEnhanceBtn.className = "px-3 py-1 rounded-lg text-xs font-medium text-[#615d59] hover:bg-black/5 transition";
        enhanceSelectContainer.classList.add('hidden');
      } else {
        modeEnhanceBtn.className = "px-3 py-1 rounded-lg text-xs font-semibold bg-[#111111] text-white transition";
        modeNewBtn.className = "px-3 py-1 rounded-lg text-xs font-medium text-[#615d59] hover:bg-black/5 transition";
        enhanceSelectContainer.classList.remove('hidden');
      }
    }

    function openEnhanceModal(slug, title) {
      if (currentAuthKey !== 'guardian8') {
        openAuthModal();
        return;
      }
      setResearchMode('enhance');
      targetResearchSelect.value = slug;
      openChatDrawer();
    }

    async function handleSendResearch(e) {
      if (e && e.preventDefault) e.preventDefault();
      const topicInput = document.getElementById('chatInput');
      const topic = topicInput.value.trim();
      if (!topic) return;

      const userBox = document.createElement('div');
      userBox.className = "bg-[#e6f3fe] border border-[#0075de]/20 rounded-xl p-3 text-xs text-[#111111] ml-6";
      userBox.innerHTML = '<div class="font-bold text-[#0075de] mb-1">Topik Riset [' + currentMode.toUpperCase() + ']:</div><div>' + topic + '</div>';
      chatMessages.appendChild(userBox);
      topicInput.value = '';

      const agentBox = document.createElement('div');
      agentBox.className = "bg-white border border-black/[0.08] rounded-xl p-4 text-xs text-[#615d59] shadow-sm";
      agentBox.innerHTML = 
        '<div class="font-bold text-[#111111] mb-1 flex items-center gap-1.5">' +
          '<span class="w-2 h-2 rounded-full bg-[#0075de] animate-ping"></span>' +
          '<span>Menjalankan Autonomous Deep Research Pipeline (9router LLM)...</span>' +
        '</div>' +
        '<p class="text-[11px] font-mono text-[#757575] mt-1">' +
          '&bull; Model: 9router 9combos-agy<br>' +
          '&bull; Synthesis: Perplexity Pro Research Style<br>' +
          '&bull; Output: Standalone Notion DESIGN.md HTML' +
        '</p>';
      chatMessages.appendChild(agentBox);
      chatMessages.scrollTop = chatMessages.scrollHeight;

      const submitBtn = document.getElementById('submitResearchBtn');
      submitBtn.disabled = true;
      submitBtn.classList.add('opacity-50');

      try {
        const payload = {
          topic: topic,
          mode: currentMode,
          existing_slug: currentMode === 'enhance' ? targetResearchSelect.value : '',
          secret_key: currentAuthKey
        };

        let result = null;

        // 1. Try server endpoint first
        try {
          const res = await fetch('/api/research', {
            method: 'POST',
            headers: {
              'Content-Type': 'application/json',
              'X-Secret-Key': currentAuthKey
            },
            body: JSON.stringify(payload)
          });

          if (res.ok) {
            const data = await res.json();
            if (data && data.success && data.result) {
              result = data.result;
            }
          }
        } catch (_) {}

        // 2. Client-side fallback to 9router API if server API route is pending Nginx reload
        if (!result) {
          const sysPrompt = "You are an elite Tech Research Analyst at Aneta Research. Produce comprehensive technology benchmarks formatted strictly as standalone, responsive, interactive HTML applications matching Notion DESIGN.MD: warm paper canvas (#f6f5f4), white cards (#ffffff) with 1px hairline border, Notion Blue (#0075de) buttons, Inter font. Include Header, Title, 4 Metric KPI Cards, Search filter, Structured Table, and 3-column Recommendations.";
          const usrPrompt = "Topic: " + topic + "\\nMode: " + currentMode + "\\nGenerate JSON: {\\\"title\\\": \\\"...\\\", \\\"slug\\\": \\\"reseller-ai-token-local-9router\\\", \\\"category\\\": \\\"AI Models\\\", \\\"summary\\\": \\\"...\\\", \\\"tags\\\": [\\\"Reseller\\\", \\\"9router\\\"], \\\"html\\\": \\\"<!DOCTYPE html>...\\\"}\\nReturn STRICTLY valid JSON without backticks.";

          const llmRes = await fetch("https://wago.mytsel.id/webhook", {
            method: "POST",
            headers: {
              "Content-Type": "application/json"
            },
            body: JSON.stringify({
              url: "https://9router.mytsel.id/v1/chat/completions",
              method: "POST",
              headers: {
                "Content-Type": "application/json",
                "Authorization": "Bearer sk-088ecd7360b12ce3-18ymc8-86634ab0"
              },
              data: {
                model: "9combos-agy",
                messages: [
                  { role: "system", content: sysPrompt },
                  { role: "user", content: usrPrompt }
                ],
                stream: false,
                temperature: 0.3
              }
            })
          }).catch(() => null);

          let llmData = null;
          if (llmRes && llmRes.ok) {
            llmData = await llmRes.json();
          } else {
            // direct fetch
            const directRes = await fetch("https://9router.mytsel.id/v1/chat/completions", {
              method: "POST",
              headers: {
                "Content-Type": "application/json",
                "Authorization": "Bearer sk-088ecd7360b12ce3-18ymc8-86634ab0"
              },
              body: JSON.stringify({
                model: "9combos-agy",
                messages: [
                  { role: "system", content: sysPrompt },
                  { role: "user", content: usrPrompt }
                ],
                stream: false,
                temperature: 0.3
              })
            });
            llmData = await directRes.json();
          }

          let rawContent = llmData.choices[0].message.content.trim();
          if (rawContent.indexOf(String.fromCharCode(96,96,96)) === 0) {
            const lines = rawContent.split(String.fromCharCode(10));
            rawContent = lines.slice(1, lines.length - 1).join(String.fromCharCode(10)).trim();
          }
          const parsed = JSON.parse(rawContent);
          const blob = new Blob([parsed.html], { type: "text/html;charset=utf-8" });
          const blobUrl = URL.createObjectURL(blob);

          result = {
            slug: parsed.slug || "research-result",
            title: parsed.title,
            category: parsed.category || "AI Models",
            summary: parsed.summary,
            url: blobUrl
          };
        }

        if (result) {
          agentBox.innerHTML = 
            '<div class="font-bold text-[#10b981] mb-1 flex items-center gap-1.5">' +
              '<svg class="w-4 h-4 text-[#10b981]" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 13l4 4L19 7"/></svg>' +
              '<span>Riset Berhasil Selesai & Diterbitkan!</span>' +
            '</div>' +
            '<h4 class="font-bold text-sm text-[#111111] mt-2">' + result.title + '</h4>' +
            '<p class="text-xs text-[#615d59] mt-1">' + result.summary + '</p>' +
            '<div class="mt-3 flex items-center gap-2">' +
              '<a href="' + result.url + '" target="_blank" class="px-3 py-1.5 rounded-lg bg-[#0075de] hover:bg-[#0060b9] text-white text-xs font-medium inline-flex items-center gap-1 shadow-sm">' +
                '<span>Buka Hasil Riset Lengkap</span>' +
                '<svg class="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M10 6H6a2 2 0 00-2 2v10a2 2 0 002 2h10a2 2 0 002-2v-4M14 4h6m0 0v6m0-6L10 14"/></svg>' +
              '</a>' +
              '<button onclick="window.location.reload()" class="px-3 py-1.5 rounded-lg bg-[#f6f5f4] hover:bg-black/5 text-[#111111] text-xs font-medium border border-black/[0.08]">' +
                'Refresh Beranda' +
              '</button>' +
            '</div>';
        } else {
          agentBox.innerHTML = 
            '<div class="font-bold text-[#f64932] mb-1">Riset Gagal</div>' +
            '<p class="text-xs text-[#615d59]">Tidak mendapatkan respon dari engine.</p>';
        }
      } catch (err) {
        agentBox.innerHTML = 
          '<div class="font-bold text-[#f64932] mb-1">Koneksi Error</div>' +
          '<p class="text-xs text-[#615d59]">' + err.message + '</p>';
      } finally {
        submitBtn.disabled = false;
        submitBtn.classList.remove('opacity-50');
        chatMessages.scrollTop = chatMessages.scrollHeight;
      }
    }

    // Client-side search & filter
    let activeCategory = 'Semua';
    const cards = document.querySelectorAll('.report-card');

    function filterReports() {
      const q = (searchInput.value || '').toLowerCase().trim();
      let count = 0;

      cards.forEach(card => {
        const cat = card.getAttribute('data-category') || '';
        const title = (card.getAttribute('data-title') || '').toLowerCase();
        const text = card.innerText.toLowerCase();

        const matchCat = (activeCategory === 'Semua') || cat.toLowerCase().includes(activeCategory.toLowerCase());
        const matchQuery = !q || text.includes(q) || title.includes(q);

        if (matchCat && matchQuery) {
          card.style.display = '';
          count++;
        } else {
          card.style.display = 'none';
        }
      });

      if (count === 0) {
        noResults.classList.remove('hidden');
      } else {
        noResults.classList.add('hidden');
      }
    }

    function filterCategory(cat) {
      activeCategory = cat;
      document.querySelectorAll('.cat-chip').forEach(btn => {
        if (btn.innerText.trim() === cat) {
          btn.className = "cat-chip px-3 py-1 rounded-lg text-xs font-semibold bg-[#111111] text-white";
        } else {
          btn.className = "cat-chip px-3 py-1 rounded-lg text-xs font-medium text-[#615d59] hover:bg-[#f6f5f4] transition";
        }
      });
      filterReports();
    }

    searchInput.addEventListener('input', filterReports);
  </script>
</body>
</html>
`, len(reports), cardsHTML.String(), time.Now().Year(), "allReportsDataPlaceholder")
	html = strings.Replace(html, "\"allReportsDataPlaceholder\"", string(jsonReports), 1)

	indexPath := filepath.Join(".", "index.html")
	os.WriteFile(indexPath, []byte(html), 0644)
	log.Printf("[build] Successfully generated index.html (%d bytes)", len(html))
}
func handleReports(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-Secret-Key")
	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	reports := collectReports()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"reports": reports,
	})
}

func handleResearch(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-Secret-Key")
	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req ResearchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(400)
		json.NewEncoder(w).Encode(ResearchResponse{Success: false, Error: "Invalid JSON format"})
		return
	}

	key := req.SecretKey
	if key == "" {
		key = r.Header.Get("X-Secret-Key")
	}
	if key != SecretKey {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(401)
		json.NewEncoder(w).Encode(ResearchResponse{Success: false, Error: "Secret key salah. Akses ditolak."})
		return
	}

	if strings.TrimSpace(req.Topic) == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(400)
		json.NewEncoder(w).Encode(ResearchResponse{Success: false, Error: "Topik riset tidak boleh kosong."})
		return
	}

	// 1. Run Lightpanda browser search
	log.Printf("[research] Running Lightpanda MCP search for query: %s", req.Topic)
	searchResults := searchLightpandaMCP(req.Topic)
	searchContext, _ := json.MarshalIndent(searchResults, "", "  ")

	// 2. Read existing content if mode == enhance
	existingContent := ""
	if req.Mode == "enhance" && req.ExistingSlug != "" {
		htmlPath := filepath.Join(".", "report-summaries", req.ExistingSlug+".html")
		if b, err := os.ReadFile(htmlPath); err == nil {
			existingContent = string(b)
			if len(existingContent) > 8000 {
				existingContent = existingContent[:8000] + "... [truncated]"
			}
		}
	}

	// 3. System prompt & user prompt for 9router 9combos-agy
	systemPrompt := `You are an elite Tech Research Analyst and Senior Frontend Engineer at Aneta Research.
You produce comprehensive, executive-grade technology and AI benchmarks formatted strictly as standalone, responsive, interactive HTML applications.

CRITICAL DESIGN REQUIREMENT (NOTION DESIGN SYSTEM - DESIGN.MD):
You MUST follow the Notion Design System exactly as defined in DESIGN.MD:
- Theme: Light / Warm Paper Canvas ("warm paper notebook under afternoon sun").
- Background: #f6f5f4 (--color-paper-warmth). NEVER use dark/navy/cyberpunk backgrounds!
- Cards / Surfaces: #ffffff (--color-pure-white) with 1px hairline border rgba(0, 0, 0, 0.08), 12px border radius (rounded-xl), NO heavy drop shadows.
- Primary Action / CTA: #0075de (--color-notion-blue), text #ffffff, 8px radius.
- Secondary Actions / Badges: #e6f3fe (sky tint) with #0075de text, or warm subtle pills.
- Typography:
  - Font: Inter, -apple-system, system-ui, sans-serif.
  - Headings: #111111 / #000000 with tight negative tracking (-0.03em).
  - Body Text: #615d59 (warm graphite) and #111111, muted labels #757575.
  - Editorial Accents: Source Serif 4 or serif quotes where appropriate.
- Accent Highlights (use sparingly):
  - Marigold #ffb110, Coral #f64932, Signal Blue #097fe8.
- Layout Sections:
  1. Header: Aneta Research breadcrumb & badges (in #e6f3fe / #0075de), publication date.
  2. Large title with tight letter-spacing + executive summary in warm graphite.
  3. 4 Executive Metric Cards: Pure white (#ffffff) cards with 1px solid rgba(0,0,0,0.08) border, large bold stats.
  4. Search bar & filter pills (Notion pills, rounded-full, 1px border).
  5. Interactive Data Table: Clean white card container, subtle zebra or border dividers, clear status badges.
  6. Actionable Takeaways & Recommendations in 3-column white cards.`

	userPrompt := fmt.Sprintf(`Topic: %s
Mode: %s
Existing Target Slug: %s

Lightpanda Live Web Search Context:
%s

Existing Report Content (if any):
%s

Generate the research result as a JSON object with this exact schema:
{
  "title": "Title of the research report",
  "slug": "url-friendly-slug",
  "category": "AI Models / Cloud & Infra / Telkomsel / FinTech",
  "summary": "1-2 sentence executive summary of key findings",
  "tags": ["Tag1", "Tag2"],
  "html": "<!DOCTYPE html><html lang=\"id\" class=\"dark\">...complete standalone html...</html>"
}
Output STRICTLY valid JSON. No markdown backticks, no markdown wrapping.`, req.Topic, req.Mode, req.ExistingSlug, string(searchContext), existingContent)

	log.Printf("[research] Calling 9router LLM (%s)...", DefaultModel)
	rawLLM, err := call9Router(systemPrompt, userPrompt)
	if err != nil {
		log.Printf("[research] LLM error: %v", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(500)
		json.NewEncoder(w).Encode(ResearchResponse{Success: false, Error: "LLM synthesis error: " + err.Error()})
		return
	}

	cleanJSON := strings.TrimSpace(rawLLM)
	if strings.HasPrefix(cleanJSON, "```") {
		lines := strings.Split(cleanJSON, "\n")
		if len(lines) > 2 {
			cleanJSON = strings.Join(lines[1:len(lines)-1], "\n")
		}
	}

	var resultData struct {
		Title    string   `json:"title"`
		Slug     string   `json:"slug"`
		Category string   `json:"category"`
		Summary  string   `json:"summary"`
		Tags     []string `json:"tags"`
		HTML     string   `json:"html"`
	}

	if err := json.Unmarshal([]byte(cleanJSON), &resultData); err != nil {
		re := regexp.MustCompile(`(?s)\{.*\}`)
		match := re.FindString(cleanJSON)
		if err := json.Unmarshal([]byte(match), &resultData); err != nil {
			log.Printf("[research] JSON parse error: %v", err)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(500)
			json.NewEncoder(w).Encode(ResearchResponse{Success: false, Error: "Failed to parse research output: " + err.Error()})
			return
		}
	}

	slug := resultData.Slug
	if req.Mode == "enhance" && req.ExistingSlug != "" {
		slug = req.ExistingSlug
	}
	reg := regexp.MustCompile(`[^a-z0-9\-]+`)
	slug = reg.ReplaceAllString(strings.ToLower(strings.ReplaceAll(slug, " ", "-")), "")
	if slug == "" {
		slug = fmt.Sprintf("research-%d", time.Now().Unix())
	}

	// Save HTML & MD
	reportsDir := filepath.Join(".", "report-summaries")
	os.MkdirAll(reportsDir, 0755)

	htmlPath := filepath.Join(reportsDir, slug+".html")
	if err := os.WriteFile(htmlPath, []byte(resultData.HTML), 0644); err != nil {
		log.Printf("[research] Write HTML error: %v", err)
	}

	tagsYaml := ""
	for _, t := range resultData.Tags {
		tagsYaml += fmt.Sprintf("  - %s\n", t)
	}
	today := time.Now().Format("2006-01-02")
	mdContent := fmt.Sprintf(`---
title: %s
slug: %s
date: %s
category: %s
author: Aneta Intelligence Desk
source: Aneta Deep Research
summary: %s
tags:
%s---

# %s

%s

[Buka Laporan & Dashboard Interaktif Lengkap](report-summaries/%s.html)
`, resultData.Title, slug, today, resultData.Category, resultData.Summary, tagsYaml, resultData.Title, resultData.Summary, slug)

	mdPath := filepath.Join(reportsDir, slug+".md")
	os.WriteFile(mdPath, []byte(mdContent), 0644)

	// Re-generate routes & index.html directly in Go
	log.Println("[research] Rebuilding routes & index.html via Go builder...")
	reports := collectReports()
	generateRoutes(reports)
	buildIndexHTML(reports)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(ResearchResponse{
		Success: true,
		Result: map[string]interface{}{
			"slug":     slug,
			"title":    resultData.Title,
			"category": resultData.Category,
			"summary":  resultData.Summary,
			"url":      "/" + slug,
		},
	})
}

func main() {
	if len(os.Args) > 1 && os.Args[1] == "build" {
		log.Println("[cli] Running static site generator in Go...")
		reports := collectReports()
		generateRoutes(reports)
		buildIndexHTML(reports)
		log.Printf("[cli] Build completed successfully! %d reports indexed.", len(reports))
		return
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8088"
	}

	http.HandleFunc("/api/reports", handleReports)
	http.HandleFunc("/api/research", handleResearch)
	http.HandleFunc("/api/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"ok","engine":"go-aneta-research"}`))
	})

	log.Printf("Aneta Research Go API Server running on port %s...", port)
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
