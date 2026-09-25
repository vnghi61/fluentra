# WO 22 — chạy local để bổ sung nội dung

Hướng dẫn này dành cho người chạy trên máy local, có AI key và mạng internet công cộng, để bổ sung
phần nội dung WO 22 còn thiếu. Code đã xong và đã test ở nhánh `claude/determined-johnson-s2e5r3`.
Phần còn lại là sinh nội dung (cần AI) và tra phát âm, tìm ảnh (cần mạng).

Tình trạng hiện tại và lý do: xem §7 của [phase-3-work-order-22.md](phase-3-work-order-22.md).

| Việc | Hiện có | Mục tiêu | Cần |
|---|---|---|---|
| Từ vựng | 6.056 từ | 10.000 từ | AI |
| Phát âm từ (IPA chuẩn, audio có ghi nguồn) | chỉ IPA của CMU | ≥ 80% từ có audio | Mạng |
| Foundation | 27/93 node | 93/93 node | AI |
| Ảnh TOEIC Part 1 | 0 | ≥ 7 ảnh mỗi đề (35 cho 5 đề) | Người chọn ảnh và viết mô tả, mạng |
| Đề thi | 0 | 5 đề mỗi kỳ thi: TOEIC, IELTS, VSTEP | AI, cần ảnh Part 1 cho TOEIC |
| Ký duyệt bảng format đề | chưa | đã ký | Người |

Thứ tự nên chạy: **0 → 1 → 2 → 3 → 4 → 5 → 6**. Bước 1 và bước 2 độc lập, chạy song song được.

---

## 0. Chuẩn bị (một lần)

### 0.1 Code và công cụ

```bash
git fetch origin claude/determined-johnson-s2e5r3
git checkout claude/determined-johnson-s2e5r3
```

Cần có: Go (bản trong `go.mod`), Docker (để có PostgreSQL), và Python 3 nếu chạy bước 1.3.

### 0.2 Mạng

Máy phải truy cập được các host sau:

- `api.dictionaryapi.dev`: tra IPA và audio (bước 1.4).
- `commons.wikimedia.org` và `upload.wikimedia.org`: audio phát âm, ảnh Part 1.
- `api.openverse.org`: tìm ảnh Part 1 (không bắt buộc).
- Endpoint của nhà cung cấp AI bạn dùng.

### 0.3 File `.env`

Các lệnh đọc `.env` ở thư mục gốc repo.

```bash
cp .env.example .env
```

Sửa các dòng sau trong `.env`:

```dotenv
DB_DSN=postgres://fluentra:fluentra@localhost:5432/fluentra?sslmode=disable

# Slot 1: model viết nội dung
AI_PROVIDER_1_NAME=groq
AI_PROVIDER_1_BASE_URL=https://api.groq.com/openai/v1
AI_PROVIDER_1_MODEL=<model A>
AI_PROVIDER_1_API_KEY=<key>

# Slot 2: model KHÁC để làm verifier độc lập (bắt buộc nếu muốn tự publish)
AI_PROVIDER_2_NAME=cerebras
AI_PROVIDER_2_BASE_URL=https://api.cerebras.ai/v1
AI_PROVIDER_2_MODEL=<model B, khác model A>
AI_PROVIDER_2_API_KEY=<key>

# Cho verifier publish những gì nó xác nhận; phần nó nghi ngờ vào hàng chờ duyệt
AI_AUTO_PUBLISH=true
```

> **Quan trọng:** nếu chỉ có một model, hoặc hai slot dùng cùng một model, thì không có verifier độc lập.
> Khi đó **mọi câu đều nằm chờ người duyệt**, không có gì được tự publish. Tên slot (`NAME`) phải khác nhau.
> Bất kỳ server nào tương thích OpenAI đều dùng được (Groq, Cerebras, Mistral, OpenRouter, Ollama…).

### 0.4 Database

```bash
make dev-infra     # postgres, redis, minio, mailpit
make migrate-up
make seed          # 2 tài khoản, từ vựng, 13 khoá học, nội dung Foundation đang có
```

`make seed` tạo tài khoản admin `nguyenvannghi1110@gmail.com`. Các lệnh sinh nội dung ghi nội dung dưới tên
tài khoản này, và bạn dùng nó để duyệt ở `/admin/review`. Nếu muốn làm lại từ đầu:

```bash
psql "$DB_DSN" -v i_understand=yes -f scripts/reset-dev-database.sql   # XOÁ HẾT dữ liệu app
make migrate-up && make seed
```

