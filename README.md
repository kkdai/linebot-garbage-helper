# 垃圾車 LINE Bot

一個整合台灣垃圾車即時資訊的 LINE Bot，使用者可透過文字或定位查詢最近垃圾車的抵達時間、路線與地點，並可設定提醒通知。

## 功能特色

- 🗑️ **即時查詢垃圾車** - 輸入地址或分享位置即可查詢附近垃圾車站點
- ⏰ **提醒通知** - 可設定垃圾車抵達前提醒
- ❤️ **收藏地點** - 儲存常用地點（家、公司）
- 🤖 **自然語言查詢** - 支援「我晚上七點前在哪裡倒垃圾？」等自然語言
- 🗺️ **地圖導航** - 提供 Google Maps 導航連結

## 技術架構

- **語言**: Go 1.23
- **雲端平台**: Google Cloud Platform
- **資料庫**: Firestore
- **外部 API**: LINE Bot SDK, Google Maps API, Gemini API
- **資料來源**: [Yukaii/garbage](https://github.com/Yukaii/garbage)

## 環境變數設定

```bash
PORT=8080
LINE_CHANNEL_SECRET=your_line_channel_secret
LINE_CHANNEL_ACCESS_TOKEN=your_line_channel_access_token
GOOGLE_MAPS_API_KEY=your_google_maps_api_key
GEMINI_API_KEY=your_gemini_api_key
GEMINI_MODEL=gemini-1.5-pro
GCP_PROJECT_ID=your_gcp_project_id
INTERNAL_TASK_TOKEN=your_random_token
```

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
   _INTERNAL_TASK_TOKEN: your_random_secure_token
   ```

4. **推送程式碼自動部署**
   ```bash
   git push origin main
   ```

### 手動 GCP Cloud Run 部署

1. **啟用必要的 API**
   ```bash
   gcloud services enable run.googleapis.com
   gcloud services enable firestore.googleapis.com
   gcloud services enable cloudscheduler.googleapis.com
   ```

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
   ```bash
   gcloud scheduler jobs create http reminder-dispatcher \
     --schedule="* * * * *" \
     --uri="https://your-service-url/tasks/dispatch-reminders" \
     --http-method=POST \
     --headers="Authorization=Bearer your_internal_task_token"
   ```

詳細部署說明請參考 [DEPLOYMENT.md](./DEPLOYMENT.md)

## API 端點

| Method | Path | 說明 |
|--------|------|------|
| POST | `/line/callback` | LINE webhook 接收端點 |
| POST | `/tasks/dispatch-reminders` | 提醒推播任務 |
| GET | `/healthz` | 健康檢查 |
| POST | `/internal/refresh-routes` | 更新垃圾車路線資料 |

## LINE Bot 指令

- `/help` - 查看幫助資訊
- `/favorite [名稱] [地址]` - 收藏地點
- `/list` - 查看收藏清單
- 直接發送地址或位置 - 查詢附近垃圾車

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