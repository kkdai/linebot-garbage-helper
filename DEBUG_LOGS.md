# 除錯日誌指南

## 🐛 問題診斷

如果 LINE Bot 沒有回覆，請按照以下步驟檢查日誌：

## 📋 新增的日誌項目

### 1. **HTTP 請求日誌**
```
Incoming request: POST /line/callback from 192.168.1.1
```
- **位置**: `cmd/server/main.go`
- **用途**: 確認 LINE 是否有發送 webhook 請求到你的服務器

### 2. **Webhook 處理日誌**
```
Received webhook request from 192.168.1.1
Successfully parsed webhook, processing 2 events
Processing event 1/2, type: webhook.MessageEvent
```
- **位置**: `internal/line/handler.go:HandleWebhook`
- **用途**: 確認 webhook 解析是否成功

### 3. **訊息事件日誌**
```
Processing MessageEvent
User ID: U1234567890abcdef
Message type: webhook.TextMessageContent
Text message received: 你好
```
- **位置**: `internal/line/handler.go:handleMessageEvent`
- **用途**: 確認收到什麼類型的訊息

### 4. **文字處理日誌**
```
Processing text message from user U1234567890abcdef: 你好
Greeting detected: 你好
```
- **位置**: `internal/line/handler.go:handleTextMessage`
- **用途**: 確認訊息處理邏輯

### 5. **地理編碼日誌**
```
Analyzing intent for text: 台北市信義區
Intent analysis result: {District:台北市信義區 TimeWindow:{From: To:} Keywords:[]}
Geocoding district: 台北市信義區
Geocoded successfully: {Lat:25.033 Lng:121.564 Address:台北市信義區}
```
- **位置**: `internal/line/handler.go:handleTextMessage`
- **用途**: 確認地址解析是否正常

### 6. **垃圾車查詢日誌**
```
Searching nearby garbage trucks for user U1234567890abcdef at coordinates: lat=25.033000, lng=121.564000
Successfully fetched garbage data, 4031 collection points available
Found 3 nearest stops
```
- **位置**: `internal/line/handler.go:searchNearbyGarbageTrucks`
- **用途**: 確認垃圾車資料查詢是否正常

### 7. **訊息發送日誌**
```
Sending reply to user U1234567890abcdef: 👋 您好！歡迎使用垃圾車助手！
Attempting to send message to user: U1234567890abcdef
Calling LINE Messaging API...
Message sent successfully to user U1234567890abcdef. Response: {...}
```
- **位置**: `internal/line/handler.go:sendMessage`
- **用途**: 確認是否成功發送訊息到 LINE

## 🔍 診斷步驟

### 步驟 1: 檢查是否收到 webhook
在 Cloud Run 日誌中尋找：
```
Incoming request: POST /line/callback from xxx.xxx.xxx.xxx
```

**如果沒有看到**：
- 檢查 LINE Bot 的 webhook URL 設定是否正確
- 確認 Cloud Run 服務是否正常運行

### 步驟 2: 檢查 webhook 解析
尋找：
```
Successfully parsed webhook, processing X events
```

**如果看到錯誤**：
```
Cannot parse request: ...
```
- 檢查 LINE_CHANNEL_SECRET 環境變數是否正確

### 步驟 3: 檢查訊息處理
尋找：
```
Processing MessageEvent
User ID: Uxxxxxxxxxxxx
Text message received: [你的訊息]
```

**如果沒有看到**：
- 可能是 LINE Bot 沒有正確接收到訊息

### 步驟 4: 檢查 API 調用
尋找 Gemini API 或 Google Maps API 的錯誤：
```
Error analyzing intent for user Uxxxx: ...
Error geocoding address 'xxx' for user Uxxxx: ...
```

**常見問題**：
- API 金鑰過期或無效
- API 配額超限

### 步驟 5: 檢查訊息發送
尋找：
```
Message sent successfully to user Uxxxx
```

**如果看到錯誤**：
```
Error sending message to user Uxxxx: ...
```
- 檢查 LINE_CHANNEL_ACCESS_TOKEN 是否正確
- 檢查 LINE Bot 是否有訊息發送權限

## 📝 Cloud Run 日誌查看指令

```bash
# 查看最近的日誌
gcloud logs read "resource.type=cloud_run_revision AND resource.labels.service_name=garbage-linebot" --limit=50

# 即時監控日誌
gcloud logs tail "resource.type=cloud_run_revision AND resource.labels.service_name=garbage-linebot"

# 過濾特定錯誤
gcloud logs read "resource.type=cloud_run_revision AND resource.labels.service_name=garbage-linebot AND severity>=ERROR" --limit=20
```

## 🔧 常見問題和解決方案

### 問題 1: 沒有收到任何請求
**可能原因**: LINE webhook URL 設定錯誤
**解決方案**: 檢查 LINE Developers Console 中的 webhook URL

### 問題 2: 收到請求但解析失敗
**可能原因**: LINE_CHANNEL_SECRET 錯誤
**解決方案**: 重新檢查並設定環境變數

### 問題 3: 處理成功但沒有回覆
**可能原因**: LINE_CHANNEL_ACCESS_TOKEN 錯誤或權限不足
**解決方案**: 檢查 token 和 Bot 權限設定

### 問題 4: API 調用失敗
**可能原因**: Google API 金鑰錯誤或配額不足
**解決方案**: 檢查 GOOGLE_MAPS_API_KEY 和 GEMINI_API_KEY

現在重新部署後，你應該能在 Cloud Run 日誌中看到詳細的處理過程！