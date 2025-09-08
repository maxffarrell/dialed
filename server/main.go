package main

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/robfig/cron/v3"
)

var (
	// configurable via env
	dataDir        = env("DATA_DIR", "./data")
	nvidiaKey      = mustEnv("NVIDIA_API_KEY")
	openAIKey      = mustEnv("OPENAI_API_KEY")
	nvidiaSTTURL   = env("NVIDIA_STT_URL", "https://integrate.api.nvidia.com/v1/audio/transcriptions")
	parakeetModel  = env("NVIDIA_STT_MODEL", "nvidia/parakeet-tdt-0.6b-v2")
	openAIRespURL  = env("OPENAI_RESPONSES_URL", "https://api.openai.com/v1/responses")
	openAIModel    = env("OPENAI_MODEL", "gpt-4o-mini")
	serverAddr     = env("ADDR", ":8080")
	tlsInsecure    = env("TLS_INSECURE", "") != "" // allow toggling for corporate proxies, etc.

	logDir      = ""
	sumDailyDir = ""
	sumWeeklyDir= ""
	sumMonthlyDir=""
	goalsDir    = ""
)

func env(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func mustEnv(k string) string {
	v := os.Getenv(k)
	if v == "" {
		log.Fatalf("missing required env: %s", k)
	}
	return v
}

func ensureDirs() {
	logDir = filepath.Join(dataDir, "logs")
	sumDailyDir = filepath.Join(dataDir, "summaries", "daily")
	sumWeeklyDir= filepath.Join(dataDir, "summaries", "weekly")
	sumMonthlyDir=filepath.Join(dataDir, "summaries", "monthly")
	goalsDir = filepath.Join(dataDir, "goals")

	for _, d := range []string{logDir, sumDailyDir, sumWeeklyDir, sumMonthlyDir, goalsDir} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			log.Fatalf("mkdir %s: %v", d, err)
		}
	}
}

func main() {
	ensureDirs()

	mux := http.NewServeMux()
	mux.HandleFunc("/ingest", handleIngest)
	mux.HandleFunc("/summarize/", handleSummarize) // /summarize/daily|weekly|monthly
	mux.HandleFunc("/goals/today", handleGoalsToday)
	mux.HandleFunc("/goals/generate", handleGoalsGenerate)

	// scheduler: 00:10 UTC — daily; weekly each Monday; monthly on day 1; also goals
	c := cron.New(cron.WithParser(cron.NewParser(cron.SecondOptional|cron.Minute|cron.Hour|cron.Dom|cron.Month|cron.Dow|cron.Descriptor)))
	_, _ = c.AddFunc("10 0 * * *", func() { // 00:10 UTC
		today := time.Now().UTC()
		_ = summarizeDaily(today)
		if today.Weekday() == time.Monday {
			_ = summarizeWeekly(today)
		}
		if today.Day() == 1 {
			_ = summarizeMonthly(today)
		}
		_, _ = generateGoals(today)
	})
	c.Start()
	defer c.Stop()

	log.Printf("GoalDial Cloud (Go) listening on %s", serverAddr)
	srv := &http.Server{Addr: serverAddr, Handler: mux}
	log.Fatal(srv.ListenAndServe())
}

// ========== /ingest ==========

func handleIngest(w http.ResponseWriter, r *http.Request) {
	// Expect: audio/L16; rate=16000; channels=1 with chunked transfer
	q := r.URL.Query()
	deviceID := q.Get("device_id")
	if deviceID == "" {
		http.Error(w, "device_id required", http.StatusBadRequest)
		return
	}
	taskID := q.Get("task_id")
	if taskID == "" {
		taskID = "current"
	}

	var t time.Time
	if ts := q.Get("ts"); ts != "" {
		if sec, err := strconv.ParseInt(ts, 10, 64); err == nil {
			t = time.Unix(sec, 0).UTC()
		}
	}
	if t.IsZero() {
		t = time.Now().UTC()
	}

	// Read entire body (typical clip ~ <1MB for 20s @16kHz)
	raw, err := io.ReadAll(r.Body)
	if err != nil || len(raw) == 0 {
		http.Error(w, "empty body or read error", http.StatusBadRequest)
		return
	}

	// Wrap to WAV
	wav := wrapPCMToWAV(raw, 16000, 1, 2) // 16kHz, mono, 16bit

	// Send to NVIDIA Parakeet
	txt, err := transcribeParakeet(r.Context(), wav)
	if err != nil {
		http.Error(w, "STT error: "+err.Error(), http.StatusBadGateway)
		return
	}

	// Append Markdown log
	path, _ := appendMarkdownLog(t, deviceID, taskID, txt)

	resp := map[string]any{"ok": true, "transcript": txt, "log": path}
	_ = json.NewEncoder(w).Encode(resp)
}