---

## 1. Từ vựng: 6.056 → 10.000 từ

Kết quả là fixture trong `db/fixtures/vocabulary/`. Bước này không ghi nội dung vào DB; `make seed` sẽ nạp fixture sau.

### 1.1 Sinh thêm từ (AI)

```bash
go run ./cmd/vocabgen -source db/fixtures/vocabulary/source/wordfreq-lemmas.tsv
```

- Lệnh đi dần xuống danh sách tần suất. Từ nào model bỏ qua (tên riêng, từ không chuẩn…) thì được thay
  bằng từ kế tiếp, cho đến khi đủ `-limit` (mặc định 10000).
- Từ đã có trong fixture hoặc trong `skipped.json` không được hỏi lại.
- Nghĩa đã sinh được lưu trong `vocabgen-cache.json` (đã gitignore). **Nếu bị ngắt, chạy lại đúng lệnh
  trên, nó sẽ chạy tiếp từ chỗ dừng.**
- Muốn xem trước mà không ghi gì: thêm `-dry-run`. Nếu model hay trả sai JSON, giảm `-batch` (mặc định 15).

### 1.2 Kiểm tra

```bash
python3 - <<'EOF'
import json, glob
n = sum(len(json.load(open(f))["words"]) for f in glob.glob("db/fixtures/vocabulary/words-*.json"))
print("words:", n)
EOF
```

Kết quả cần là `words: 10000`, hoặc sát 10.000.

### 1.3 IPA cho từ mới (offline, không bắt buộc nhưng nên làm)

Bước này thêm IPA từ từ điển CMU cho từ chưa có, và loại thêm từ rác mà model bỏ sót.

```bash
python3 -m venv .venv && . .venv/bin/activate
pip install wordfreq==3.1.1 lemminflect==0.2.3 eng-to-ipa==0.0.2
python scripts/vocab-source-list.py clean
go run ./cmd/vocabgen -canonicalise
```

### 1.4 Phát âm: IPA chuẩn và audio có ghi nguồn (MẠNG)

```bash
go run ./cmd/vocabgen -pronounce
```

- Với mỗi từ chưa có audio, lệnh tra `api.dictionaryapi.dev`. Nó chỉ nhận audio có ghi nguồn và giấy phép
  CC0, CC BY hoặc CC BY-SA.
- Mỗi lần gọi cách nhau 250 ms, nên 10.000 từ mất khoảng **1 giờ**. Chạy lại được bất cứ lúc nào;
  từ đã có audio sẽ được bỏ qua.
- Cần đạt: **≥ 80% từ có audio** (gate của plan). Kiểm tra:

```bash
python3 - <<'EOF'
import json, glob
w = [x for f in glob.glob("db/fixtures/vocabulary/words-*.json") for x in json.load(open(f))["words"]]
print(f"{len(w)} words, audio {sum(1 for x in w if x.get('audio_url'))/len(w):.0%}, "
      f"ipa {sum(1 for x in w if x.get('ipa'))/len(w):.0%}")
EOF
```

### 1.5 Commit

```bash
git add db/fixtures/vocabulary && git commit -m "feat(vocabulary): wo22 — 10,000 words with pronunciation"
```

---

## 2. Foundation: 27 → 93 node (AI)

```bash
go run ./cmd/foundation -missing
```

- Lệnh chỉ chạy các node chưa có topic đã publish. Chạy lại cho đến khi báo không còn node nào.
- Nếu một node cứ fail mãi: `go run ./cmd/foundation -missing -after <MÃ_NODE>` để đi tiếp qua node đó.
  Ghi lại mã node để xử lý sau.
- Chạy thử một node: `go run ./cmd/foundation -node PRESENT_PERFECT`.
- Node chỉ được publish khi cả topic, quiz và review đều đạt (D22-13). Phần verifier nghi ngờ sẽ nằm
  trong hàng chờ.

**Duyệt phần bị nghi ngờ:** đăng nhập bằng tài khoản admin, mở `/admin/review`, rồi mở từng batch.
Duyệt hoặc sửa từng câu. Sau khi duyệt xong, chạy lại `-missing` để publish các node đã đủ.

**Xuất ra fixture và commit:**

```bash
go run ./cmd/foundation -export        # ghi vào db/fixtures/foundation/
git add db/fixtures/foundation && git commit -m "feat(foundation): wo22 — content for every spine node"
```

