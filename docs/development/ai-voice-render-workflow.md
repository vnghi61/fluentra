# Hướng Dẫn Kiến Trúc AI, Voice Generation & Cơ Chế Auto-Wake Trên Render

Tài liệu này tổng hợp toàn bộ phân tích kiến trúc, cấu hình môi trường và các thay đổi mã nguồn đã được thực hiện cho hệ thống **Fluentra** xoay quanh 3 chủ đề chính:

1. **Cấu hình AI Provider (Mô hình 4 Slots vs Biến cũ)**
2. **Hệ thống tạo giọng đọc (TTS) & Tinh chỉnh tốc độ bài nghe**
3. **Cơ chế Fallback tự động đánh thức Worker khi Render Sleep**

---

## I. Cấu hình AI Provider (Hợp nhất 4 Numbered Slots)

### 1. Hiện trạng và nguyên lý hoạt động

Fluentra sử dụng một adapter chuẩn duy nhất là `OpenAICompatibleProvider` ([internal/platform/ai/provider_openai.go](../../internal/platform/ai/provider_openai.go)), tương thích với mọi dịch vụ cung cấp API chuẩn `POST /chat/completions` (OpenAI, Groq, Cerebras, OpenCode, OpenRouter, Mistral, Ollama, vLLM, LM Studio...).