func wrapPCMToWAV(pcm []byte, sampleRate, channels, sampleWidth int) []byte {
	// WAV header (PCM)
	byteRate := sampleRate * channels * sampleWidth
	blockAlign := channels * sampleWidth
	dataSize := len(pcm)
	riffSize := 36 + dataSize

	var b bytes.Buffer
	b.WriteString("RIFF")
	b.Write(u32le(uint32(riffSize)))
	b.WriteString("WAVE")

	// fmt chunk
	b.WriteString("fmt ")
	b.Write(u32le(16))                 // chunk size
	b.Write(u16le(1))                  // PCM format
	b.Write(u16le(uint16(channels)))
	b.Write(u32le(uint32(sampleRate)))
	b.Write(u32le(uint32(byteRate)))
	b.Write(u16le(uint16(blockAlign)))
	b.Write(u16le(uint16(sampleWidth * 8))) // bits per sample

	// data chunk
	b.WriteString("data")
	b.Write(u32le(uint32(dataSize)))
	b.Write(pcm)
	return b.Bytes()
}

func u16le(v uint16) []byte { return []byte{byte(v), byte(v >> 8)} }
func u32le(v uint32) []byte { return []byte{byte(v), byte(v >> 8), byte(v >> 16), byte(v >> 24)} }

func transcribeParakeet(ctx context.Context, wav []byte) (string, error) {
	// multipart form: file + model
	var body bytes.Buffer
	w := multipart.NewWriter(&body)

	fw, _ := w.CreateFormFile("file", "audio.wav")
	_, _ = fw.Write(wav)
	_ = w.WriteField("model", parakeetModel)
	_ = w.Close()

	tr := &http.Transport{}
	if tlsInsecure {
		tr.TLSClientConfig = &tls.Config{InsecureSkipVerify: true} // optional
	}
	client := &http.Client{Timeout: 120 * time.Second, Transport: tr}

	req, _ := http.NewRequestWithContext(ctx, "POST", nvidiaSTTURL, &body)
	req.Header.Set("Authorization", "Bearer "+nvidiaKey)
	req.Header.Set("Content-Type", w.FormDataContentType())

	res, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()
	bs, _ := io.ReadAll(res.Body)
	if res.StatusCode != 200 {
		return "", fmt.Errorf("%s: %s", res.Status, string(bs))
	}

	var out map[string]any
	if json.Unmarshal(bs, &out) == nil {
		// common shapes: {"text": "..."} or {"output_text": "..."}
		if s, ok := out["text"].(string); ok && s != "" {
			return strings.TrimSpace(s), nil
		}
		if s, ok := out["output_text"].(string); ok && s != "" {
			return strings.TrimSpace(s), nil
		}
	}
	// Fallback: return raw
	return strings.TrimSpace(string(bs)), nil
}

func appendMarkdownLog(t time.Time, deviceID, taskID, text string) (string, error) {
	day := t.UTC().Format("2006-01-02")
	p := filepath.Join(logDir, day+".md")
	_ = os.MkdirAll(filepath.Dir(p), 0o755)
	entry := fmt.Sprintf("### %s — %s / %s\n- Note: %s\n\n", t.UTC().Format("15:04"), deviceID, taskID, text)
	f, err := os.OpenFile(p, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return p, err
	}
	defer f.Close()
	_, err = io.WriteString(f, entry)
	return p, err
}

// ========== Summaries ==========