Kiểm tra số node có nội dung:

```bash
python3 -c "import json,glob; f=glob.glob('db/fixtures/foundation/*.json'); \
print(sum(1 for p in f if json.load(open(p)).get('items')), '/', len(f))"
```

---

## 3. Ảnh TOEIC Part 1 (NGƯỜI và MẠNG)

Model không nhìn thấy ảnh. Nó viết 4 câu mô tả **chỉ dựa trên phần mô tả do bạn viết**. Vì vậy mô tả phải đúng
và đủ chi tiết.

1. Tìm ảnh trên [Wikimedia Commons](https://commons.wikimedia.org) hoặc [Openverse](https://openverse.org).
   **Chỉ lấy ảnh CC0 hoặc CC BY.** Không lấy ảnh BY-SA, NC hay ND; file có ảnh như vậy sẽ bị từ chối
   lúc nạp.
2. Chọn cảnh kiểu TOEIC: người đang làm việc gì đó, văn phòng, nhà ga, cửa hàng, công trường… Ảnh phải
   rõ ràng, không có chữ lớn, không có người nổi tiếng.
3. Thêm vào `db/fixtures/exams/toeic-part1-photos.json`:

```json
{
  "format": "fluentra.exam.part1photos.fixture.v1",
  "source": "Wikimedia Commons and Openverse, photographs by link only",
  "checked_at": "2026-09-25",
  "licence_note": "…giữ nguyên…",
  "photos": [
    {
      "id": "office-printer-01",
      "url": "https://upload.wikimedia.org/wikipedia/commons/x/xx/Example.jpg",
      "credit_page": "https://commons.wikimedia.org/wiki/File:Example.jpg",
      "licence": "CC BY 4.0",
      "description": "A woman in a grey jacket is standing at a photocopier, lifting the lid with one hand. A stack of paper sits on the table beside her. Nobody else is in the room."
    }
  ]
}
```

- `url` là link trực tiếp tới file ảnh (`upload.wikimedia.org/...`). `credit_page` là trang mô tả file.
- `description` viết bằng tiếng Anh, 2–4 câu. Nêu: ai, đang làm gì, ở đâu, có vật gì, và điều gì **không**
  có trong ảnh.
- `id` không được trùng.
- Cần ít nhất **7 ảnh cho mỗi đề** (6 câu, cộng 1 dự phòng), tức **≥ 35 ảnh cho 5 đề**. Nên chuẩn bị dư
  khoảng 20%.
- Mỗi ảnh chỉ dùng một lần. Ảnh đã nằm trong một câu nào đó (kể cả câu đang chờ duyệt) sẽ không bị lấy
  lại. Ảnh của câu bị loại hẳn thì lần chạy sau vẫn được dùng.

Kiểm tra file hợp lệ. `-dry-run` đọc file ảnh trước, và báo lỗi nếu có ảnh thiếu trường, sai giấy phép
hoặc trùng `id`:

```bash
go run ./cmd/examgen -exam TOEIC_LR_2026 -dry-run
```

Commit: `git add db/fixtures/exams/toeic-part1-photos.json && git commit -m "feat(exams): toeic part 1 photographs"`

---

## 4. Đề thi: 5 đề cho mỗi kỳ thi (AI)

Làm bước 3 trước khi chạy TOEIC. Nếu không có ảnh, Part 1 bị bỏ qua và TOEIC không ghép được đề nào.

```bash
go run ./cmd/examgen -exam TOEIC_LR_2026 -tests 5
go run ./cmd/examgen -exam IELTS_ACADEMIC_2026_R2 -tests 5
go run ./cmd/examgen -exam VSTEP_3_5 -tests 5
```

- Mỗi vòng sinh ra lượng câu đủ cho một đề, cộng thêm phần dự phòng. Lệnh dừng khi đủ số đề, hoặc sau
  tối đa (số đề còn thiếu + 3) vòng.
- Nếu lệnh báo `has N of 5 fixed test(s)`, nghĩa là phần còn thiếu đang nằm chờ duyệt. Vào `/admin/review`
  duyệt, rồi chạy lại đúng lệnh đó.
- IELTS Writing Task 1 tự có biểu đồ: model trả số liệu, code vẽ SVG. Câu nào số liệu sai sẽ bị loại và
  sinh lại.
- Xem trước mà không sinh gì: thêm `-dry-run`.

**Audio cho phần nghe:** `examgen` không tạo audio. Để render bằng Piper, cần đặt `SPEECH_PIPER_BINARY` và
`SPEECH_PIPER_MODELS` trong `.env`. `SPEECH_TTS_VOICE_B` là giọng thứ hai cho hội thoại; nếu để trống thì
dùng một giọng. Sau đó chạy:

```bash
make tts
```

**Xuất đề ra fixture và commit:**

```bash
go run ./cmd/examgen -exam TOEIC_LR_2026 -export
go run ./cmd/examgen -exam IELTS_ACADEMIC_2026_R2 -export
go run ./cmd/examgen -exam VSTEP_3_5 -export
git add db/fixtures/exams && git commit -m "feat(exams): wo22 — five fixed tests per exam"
```

Kiểm tra số đề trong DB:

```bash
psql "$DB_DSN" -c "SELECT v.code, count(*) FROM assess.mock_tests m
  JOIN assess.blueprints b ON b.id = m.blueprint_id
  JOIN assess.exam_versions v ON v.id = b.version_id
  WHERE m.mode = 'fixed' GROUP BY v.code;"
```

---

## 5. Ký duyệt bảng format đề thi (NGƯỜI)

Bảng nằm ở `db/migrations/exam/1700000943_exam_format_spec.sql`. Mỗi phần thi có `source` ghi nguồn.
Đối chiếu từng dòng (số câu, số lựa chọn, dạng câu, giới hạn từ, thời gian, số lần nghe) với nguồn chính thức:

- **TOEIC:** ETS, TOEIC Listening & Reading examinee handbook.
- **IELTS Academic:** trang Test format trên ielts.org.
- **VSTEP.3-5:** Quyết định 729/QĐ-BGDĐT (2015). Kiểm tra lại xem đây có còn là văn bản hiện hành không.

Nếu có dòng sai, sửa bằng một **migration mới**, không sửa migration cũ. Sau đó điền tên và ngày vào bảng
ký duyệt ở cuối §7 của `phase-3-work-order-22.md`.

---

## 6. Kiểm tra cuối (gate của plan, §5)

Chạy trên DB sạch để chắc chắn mọi thứ đi từ fixture, không phụ thuộc DB cũ:

```bash
psql "$DB_DSN" -v i_understand=yes -f scripts/reset-dev-database.sql
make migrate-up && make seed
```

Cần thấy:

- [ ] 10.000 từ, ≥ 80% có audio; deck A1–C1 và "Top 1,000" có từ.
- [ ] 13 khoá học, bài nào cũng có nội dung.
- [ ] TOEIC, IELTS, VSTEP: mỗi kỳ thi có 5 đề cố định.
- [ ] Trên web (`make api`, `make worker`, `make web`), thử các luồng:
  - TOEIC → Đề 1 → Thi thử → nộp → xem báo cáo. Part 1 có ảnh và dòng ghi nguồn ảnh.
  - IELTS → Writing Task 1 có biểu đồ.
  - Một bài Foundation, từ phần giải thích đến quiz.
- [ ] `make check` pass (cần có `goimports-reviser` và `golangci-lint` bản build bằng Go 1.26).

Push lên nhánh:

```bash
git push -u origin claude/determined-johnson-s2e5r3
```

---

## Sự cố thường gặp

| Triệu chứng | Nguyên nhân và cách xử lý |
|---|---|
| `no AI provider is configured` | `.env` vẫn để `AI_PROVIDER_1_NAME=mock`, hoặc thiếu key |
| Không có gì được publish, mọi thứ nằm ở `/admin/review` | Chỉ có một model, hoặc hai slot trùng model, hoặc `AI_AUTO_PUBLISH=false` |
| `find an admin to attribute the drafts to` | Chưa chạy `make seed` trên DB này |
| TOEIC báo `photo part skipped` | Fixture ảnh rỗng, hoặc ảnh đã dùng hết. Thêm ảnh (bước 3) |
| `-pronounce` rất chậm hoặc toàn lỗi | Không vào được `api.dictionaryapi.dev`; kiểm tra mạng hoặc proxy |
| `examgen` báo đủ vòng mà vẫn thiếu đề | Phần thiếu đang chờ duyệt: duyệt ở `/admin/review` rồi chạy lại |
| Một node Foundation fail mãi | Dùng `-after <MÃ>` để đi tiếp, rồi xử lý node đó bằng `-node <MÃ>` |