Hệ thống quản lý AI qua **4 numbered provider slots** ([cmd/api/main.go:L155-L181](../../cmd/api/main.go#L155-L181)):

* **Slot 1 (`AI_PROVIDER_1_*`)**: Provider chính (Primary).
* **Slot 2–4 (`AI_PROVIDER_2_*` ... `4_*`)**: Fallback chain dự phòng khi slot trước gặp sự cố hoặc hết quota.

### 2. Điểm lưu ý quan trọng với `mock`

Trong [internal/platform/ai/module.go:L109-L113](../../internal/platform/ai/module.go#L109-L113):

```go
if name == ProviderMock {
    built = append(built, NewMockProvider(registry))
    continue
}
```

* Khi `AI_PROVIDER_1_NAME=mock`, hệ thống khởi tạo `MockProvider` (chế độ offline, tự chấp nhận mọi từ vựng mà không gọi ra ngoài).
* **Toàn bộ `BASE_URL`, `MODEL`, `API_KEY` sẽ bị bỏ qua**.
* Muốn kích hoạt AI thật, **bắt buộc đổi `NAME` sang một tên bất kỳ khác `mock`** (ví dụ: `groq`, `opencode`, `openai`).

### 3. Cụm biến cấu hình cũ (Đã bị loại bỏ)

Các biến sau **không còn được nạp vào code**:

```env
AI_ENABLED=true
AI_TASK_ROUTES=config/ai-routes.yaml
AI_DEFAULT_CHAIN=anthropic,openai
AI_DAILY_BUDGET_USD=50
AI_BUDGET_WARN_PERCENT=80
AI_REQUEST_TIMEOUT=30s
AI_STREAM_TIMEOUT=120s
AI_MAX_RETRIES=3
AI_BREAKER_FAILURE_RATIO=0.5
AI_CACHE_ENABLED=true
AI_SEMANTIC_CACHE_ENABLED=false
ANTHROPIC_API_KEY=
OPENAI_API_KEY=
GEMINI_API_KEY=
OPENROUTER_API_KEY=
OLLAMA_BASE_URL=
```

**Dẫn chứng từ mã nguồn & tài liệu:**

1. **[docs/development/phase-3-work-order-7.md:L125-L135](../../docs/development/phase-3-work-order-7.md#L125-L135)**:
   > *"Replace the existing ten `AI_*` / `AI_FALLBACK_*` keys rather than layering the new ones beside them... Four slots, twenty keys, all in .env.example..."*
2. **[.env.example:L177-L179](../../.env.example#L177-L179)**:
   > *"# Everything below describes internal/platform/ai's full specification... and is NOT read by any code yet."*
3. **[cmd/api/main.go:L155-L181](../../cmd/api/main.go#L155-L181)** và **[cmd/worker/main.go:L119-L145](../../cmd/worker/main.go#L119-L145)**: Struct `AI` chỉ nhận diện các trường `provider_1_*` đến `provider_4_*`. Thư viện `koanf` tự động drop tất cả các biến ngoài struct.

### 4. Mẫu cấu hình chuẩn khuyến nghị cho `.env`

```env
# -------------------------------------------------------------------- ai
AI_PROVIDER_1_NAME=opencode
AI_PROVIDER_1_BASE_URL=https://opencode.ai/inference/openai/v1
AI_PROVIDER_1_MODEL=claude-3-5-sonnet
AI_PROVIDER_1_API_KEY=opencode_token_here
AI_PROVIDER_1_TIMEOUT=120s

AI_PROVIDER_2_NAME=groq-backup
AI_PROVIDER_2_BASE_URL=https://api.groq.com/openai/v1
AI_PROVIDER_2_MODEL=llama-3.3-70b-versatile
AI_PROVIDER_2_API_KEY=your_groq_api_key_here
AI_PROVIDER_2_TIMEOUT=120s

AI_PROVIDER_3_NAME=cerebras
AI_PROVIDER_3_BASE_URL=https://api.cerebras.ai/v1
AI_PROVIDER_3_MODEL=qwen-3.8-27b
AI_PROVIDER_3_API_KEY=your_cerebras_api_key_here
AI_PROVIDER_3_TIMEOUT=120s

AI_PROVIDER_4_NAME=
AI_WRITING_DAILY_LIMIT=10
```

---

## II. Hệ Thống Tạo Giọng Đọc (TTS) & Tinh Chỉnh Tốc Độ Bài Nghe

### 1. Kiến trúc phân tách TTS và ASR

Theo thiết kế tại [docs/development/phase-3-work-order-12.md](../../docs/development/phase-3-work-order-12.md):

* **ASR (Speech-to-Text - Chấm điểm bài nói Speaking):** Dùng API OpenAI Whisper / Groq Whisper qua các biến `SPEECH_ASR_BASE_URL` và `SPEECH_ASR_API_KEY`.
* **TTS (Text-to-Speech - Tạo giọng đọc bài nghe Listening):** Không dùng Cloud API tốn tiền mà chạy **Piper TTS offline** hoàn toàn miễn phí trên CPU (`internal/platform/media/piper.go`). Kết quả audio WAV được upload lên Cloudflare R2 / MinIO và lưu metadata vào bảng `content.tts_cache`.

### 2. Sự khác biệt giữa Local và Production

* **Local:** Chạy `make tts` sử dụng binary `piper.exe` và model tải tại máy cá nhân (`C:\Users\HP\Downloads\...`).
* **Production CI/CD:** Server Render (gói Free) không đủ tài nguyên để render TTS. Do đó audio được render qua GitHub Actions workflow:
  * File workflow: [.github/workflows/tts-render.yml](../../.github/workflows/tts-render.yml).
  * Mỗi khi có bài nghe mới được sinh ra, Worker gửi tín hiệu GitHub Dispatch để workflow khởi chạy.
  * Workflow tải Piper Linux và model gốc `en_US-lessac-medium.onnx` + `.onnx.json` từ Hugging Face về Ubuntu runner, chạy `go run ./cmd/tts -all -engine piper` rồi upload lên R2.

### 3. Tinh chỉnh tốc độ đọc (Fix giọng đọc quá nhanh)

Trong Piper TTS, tốc độ đọc được quyết định bởi tham số **`length_scale`** (tỷ lệ kéo dài âm vị):

* Giá trị mặc định: `1.0` (tốc độ tự nhiên bản ngữ, học viên nghe sẽ cảm thấy bị nói lướt, quá nhanh).
* Tỷ lệ tối ưu cho bài luyện nghe: **`1.2` – `1.3`** (giảm tốc độ 20% – 30%, phát âm tròn vành rõ chữ, ngắt nghỉ câu rõ ràng).

#### Các phương án can thiệp

1. **Môi trường Local (`make tts`):**
   Mở file `C:\Users\HP\Downloads\en_US-lessac-medium.onnx.json`, sửa:

   ```json
   "inference": {
     "noise_scale": 0.667,
     "length_scale": 1.25,
     "noise_w": 0.8
   }
   ```

2. **Môi trường Production (CI/CD):**
   Vì GitHub Actions luôn tải file `.onnx.json` mới từ Hugging Face, cách xử lý triệt để nhất là thêm cờ `--length_scale` trực tiếp vào mã nguồn Go trong [internal/platform/media/piper.go:L76](../../internal/platform/media/piper.go#L76):

   ```go
   args := []string{
       "--model", filepath.Join(p.ModelDir, voice+".onnx"),
       "--output_file", outPath,
       "--length_scale", "1.25",
   }
   ```

   Hoặc trong bước download của workflow `.github/workflows/tts-render.yml`, dùng `sed -i 's/"length_scale": 1/"length_scale": 1.25/' .piper/voices/${PIPER_VOICE}.onnx.json`.

---

## III. Cơ Chế Fallback & Tự Động Đánh Thức Worker Khi Render Sleep

### 1. Vấn đề của Render Free Tier

* Khi không có lượt truy cập (sau ~15 phút), Render tự động đưa các web service vào trạng thái **Sleep** (ngủ đông).
* Khi sleep, toàn bộ bộ đếm thời gian (`time.Ticker`) và cron job định kỳ (như `top_up_practice_pool`, `top_up_exam_pool` chạy mỗi 1 giờ) đều dừng hoạt động.
* Khi có người dùng truy cập web, chỉ có **Server API thức dậy** (cold start). Nếu Worker không nhận được request, Worker vẫn ngủ hoặc bị trễ lịch sinh câu hỏi, dẫn đến tình trạng học viên vào làm bài thì kho câu hỏi bị trống.

### 2. Giải pháp Fallback đã triển khai trong mã nguồn

#### Bước 1: API đánh thức Worker khi khởi động

Tại [cmd/api/main.go:L374-L454](../../cmd/api/main.go#L374-L454):

* Khởi tạo `workerNudger := newWorkerNudger(cfg.Worker.URL)`.
* Ngay sau khi server API bật port lắng nghe (`server.ListenAndServe()`), API gửi ngay một request `Nudge` ngầm tới Worker:
  `GET {WORKER_URL}/ready`
* Thao tác này giúp Worker ngay lập tức nhận được HTTP traffic và thức giấc cùng lúc với API.

#### Bước 2: Bổ sung khả năng kiểm tra và kích hoạt job quá hạn (`TriggerDue`)

Tại [internal/platform/job/cron.go](../../internal/platform/job/cron.go):

* Bổ sung trường `lastRun map[string]time.Time` vào `CronScheduler` để theo dõi thời điểm chạy gần nhất của từng job.
* Hiện thực phương thức `TriggerDue(ctx context.Context)`:
  1. Quét danh sách tất cả các cron job đã đăng ký (`learning.top_up_practice_pool`, `learning.top_up_exam_pool`, `learning.top_up_placement_pool`...).
  2. Kiểm tra nếu job chưa từng chạy (`!ran`) hoặc thời gian trôi qua từ lần chạy cuối đã vượt quá chu kỳ (`time.Since(lastRun) >= job.Interval`).
  3. Đánh dấu trước thời gian `lastRun = now` (in-memory debouncing) để ngăn chặn các lượt ping dồn dập kích hoạt trùng lặp.
  4. Khởi chạy job ngầm qua `executeWithLock(ctx, j)`.
* Được bảo vệ an toàn bởi PostgreSQL advisory lock (`pg_try_advisory_lock`), đảm bảo dù có nhiều replica hoặc gọi nhiều lần cũng chỉ có 1 instance được quyền chạy.

#### Bước 3: Worker kích hoạt fallback khi nhận ping `/ready`

Tại [cmd/worker/main.go:L442-L447](../../cmd/worker/main.go#L442-L447):

```go
mux.HandleFunc("/ready", func(w http.ResponseWriter, r *http.Request) {
    health.Ready(w, r)
    // Trigger overdue cron jobs in background detached from request context
    // so pinging /ready (e.g. from API wakeup on cold start) catches up pool generation.
    go cron.TriggerDue(context.WithoutCancel(ctx))
})
```

* Trả lời ngay lập tức mã `200 OK` cho bộ kiểm tra readiness của Render (không gây timeout health check).
* Đồng thời chạy ngầm `cron.TriggerDue()` với context tách biệt (`context.WithoutCancel`), tự động kiểm tra và bù đắp các câu hỏi định kỳ nếu kho câu hỏi đang thiếu.

---

## IV. Tổng Kết Các File Đã Chỉnh Sửa

| File | Nội dung thay đổi |
|---|---|
| [internal/platform/job/cron.go](../../internal/platform/job/cron.go) | Thêm quản lý `lastRun`, hàm `TriggerDue()` và `LastRun()` kiểm tra job quá hạn do server sleep. |
| [internal/platform/job/cron_trigger_test.go](../../internal/platform/job/cron_trigger_test.go) | Unit test kiểm tra `TriggerDue` và cơ chế debouncing chống gọi lặp. |
| [cmd/worker/main.go](../../cmd/worker/main.go) | Hook `cron.TriggerDue()` vào endpoint `/ready` để tự động bù job khi nhận ping từ API. |
| [cmd/api/main.go](../../cmd/api/main.go) | Kích hoạt `workerNudger.Nudge(ctx)` ngay khi API khởi động để đánh thức Worker. |
