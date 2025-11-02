# 地址處理測試

這個測試程式用來驗證地址處理邏輯是否正常工作，特別是針對「台北市中正區重慶南路一段122號」這類具體地址。

## 測試選項

### 1. 完整測試 (需要 Gemini + Google Maps API)

測試完整的地址處理流程，包括地理編碼：

```bash
# 設定環境變數
export GEMINI_API_KEY='your_gemini_api_key_here'
export GOOGLE_MAPS_API_KEY='your_google_maps_api_key_here'

# 執行測試
./test/run_test.sh
```

### 2. Gemini 測試 (只需要 Gemini API)

只測試 Gemini 相關的地址處理邏輯：

```bash
# 設定環境變數
export GEMINI_API_KEY='your_gemini_api_key_here'

# 執行測試
./test/run_gemini_test.sh
```

## 測試地址

程式會測試以下地址：

1. `台北市中正區重慶南路一段122號` - 問題地址
2. `台北市大安區忠孝東路四段` - 一般地址
3. `新北市板橋區縣民大道二段7號` - 具體地址
4. `高雄市左營區博愛二路777號` - 南部地址
5. `家` - 收藏地點
6. `我晚上七點前在台北市大安區哪裡倒垃圾？` - 時間查詢

## 測試流程

1. **Gemini 意圖分析** - 分析用戶意圖和提取地區資訊
2. **Gemini 地址提取** - 從文字中提取地址
3. **本地地址簡化** - 使用正則表達式提取縣市區
4. **地理編碼** (完整測試) - 將地址轉換為經緯度
5. **Fallback 策略** - 多層備用方案

## 預期結果

- 「台北市中正區重慶南路一段122號」應該能通過至少一種方法成功處理
- 如果 Gemini 分析失敗，應該有適當的 fallback
- 最終應該能提取出「台北市中正區」作為備用地址

## 除錯資訊

測試程式會顯示詳細的處理步驟和結果，方便除錯：

- ✅ 成功步驟
- ❌ 失敗步驟  
- 🔄 正在處理
- ⚠️ 警告或 fallback
- 📍 地址資訊
- 🎯 最終結果

## 使用建議

建議先執行 `run_gemini_test.sh` 來快速測試 Gemini 相關邏輯，確認 API key 正常且 Gemini 回應符合預期。如果需要測試完整的地理編碼流程，再執行 `run_test.sh`。