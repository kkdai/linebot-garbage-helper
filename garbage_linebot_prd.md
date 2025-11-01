# 垃圾車 LINE Bot PRD（Golang + GCP + Gemini + LINE）

## 一、專案目標
建構一個整合台灣「垃圾車即時資訊」的 LINE Bot，使用者可透過文字或定位查詢最近垃圾車的抵達時間、路線與地點，並可設定提醒通知。

---

## 二、主要功能 ✅
1. **即時查詢垃圾車** ✅
   - 使用者可輸入地址或分享 LINE 定位點。 ✅
   - 系統查詢最近的垃圾車站點與預估抵達時間。 ✅
   - 使用 [Yukaii/garbage](https://github.com/Yukaii/garbage) 專案提供的資料源。 ✅
2. **提醒通知** ✅
   - 使用者可設定「提前 X 分鐘提醒」。 ✅
   - Cloud Scheduler 透過 Pub/Sub 觸發 Cloud Run 任務推播通知。 ✅
3. **收藏地點** ✅
   - 使用者可將常用地點（家、公司）收藏。 ✅
4. **自然語言查詢** ✅
   - 使用 Gemini API 理解自然語言，例如：「我晚上七點前在哪裡倒垃圾？」。 ✅
5. **地圖與導航** ✅
   - 使用 Google Maps API 反查地址與產生導航連結。 ✅

---

## 三、技術架構

### Cloud 架構 ✅
- **Cloud Run**：主應用（LINE webhook + API） ✅
- **Firestore**：使用者、提醒、地點資料儲存 ✅
- **Cloud Scheduler + Pub/Sub**：推播提醒任務 ✅
- **GCS（可選）**：暫存地圖快照
- **外部 API**： ✅
  - Google Maps Geocoding / Places ✅
  - Gemini 1.5 Pro ✅
  - [Yukaii/garbage](https://github.com/Yukaii/garbage) JSON 資料 ✅

### 開發語言 ✅
- Golang（使用 [LINE Bot SDK for Go](https://github.com/line/line-bot-sdk-go)）

### 部署 ✅
- Dockerfile + GCP Cloud Run 部署  
- 所有機密以環境變數設定

---

## 四、資料流

### 1️⃣ 查詢流程
1. LINE → webhook (`/line/callback`)
2. **位置處理**：
   - 若傳送位置：直接解析 lat/lng，若無地址則使用 Google Reverse Geocoding 取得地址
   - 若傳送文字：透過 Gemini 辨識地址關鍵字，再使用 Google Geocoding 轉換座標
3. 發送確認訊息給使用者（顯示收到的位置/地址）
4. 查詢 Garbage API（Yukaii 開源資料）找最近站點與 ETA
5. 回覆 LINE Flex Message（站點、時間、距離、導航連結）

### 2️⃣ 提醒流程
1. 使用者設定提醒 → 儲存 Firestore。
2. Cloud Scheduler 每分鐘觸發 `/tasks/dispatch-reminders`。
3. 找出 ETA - advanceMinutes ≈ now 的提醒 → 發 LINE push。

---

## 五、外部資料源（Yukaii Garbage）

### 來源
- GitHub: [https://github.com/Yukaii/garbage](https://github.com/Yukaii/garbage)
- 提供全台垃圾車路線、站點與時刻資料（JSON）。

### 使用方式
- 直接 fetch `garbage.json`，快取於 Firestore 或 GCS。  
- 更新頻率建議每日或每週。

### 範例 JSON 結構
```json
{
  "routes": [
    {
      "id": "taichung_001",
      "name": "台中市中區路線1",
      "stops": [
        {
          "name": "民族路口",
          "lat": 24.1403,
          "lng": 120.6815,
          "time": "19:35"
        }
      ]
    }
  ]
}
```

---

## 六、API 介面 ✅

| Method | Path | Description |
|--------|------|-------------|
| POST | `/line/callback` | 接收 LINE webhook ✅ |
| POST | `/tasks/dispatch-reminders` | 定期提醒推播 ✅ |
| GET  | `/healthz` | 健康檢查 ✅ |
| POST | `/internal/refresh-routes` | 更新 Garbage 路線資料 ✅ |
| GET  | `/internal/token` | 取得自動生成的內部 API token ✅ |

---

## 七、環境變數設定

```bash
PORT=8080
LINE_CHANNEL_SECRET=xxxx
LINE_CHANNEL_ACCESS_TOKEN=xxxx
GOOGLE_MAPS_API_KEY=xxxx
GEMINI_API_KEY=xxxx
GEMINI_MODEL=gemini-1.5-pro
GCP_PROJECT_ID=your-project-id
# INTERNAL_TASK_TOKEN=randomstring  # 已改為自動生成，無需手動設定
```

---

## 八、Firestore 結構

```
users/{userId}
  favorites: [ { name, lat, lng, address } ]

reminders/{reminderId}
  userId
  stopName
  routeId
  eta
  advanceMinutes
  status

routes/{routeId}
  data: JSON (from Yukaii/garbage)
  updatedAt
```

---

## 九、Golang 專案結構 ✅

```
/cmd/server/main.go ✅
/internal/line/handler.go ✅
/internal/geo/geocode.go ✅
/internal/garbage/adapter.go ✅
/internal/gemini/nlu.go ✅
/internal/reminder/scheduler.go ✅
/internal/store/firestore.go ✅
/internal/config/config.go ✅
```

---

## 十、Dockerfile 範例 ✅

```dockerfile
FROM golang:1.24-alpine AS builder

RUN apk add --no-cache ca-certificates

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-w -s" -o server ./cmd/server

FROM gcr.io/distroless/base-debian12

WORKDIR /app

ENV PORT=8080

COPY --from=builder /app/server .

USER nonroot:nonroot

EXPOSE 8080

ENTRYPOINT ["/app/server"]
```

---

## 十一、Gemini NLU Prompt 範例

```text
你是一個查詢意圖分析器，輸入可能包含地名與時間。請輸出：
{ "district": "...", "time_window": { "from": "...", "to": "..." }, "keywords": ["..."], "query_type": "garbage_truck_eta" }
```

---

## 十二、Flex Message 回覆範例

```json
{
  "type": "bubble",
  "body": {
    "type": "box",
    "layout": "vertical",
    "contents": [
      { "type": "text", "text": "長安國小站", "weight": "bold", "size": "lg" },
      { "type": "text", "text": "下一班：19:35", "size": "md" },
      { "type": "text", "text": "距離：約400公尺", "size": "sm", "color": "#888888" }
    ]
  },
  "footer": {
    "type": "box",
    "layout": "horizontal",
    "contents": [
      { "type": "button", "action": { "type": "uri", "label": "導航", "uri": "https://maps.google.com/?q=25.0523,121.5334" } },
      { "type": "button", "action": { "type": "postback", "label": "提醒我", "data": "route=R001&stop=S001" } }
    ]
  }
}
```

---

## 十三、後續擴充想法
- 社區模式：群組共享垃圾車狀況
- 智慧推播：依定位自動提醒
- 天氣整合：下雨天提前通知「請提早出門」
