# 地址查詢功能修正說明

## 🔍 問題診斷

你提到的問題：**輸入地址不會直接查詢相關資訊，只有 LINE 的位址資訊才有效**

### 原始問題分析

1. **複雜的處理流程**：文字輸入需要經過多層解析，任何一層失敗都會中斷
2. **過度依賴 Gemini**：依賴 Gemini 正確解析 `District` 或 `ExtractLocationFromText`
3. **容錯性不足**：當 AI 解析失敗時，沒有備用方案
4. **錯誤處理不友善**：失敗時提供的錯誤訊息不夠具體

### LINE 位置 vs 文字地址的差異

**LINE 位置訊息**：
- 直接提供經緯度座標
- 跳過所有地址解析步驟
- 直接調用 `searchNearbyGarbageTrucks`

**文字地址輸入**：
- 需要 Gemini 分析意圖
- 需要提取地址資訊  
- 需要 Google Maps 地理編碼
- 多個步驟，容易失敗

## ✅ 修正方案

### 1. 簡化處理流程

**改進前**：
```
文字輸入 → Gemini 分析 → 檢查 District → 如果無 District → 呼叫 ExtractLocationFromText → 地理編碼
```

**改進後**：
```
文字輸入 → 收藏地點檢查 → 多層地址提取 → 地理編碼
```

### 2. 多層地址提取策略

```go
// 方法1：使用 Gemini 解析的 District
if intent.District != "" {
    addressToGeocode = intent.District
}
// 方法2：使用 Gemini 提取地址  
else if extractedLocation != "" {
    addressToGeocode = extractedLocation
}
// 方法3：直接使用原始文字作為地址
else {
    addressToGeocode = text
}
```

### 3. 改善錯誤處理

**改進前**：
```go
h.replyMessage(ctx, userID, "請提供具體的地址或分享您的位置，我幫您查詢附近的垃圾車。")
```

**改進後**：
```go
h.replyMessage(ctx, userID, fmt.Sprintf("抱歉，我找不到「%s」的位置資訊。\n\n💡 請嘗試：\n📍 分享您的位置\n💬 輸入更具體的地址（如：台北市信義區忠孝東路）", text))
```

## 🚀 測試方案

### 測試案例 1：簡單地址
```
輸入：「台北市信義區」
預期：找到信義區附近的垃圾車站點
```

### 測試案例 2：詳細地址
```
輸入：「台北市信義區忠孝東路五段」
預期：找到該地址附近的垃圾車站點
```

### 測試案例 3：不完整地址
```
輸入：「信義區」
預期：嘗試地理編碼，如果失敗提供友善的錯誤訊息
```

### 測試案例 4：無效地址
```
輸入：「abcdefg」
預期：提供具體的錯誤訊息和改善建議
```

## 🔧 部署和測試

### 1. 部署修正版本
```bash
git add .
git commit -m "fix: improve address query reliability with fallback strategies"
git push origin main
```

### 2. 測試步驟

1. **簡單地址測試**：
   - 輸入：「台北車站」
   - 檢查是否回傳垃圾車資訊

2. **詳細地址測試**：
   - 輸入：「台北市中正區重慶南路一段122號」
   - 檢查是否回傳垃圾車資訊

3. **模糊地址測試**：
   - 輸入：「信義區」
   - 檢查是否能成功解析

4. **錯誤處理測試**：
   - 輸入：「不存在的地址xyzabc」
   - 檢查錯誤訊息是否友善

### 3. 檢查日誌
```bash
# 檢查地址解析日誌
gcloud logs read "resource.type=cloud_run_revision AND resource.labels.service_name=garbage-linebot AND textPayload:~'Geocoding'" --limit=20

# 檢查錯誤日誌
gcloud logs read "resource.type=cloud_run_revision AND resource.labels.service_name=garbage-linebot AND textPayload:~'Error geocoding'" --limit=10
```

## 📊 改進效果

### 提高成功率
- **多層地址提取**：即使 Gemini 失敗，仍有備用方案
- **直接地理編碼**：允許直接使用用戶輸入的文字進行地理編碼
- **更好的容錯性**：減少因單一步驟失敗導致的整體失敗

### 改善用戶體驗
- **更具體的錯誤訊息**：告訴用戶具體輸入了什麼，以及如何改善
- **明確的建議**：提供具體的輸入格式範例
- **一致的行為**：文字輸入和 LINE 位置應該有相似的成功率

### 調試和維護
- **詳細的日誌**：每個步驟都有清楚的日誌記錄
- **可追蹤的流程**：容易識別失敗點
- **漸進式降級**：從最精確到最基本的地址解析方法

## 🎯 預期結果

修正後，用戶輸入地址應該能夠：
1. ✅ 成功解析常見的台灣地址格式
2. ✅ 提供友善的錯誤處理和建議
3. ✅ 與 LINE 位置分享功能有相似的可靠性
4. ✅ 支援不同詳細程度的地址輸入

這個修正應該能解決你遇到的地址查詢問題！