const dailyPrompt = `You are a concise project journal editor.
Input is a Markdown daily log of short completion notes.
Write a compact **Daily Summary** in Markdown with:
- 3–6 bullet highlights (actionable phrasing)
- 1 risk/blocked item if any
- 1 concrete next step
Stay under 120 words.`

const weeklyPrompt = `Summarize the week's Markdown logs into:
## Weekly Summary
- Top wins (3–5)
- Metrics/Signals (if present)
- Risks & Mitigations (≤3)
- Priorities for next week (3 exact bullets)
Tone: crisp, operational.`

const monthlyPrompt = `Create a one-page **Monthly Summary** from Markdown logs:
- Progress by workstream
- What moved the needle (evidence)
- Decisions made
- What to stop/start/continue (3 each)
Limit to ~250 words.`

func handleSummarize(w http.ResponseWriter, r *http.Request) {
	// /summarize/daily | /summarize/weekly | /summarize/monthly
	period := strings.TrimPrefix(r.URL.Path, "/summarize/")
	now := time.Now().UTC()
	var path string
	var err error
	switch period {
	case "daily":
		err = summarizeDaily(now)
		path = filepath.Join(sumDailyDir, now.Format("2006-01-02")+".md")
	case "weekly":
		err = summarizeWeekly(now)
		path = filepath.Join(sumWeeklyDir, isoYearWeek(now)+".md")
	case "monthly":
		err = summarizeMonthly(now)
		path = filepath.Join(sumMonthlyDir, now.Format("2006-01")+".md")
	default:
		http.Error(w, "period must be daily|weekly|monthly", http.StatusBadRequest)
		return
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	_ = json.NewEncoder(w).Encode(map[string]string{"ok": "true", "path": path})
}

func summarizeDaily(now time.Time) error {
	src := filepath.Join(logDir, now.Format("2006-01-02")+".md")
	text := readFile(src)
	if strings.TrimSpace(text) == "" {
		return nil
	}
	out, err := callOpenAI(dailyPrompt + "\n\n" + text)
	if err != nil {
		return err
	}
	dst := filepath.Join(sumDailyDir, now.Format("2006-01-02")+".md")
	return writeFile(dst, out)
}

func summarizeWeekly(now time.Time) error {
	var b strings.Builder
	for i := 0; i < 7; i++ {
		d := now.AddDate(0, 0, -i).Format("2006-01-02")
		b.WriteString(readFile(filepath.Join(logDir, d+".md")))
		b.WriteString("\n")
	}
	out, err := callOpenAI(weeklyPrompt + "\n\n" + b.String())
	if err != nil {
		return err
	}
	dst := filepath.Join(sumWeeklyDir, isoYearWeek(now)+".md")
	return writeFile(dst, out)
}

func summarizeMonthly(now time.Time) error {
	start := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	next := start.AddDate(0, 1, 0)
	var b strings.Builder
	for d := start; d.Before(next); d = d.Add(24 * time.Hour) {
		b.WriteString(readFile(filepath.Join(logDir, d.Format("2006-01-02")+".md")))
		b.WriteString("\n")
	}
	out, err := callOpenAI(monthlyPrompt + "\n\n" + b.String())
	if err != nil {
		return err
	}
	dst := filepath.Join(sumMonthlyDir, now.Format("2006-01")+".md")
	return writeFile(dst, out)
}

func isoYearWeek(t time.Time) string {
	y, w := t.ISOWeek()
	return fmt.Sprintf("%04d-%02d", y, w)
}

// ========== Goals ==========

const goalPrompt = `You are an execution coach. From the MEMORY below, output exactly THREE goals for TODAY.
Rules:
- Each goal must be one crisp, verifiable action (<= 16 words).
- Prefer leverage: unblock dependencies, run the highest-ROI experiment, or finish a near-done task.
- Avoid vague goals ("work on", "brainstorm"). No duplicates.
Format JSON: {"date":"YYYY-MM-DD","goals":[{"title":"..."},{"title":"..."},{"title":"..."}]}

MEMORY:
`

func handleGoalsToday(w http.ResponseWriter, r *http.Request) {
	now := time.Now().UTC()
	p := filepath.Join(goalsDir, now.Format("2006-01-02")+".json")
	if bs, err := os.ReadFile(p); err == nil {
		w.Header().Set("Content-Type", "application/json")
		w.Write(bs)
		return
	}
	data, _ := generateGoals(now)
	_ = json.NewEncoder(w).Encode(data)
}

func handleGoalsGenerate(w http.ResponseWriter, r *http.Request) {
	now := time.Now().UTC()
	data, _ := generateGoals(now)
	_ = json.NewEncoder(w).Encode(data)
}

func generateGoals(now time.Time) (map[string]any, error) {
	yesterday := now.AddDate(0, 0, -1)
	var mem strings.Builder
	mem.WriteString(readFile(filepath.Join(logDir, yesterday.Format("2006-01-02")+".md")))
	mem.WriteString("\n")
	mem.WriteString(readFile(filepath.Join(sumWeeklyDir, isoYearWeek(now)+".md")))
	if mem.Len() == 0 {
		// fallback to last week
		mem.WriteString(readFile(filepath.Join(sumWeeklyDir, isoYearWeek(now.AddDate(0, 0, -7))+".md")))
	}
	mem.WriteString("\n")
	mem.WriteString(readFile(filepath.Join(sumMonthlyDir, now.Format("2006-01")+".md")))

	out, err := callOpenAIJSON(goalPrompt + mem.String())
	if err != nil {
		return nil, err
	}
	var data map[string]any
	if json.Unmarshal([]byte(out), &data) != nil {
		// very defensive fallback
		lines := strings.Split(out, "\n")
		var g []map[string]string
		for _, ln := range lines {
			ln = strings.TrimSpace(strings.TrimPrefix(ln, "-"))
			if ln != "" {
				g = append(g, map[string]string{"title": ln})
			}
			if len(g) == 3 {
				break
			}
		}
		data = map[string]any{"date": now.Format("2006-01-02"), "goals": g}
	} else {
		data["date"] = now.Format("2006-01-02")
	}

	dst := filepath.Join(goalsDir, now.Format("2006-01-02")+".json")
	_ = writeFile(dst, toJSON(data))
	return data, nil
}

// ========== OpenAI Responses (helper) ==========

func callOpenAI(prompt string) (string, error) {
	payload := map[string]any{
		"model": openAIModel,
		"input": []map[string]string{{"role": "user", "content": prompt}},
	}
	return openAIRequest(payload)
}

func callOpenAIJSON(prompt string) (string, error) {
	payload := map[string]any{
		"model": openAIModel,
		"input": []map[string]string{{"role": "user", "content": prompt}},
		"response_format": map[string]any{"type": "json_object"},
	}
	return openAIRequest(payload)
}

func openAIRequest(payload map[string]any) (string, error) {
	bs, _ := json.Marshal(payload)

	tr := &http.Transport{}
	if tlsInsecure {
		tr.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	}
	client := &http.Client{Timeout: 120 * time.Second, Transport: tr}

	req, _ := http.NewRequestWithContext(context.Background(), "POST", openAIRespURL, bytes.NewReader(bs))
	req.Header.Set("Authorization", "Bearer "+openAIKey)
	req.Header.Set("Content-Type", "application/json")

	res, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()
	body, _ := io.ReadAll(res.Body)
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return "", fmt.Errorf("%s: %s", res.Status, string(body))
	}
	var out map[string]any
	if json.Unmarshal(body, &out) == nil {
		if s, ok := out["output_text"].(string); ok && s != "" {
			return s, nil
		}
	}
	return string(body), nil
}

// ========== utils ==========

func readFile(p string) string {
	bs, err := os.ReadFile(p)
	if err != nil {
		return ""
	}
	return string(bs)
}

func writeFile(p, s string) error {
	_ = os.MkdirAll(filepath.Dir(p), 0o755)
	return os.WriteFile(p, []byte(s), 0o644)
}

func toJSON(v any) string {
	bs, _ := json.MarshalIndent(v, "", "  ")
	return string(bs)
}
