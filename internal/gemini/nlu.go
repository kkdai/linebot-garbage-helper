package gemini

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/generative-ai-go/genai"
	"google.golang.org/api/option"
)

type GeminiClient struct {
	client *genai.Client
	model  string
}

type IntentResult struct {
	District   string      `json:"district"`
	TimeWindow TimeWindow  `json:"time_window"`
	Keywords   []string    `json:"keywords"`
	QueryType  string      `json:"query_type"`
}

type TimeWindow struct {
	From string `json:"from"`
	To   string `json:"to"`
}

func NewGeminiClient(ctx context.Context, apiKey, model string) (*GeminiClient, error) {
	client, err := genai.NewClient(ctx, option.WithAPIKey(apiKey))
	if err != nil {
		return nil, err
	}
	
	return &GeminiClient{
		client: client,
		model:  model,
	}, nil
}

func (gc *GeminiClient) Close() error {
	return gc.client.Close()
}

func (gc *GeminiClient) AnalyzeIntent(ctx context.Context, userMessage string) (*IntentResult, error) {
	model := gc.client.GenerativeModel(gc.model)
	
	prompt := fmt.Sprintf(`分析使用者關於垃圾車的查詢，並提取地址資訊。

任務：從輸入文字中提取地址的「縣市」和「區/鄉鎮」。

步驟：
1. 識別文字中的縣市名稱（如：台北市、新北市、桃園市等）
2. 識別文字中的區/鄉鎮名稱（如：中正區、三重區、板橋區等）
3. 將縣市和區/鄉鎮組合成完整地址（如：台北市中正區、新北市三重區）

critical_rules：
- 如果文字同時包含縣市和區域，district 必須包含兩者
- "新北市三重區仁義街" → district = "新北市三重區"
- "台北市中正區重慶南路一段122號" → district = "台北市中正區"
- "台北市" → district = "台北市"

輸出 JSON 格式：
{
  "district": "縣市+區域的完整組合",
  "time_window": {"from": "", "to": ""},
  "keywords": ["關鍵字"],
  "query_type": "garbage_truck_eta"
}

範例：

Input: "新北市三重區仁義街"
Output: {"district": "新北市三重區", "time_window": {"from": "", "to": ""}, "keywords": ["新北市", "三重區", "仁義街"], "query_type": "garbage_truck_eta"}

Input: "台北市中正區重慶南路一段122號"
Output: {"district": "台北市中正區", "time_window": {"from": "", "to": ""}, "keywords": ["台北市", "中正區", "重慶南路"], "query_type": "garbage_truck_eta"}

Input: "台北市"
Output: {"district": "台北市", "time_window": {"from": "", "to": ""}, "keywords": ["台北市"], "query_type": "garbage_truck_eta"}

現在分析：「%s」

只回傳 JSON，不要其他文字。`, userMessage)

	resp, err := model.GenerateContent(ctx, genai.Text(prompt))
	if err != nil {
		return nil, err
	}
	
	if len(resp.Candidates) == 0 || len(resp.Candidates[0].Content.Parts) == 0 {
		return nil, fmt.Errorf("no response from Gemini")
	}
	
	responseText := fmt.Sprintf("%v", resp.Candidates[0].Content.Parts[0])
	
	var result IntentResult
	if err := json.Unmarshal([]byte(responseText), &result); err != nil {
		return &IntentResult{
			District:  extractDistrict(userMessage),
			Keywords:  []string{userMessage},
			QueryType: "garbage_truck_eta",
			TimeWindow: TimeWindow{
				From: "",
				To:   "",
			},
		}, nil
	}
	
	if result.QueryType == "" {
		result.QueryType = "garbage_truck_eta"
	}
	
	return &result, nil
}

func (gc *GeminiClient) ExtractLocationFromText(ctx context.Context, text string) (string, error) {
	model := gc.client.GenerativeModel(gc.model)
	
	prompt := fmt.Sprintf(`請從以下文字中抽取出地址或地名，如果找不到具體地址，請回傳空字串。

文字：「%s」

請只回傳地址或地名，不要包含其他說明文字。如果沒有找到地址，請回傳空字串。`, text)

	resp, err := model.GenerateContent(ctx, genai.Text(prompt))
	if err != nil {
		return "", err
	}
	
	if len(resp.Candidates) == 0 || len(resp.Candidates[0].Content.Parts) == 0 {
		return "", fmt.Errorf("no response from Gemini")
	}
	
	location := fmt.Sprintf("%v", resp.Candidates[0].Content.Parts[0])
	// 清理回應文字，移除多餘的換行符和空白
	location = strings.TrimSpace(location)
	return location, nil
}

func extractDistrict(text string) string {
	commonDistricts := []string{
		"台北市", "新北市", "桃園市", "台中市", "台南市", "高雄市",
		"基隆市", "新竹市", "新竹縣", "苗栗縣", "彰化縣", "南投縣",
		"雲林縣", "嘉義市", "嘉義縣", "屏東縣", "宜蘭縣", "花蓮縣",
		"台東縣", "澎湖縣", "金門縣", "連江縣",
	}
	
	for _, district := range commonDistricts {
		if contains(text, district) {
			return district
		}
	}
	
	return ""
}

func contains(text, substr string) bool {
	return len(text) >= len(substr) && 
		   findSubstring(text, substr) != -1
}

func findSubstring(text, substr string) int {
	textRunes := []rune(text)
	substrRunes := []rune(substr)
	
	if len(substrRunes) > len(textRunes) {
		return -1
	}
	
	for i := 0; i <= len(textRunes)-len(substrRunes); i++ {
		found := true
		for j := 0; j < len(substrRunes); j++ {
			if textRunes[i+j] != substrRunes[j] {
				found = false
				break
			}
		}
		if found {
			return i
		}
	}
	
	return -1
}

func (gc *GeminiClient) ParseTimeWindow(timeWindow TimeWindow) (time.Time, time.Time, error) {
	var fromTime, toTime time.Time
	var err error
	
	if timeWindow.From != "" {
		fromTime, err = parseTimeString(timeWindow.From)
		if err != nil {
			return time.Time{}, time.Time{}, err
		}
	}
	
	if timeWindow.To != "" {
		toTime, err = parseTimeString(timeWindow.To)
		if err != nil {
			return time.Time{}, time.Time{}, err
		}
	}
	
	return fromTime, toTime, nil
}

func parseTimeString(timeStr string) (time.Time, error) {
	now := time.Now()
	
	t, err := time.Parse("15:04", timeStr)
	if err != nil {
		return time.Time{}, err
	}
	
	return time.Date(now.Year(), now.Month(), now.Day(), t.Hour(), t.Minute(), 0, 0, now.Location()), nil
}