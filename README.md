# 垃圾車 LINE Bot

一個整合台灣垃圾車即時資訊的 LINE Bot，使用者可透過文字或定位查詢最近垃圾車的抵達時間、路線與地點，並可設定提醒通知。

## 功能特色

- 🗑️ **即時查詢垃圾車** - 輸入地址或分享位置即可查詢附近垃圾車站點
- ⏰ **智慧提醒系統** - 可設定垃圾車抵達前 N 分鐘提醒，自動推播通知
- ❤️ **收藏地點** - 儲存常用地點（家、公司）
- 🤖 **自然語言查詢** - 支援「我晚上七點前在哪裡倒垃圾？」等自然語言
- 🗺️ **地圖導航** - 提供 Google Maps 導航連結

## 技術架構

- **語言**: Go 1.24
- **雲端平台**: Google Cloud Platform
- **資料庫**: Firestore
- **外部 API**: LINE Bot SDK, Google Maps API, Gemini API
- **資料來源**: [Yukaii/garbage](https://github.com/Yukaii/garbage)

## 環境變數設定

```bash
# 必要環境變數
PORT=8080
LINE_CHANNEL_SECRET=your_line_channel_secret
LINE_CHANNEL_ACCESS_TOKEN=your_line_channel_access_token
GOOGLE_MAPS_API_KEY=your_google_maps_api_key
GEMINI_API_KEY=your_gemini_api_key
GEMINI_MODEL=gemini-1.5-pro
GCP_PROJECT_ID=your_gcp_project_id

# 可選環境變數（如不提供將自動生成）
# INTERNAL_TASK_TOKEN=your_custom_token
```

### 🔑 Google Maps API Key 設定指南

Google Maps API 是本專案的核心依賴，用於地址轉換和地理編碼。請確保完成以下設定步驟：

#### 1. 啟用必要的 API

前往 [GCP Console - API Library](https://console.cloud.google.com/apis/library)，啟用以下 API：

- ✅ **Geocoding API** - 地址轉坐標（必需）
- ✅ **Maps JavaScript API** - 地圖顯示
- ✅ **Places API** - 地點搜索
- ✅ **Geolocation API** - 定位服務

**快速啟用命令**：
```bash
gcloud services enable \
  geocoding-backend.googleapis.com \
  maps-backend.googleapis.com \
  places-backend.googleapis.com \
  geolocation.googleapis.com \
  --project=your-project-id
```

#### 2. 建立 API Key

1. 前往 [APIs & Services → Credentials](https://console.cloud.google.com/apis/credentials)
2. 點擊 **"CREATE CREDENTIALS"** → **"API key"**
3. 複製產生的 API key

#### 3. 設定 API Key 限制（重要！）

為了安全性，請限制 API Key 的使用範圍：

**API 限制**：
- 選擇 **"Restrict key"**
- 勾選：Geocoding API、Places API、Maps JavaScript API、Geolocation API

**應用程式限制**（建議）：
- **本地開發**：選擇 "None"
- **生產環境**：選擇 "IP addresses" 並設定 Cloud Run 的出站 IP

#### 4. 驗證設定

等待 1-2 分鐘讓 API key 生效，然後測試：

```bash
curl "https://maps.googleapis.com/maps/api/geocode/json?address=台北101&key=你的API_KEY"
```

成功回應應包含 `"status": "OK"`

#### 5. 費用說明

- 免費額度：每月 $200（約 40,000 次 Geocoding 請求）
- 建議設定 [預算提醒](https://console.cloud.google.com/billing/budgets) 避免超支

## 快速開始

1. **克隆專案**
   ```bash
   git clone <repository-url>
   cd linebot-garbage-helper
   ```

2. **設定環境變數**
   ```bash
   cp .env.example .env
   # 編輯 .env 文件，填入必要的 API 金鑰
   ```

   ⚠️ **重要**：請確保已按照上方 [Google Maps API Key 設定指南](#-google-maps-api-key-設定指南) 完成以下步驟：
   - 在 GCP Console 啟用 Geocoding API 等必要服務
   - 建立並配置 API Key
   - 將 API Key 填入 `.env` 文件的 `GOOGLE_MAPS_API_KEY` 欄位

3. **安裝依賴**
   ```bash
   go mod tidy
   ```

4. **本地運行**
   ```bash
   go run cmd/server/main.go
   ```

## Docker 部署

1. **建構映像**
   ```bash
   docker build -t garbage-linebot .
   ```

2. **運行容器**
   ```bash
   docker run -p 8080:8080 --env-file .env garbage-linebot
   ```

## Cloud Build 自動部署 (推薦)

### 設定 Cloud Build 觸發器

1. **連接 GitHub Repository**
   - 前往 [Cloud Build Console](https://console.cloud.google.com/cloud-build/triggers)
   - 點擊「建立觸發器」
   - 連接你的 GitHub repository

2. **設定觸發器**
   - 名稱: `garbage-linebot-deploy`
   - 事件: `推送至分支`
   - 分支: `^main$`
   - 設定檔: `/cloudbuild.yaml`

3. **環境變數設定**
   在觸發器的「替代變數」中設定：
   ```
   _LINE_CHANNEL_SECRET: your_line_channel_secret
   _LINE_CHANNEL_ACCESS_TOKEN: your_line_channel_access_token
   _GOOGLE_MAPS_API_KEY: your_google_maps_api_key
   _GEMINI_API_KEY: your_gemini_api_key
   ```

   ⚡ **注意**：
   - `INTERNAL_TASK_TOKEN` 現在會自動生成，無需手動設定
   - ⚠️ `_GOOGLE_MAPS_API_KEY` 請確保已按照上方 [Google Maps API Key 設定指南](#-google-maps-api-key-設定指南) 完成 API 啟用步驟

4. **推送程式碼自動部署**
   ```bash
   git push origin main
   ```

### 手動 GCP Cloud Run 部署

1. **啟用必要的 API**
   ```bash
   # 啟用 Cloud Run 和資料庫服務
   gcloud services enable run.googleapis.com
   gcloud services enable firestore.googleapis.com
   gcloud services enable cloudscheduler.googleapis.com

   # ⚠️ 重要：啟用 Google Maps API（必需）
   gcloud services enable geocoding-backend.googleapis.com
   gcloud services enable maps-backend.googleapis.com
   gcloud services enable places-backend.googleapis.com
   gcloud services enable geolocation.googleapis.com
   ```

   詳細的 Google Maps API 配置請參考上方 [Google Maps API Key 設定指南](#-google-maps-api-key-設定指南)

2. **部署到 Cloud Run**
   ```bash
   gcloud run deploy garbage-linebot \
     --source . \
     --platform managed \
     --region asia-east1 \
     --allow-unauthenticated \
     --set-env-vars "LINE_CHANNEL_SECRET=xxx,LINE_CHANNEL_ACCESS_TOKEN=xxx,..."
   ```

3. **設定 Cloud Scheduler**
   應用程式部署後會自動設定 Cloud Scheduler。如需手動設定：
   
   ### 自動部署設定（推薦）
   透過 Cloud Build 觸發器部署會自動建立 Cloud Scheduler。
   
   ### 手動設定（適用於 Cloud Run 直接連接 GitHub 部署）
   ```bash
   # 1. 啟用必要的 API
   gcloud services enable cloudscheduler.googleapis.com
   
   # 2. 取得服務 URL
   SERVICE_URL=$(gcloud run services describe garbage-linebot --region=asia-east1 --format='value(status.url)')
   
   # 3. 取得內部 API token
   TOKEN=$(curl -s "${SERVICE_URL}/internal/token" | grep -o '"token":"[^"]*"' | cut -d'"' -f4)
   
   # 4. 建立 Cloud Scheduler 工作
   gcloud scheduler jobs create http reminder-dispatcher \
     --location=asia-east1 \
     --schedule="*/5 * * * *" \
     --uri="${SERVICE_URL}/tasks/dispatch-reminders" \
     --http-method=POST \
     --headers="Authorization=Bearer $TOKEN" \
     --description="Garbage truck reminder dispatcher"
   
   # 5. 驗證設定
   gcloud scheduler jobs list --location=asia-east1
   
   # 6. 測試執行
   gcloud scheduler jobs run reminder-dispatcher --location=asia-east1
   ```
   
   ### ⚠️ 重要注意事項
   - **區域一致性**：確保 Cloud Scheduler 和 Cloud Run 在同一區域 (`asia-east1`)
   - **Token 有效性**：應用程式重新部署時，token 可能會改變，需要重新取得並更新 scheduler
   - **權限檢查**：確認 GCP 帳戶有 Cloud Scheduler 的建立權限
   - **雙重保障**：本地排程器會自動運作，Cloud Scheduler 提供額外可靠性保障

詳細部署說明請參考 [DEPLOYMENT.md](./DEPLOYMENT.md)

## API 端點

| Method | Path | 說明 |
|--------|------|------|
| POST | `/line/callback` | LINE webhook 接收端點 |
| POST | `/tasks/dispatch-reminders` | 提醒推播任務 |
| GET | `/healthz` | 健康檢查 |
| GET | `/internal/token` | 取得內部 API token |
| POST | `/internal/refresh-routes` | 更新垃圾車路線資料 |

## LINE Bot 功能

### 🗑️ 垃圾車查詢方式
- **📍 分享位置**：點擊「+」→「位置」→「即時位置」或「傳送位置」
- **💬 輸入地址**：直接輸入地址，例如「台北市信義區忠孝東路」
- **🕐 時間查詢**：自然語言查詢，例如「我晚上七點前在哪裡倒垃圾？」

### 📋 指令列表
- `/help` - 查看幫助資訊
- `/favorite [名稱] [地址]` - 收藏地點
- `/list` - 查看收藏清單
- `你好` / `hello` - 歡迎訊息和快速開始指南

## 📅 提醒排程系統

### 核心功能
- **自動排程檢查**: 每分鐘掃描一次活躍提醒，檢查是否需要發送通知
- **智慧通知時機**: 根據設定的提前分鐘數，在垃圾車抵達前精準推播
- **狀態管理**: 提醒狀態包括 `active`（活躍）、`sent`（已發送）、`expired`（已過期）、`cancelled`（已取消）
- **自動清理**: 每小時清理過期提醒（超過 24 小時的舊提醒）

### 運作機制
1. **本地排程器**: 應用啟動時自動開始背景排程服務
2. **外部觸發**: 支援透過 Cloud Scheduler 調用 `/tasks/dispatch-reminders` 端點
3. **雙重保障**: 內建排程器與外部排程器同時運作，確保提醒不遺漏
4. **效能優化**: 使用 Firestore count 查詢避免不必要的資料讀取

### 提醒資料結構
```go
type Reminder struct {
    ID             string    // 提醒 ID
    UserID         string    // 用戶 LINE ID
    StopName       string    // 垃圾車站點名稱
    RouteID        string    // 路線 ID
    ETA            time.Time // 預計抵達時間
    AdvanceMinutes int       // 提前幾分鐘提醒
    Status         string    // 提醒狀態
    CreatedAt      time.Time // 建立時間
    UpdatedAt      time.Time // 更新時間
}
```

## 專案結構

```
├── cmd/server/           # 主程式進入點
├── internal/
│   ├── config/          # 配置管理
│   ├── store/           # Firestore 資料存取
│   ├── line/            # LINE Bot 處理器
│   ├── geo/             # 地理編碼服務
│   ├── garbage/         # 垃圾車資料適配器
│   ├── gemini/          # Gemini NLU 服務
│   └── reminder/        # 提醒排程服務
├── Dockerfile
└── README.md
```

## 授權

MIT License