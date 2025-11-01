# Cloud Build 部署設定指南

## 前置準備

### 1. 啟用必要的 GCP API
```bash
gcloud services enable cloudbuild.googleapis.com
gcloud services enable run.googleapis.com
gcloud services enable firestore.googleapis.com
gcloud services enable cloudscheduler.googleapis.com
gcloud services enable containerregistry.googleapis.com
```

### 2. 設定 Cloud Build 服務帳戶權限
```bash
# 取得專案編號
PROJECT_NUMBER=$(gcloud projects describe $PROJECT_ID --format="value(projectNumber)")

# 賦予 Cloud Build 服務帳戶必要權限
gcloud projects add-iam-policy-binding $PROJECT_ID \
    --member="serviceAccount:${PROJECT_NUMBER}@cloudbuild.gserviceaccount.com" \
    --role="roles/run.admin"

gcloud projects add-iam-policy-binding $PROJECT_ID \
    --member="serviceAccount:${PROJECT_NUMBER}@cloudbuild.gserviceaccount.com" \
    --role="roles/cloudscheduler.admin"

gcloud projects add-iam-policy-binding $PROJECT_ID \
    --member="serviceAccount:${PROJECT_NUMBER}@cloudbuild.gserviceaccount.com" \
    --role="roles/iam.serviceAccountUser"
```

## Cloud Build 觸發器設定

### 透過 GCP Console 設定

1. **進入 Cloud Build**
   - 前往 [Cloud Build Console](https://console.cloud.google.com/cloud-build/triggers)
   - 點擊「建立觸發器」

2. **基本設定**
   - 名稱: `garbage-linebot-deploy`
   - 說明: `Deploy Garbage LINE Bot to Cloud Run`
   - 事件: `推送至分支`
   - 來源: 選擇你的 GitHub repository
   - 分支: `^main$` (或你想要的分支)

3. **設定檔**
   - 類型: `Cloud Build 設定檔`
   - 位置: `Repository`
   - Cloud Build 設定檔位置: `/cloudbuild.yaml`

4. **替代變數設定**
   在「替代變數」區塊中設定以下環境變數：
   ```
   _LINE_CHANNEL_SECRET: your_line_channel_secret
   _LINE_CHANNEL_ACCESS_TOKEN: your_line_channel_access_token
   _GOOGLE_MAPS_API_KEY: your_google_maps_api_key
   _GEMINI_API_KEY: your_gemini_api_key
   _INTERNAL_TASK_TOKEN: your_random_secure_token
   ```

### 透過 gcloud CLI 設定

```bash
# 創建觸發器
gcloud builds triggers create github \
    --repo-name=your-repo-name \
    --repo-owner=your-github-username \
    --branch-pattern=main \
    --build-config=cloudbuild.yaml \
    --substitutions="_LINE_CHANNEL_SECRET=your_secret,_LINE_CHANNEL_ACCESS_TOKEN=your_token,_GOOGLE_MAPS_API_KEY=your_key,_GEMINI_API_KEY=your_key,_INTERNAL_TASK_TOKEN=your_token" \
    --name=garbage-linebot-deploy
```

## 環境變數安全管理

建議使用 Google Secret Manager 來管理敏感資訊：

### 1. 啟用 Secret Manager
```bash
gcloud services enable secretmanager.googleapis.com
```

### 2. 創建 Secrets
```bash
# LINE Bot Secrets
echo -n "your_channel_secret" | gcloud secrets create line-channel-secret --data-file=-
echo -n "your_access_token" | gcloud secrets create line-channel-access-token --data-file=-

# Google Maps API Key
echo -n "your_maps_api_key" | gcloud secrets create google-maps-api-key --data-file=-

# Gemini API Key
echo -n "your_gemini_api_key" | gcloud secrets create gemini-api-key --data-file=-

# Internal Task Token
echo -n "your_random_token" | gcloud secrets create internal-task-token --data-file=-
```

### 3. 更新 cloudbuild.yaml 使用 Secret Manager
可以修改 cloudbuild.yaml 來使用 Secret Manager：

```yaml
availableSecrets:
  secretManager:
  - versionName: projects/$PROJECT_ID/secrets/line-channel-secret/versions/latest
    env: 'LINE_CHANNEL_SECRET'
  - versionName: projects/$PROJECT_ID/secrets/line-channel-access-token/versions/latest
    env: 'LINE_CHANNEL_ACCESS_TOKEN'
  - versionName: projects/$PROJECT_ID/secrets/google-maps-api-key/versions/latest
    env: 'GOOGLE_MAPS_API_KEY'
  - versionName: projects/$PROJECT_ID/secrets/gemini-api-key/versions/latest
    env: 'GEMINI_API_KEY'
  - versionName: projects/$PROJECT_ID/secrets/internal-task-token/versions/latest
    env: 'INTERNAL_TASK_TOKEN'
```

## 部署流程

1. **推送程式碼到 GitHub**
   ```bash
   git add .
   git commit -m "feat: initial implementation"
   git push origin main
   ```

2. **觸發自動部署**
   - 程式碼推送到 main 分支會自動觸發 Cloud Build
   - 建構過程會：
     - 建置 Docker 映像
     - 推送到 Container Registry
     - 部署到 Cloud Run
     - 設定 Cloud Scheduler

3. **檢查部署狀態**
   ```bash
   # 查看 Cloud Build 狀態
   gcloud builds list --limit=5
   
   # 查看 Cloud Run 服務
   gcloud run services list
   
   # 查看服務 URL
   gcloud run services describe garbage-linebot --region=asia-east1 --format='value(status.url)'
   ```

## 設定 LINE Bot Webhook

1. **取得 Cloud Run 服務 URL**
   ```bash
   SERVICE_URL=$(gcloud run services describe garbage-linebot --region=asia-east1 --format='value(status.url)')
   echo "Webhook URL: ${SERVICE_URL}/line/callback"
   ```

2. **在 LINE Developers Console 設定 Webhook URL**
   - 前往 [LINE Developers Console](https://developers.line.biz/)
   - 選擇你的 Bot
   - 在 Messaging API 設定中設定 Webhook URL: `https://your-service-url/line/callback`

## 監控和日誌

### 查看應用程式日誌
```bash
gcloud logs read "resource.type=cloud_run_revision AND resource.labels.service_name=garbage-linebot" --limit=50
```

### 查看 Cloud Build 日誌
```bash
gcloud builds log [BUILD_ID]
```

### 設定錯誤通知
建議設定 Cloud Monitoring 警報來監控服務狀態和錯誤率。

## 疑難排解

### 常見問題

1. **權限錯誤**: 確認 Cloud Build 服務帳戶有足夠權限
2. **環境變數未設定**: 檢查觸發器的替代變數設定
3. **API 未啟用**: 確認所有必要的 GCP API 都已啟用
4. **Firestore 權限**: 確認應用程式有 Firestore 讀寫權限

### 手動部署測試
```bash
# 手動觸發建構
gcloud builds submit --config cloudbuild.yaml

# 本地測試 Docker 映像
docker build -t garbage-linebot-test .
docker run -p 8080:8080 --env-file .env garbage-linebot-test
```