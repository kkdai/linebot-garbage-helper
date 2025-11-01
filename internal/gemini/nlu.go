package gemini

import (
	"context"
	"encoding/json"
	"fmt"
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
	
	prompt := fmt.Sprintf(`你是一個查詢意圖分析器，專門分析使用者關於垃圾車的查詢。

使用者輸入可能包含地名與時間。請分析輸入並輸出 JSON 格式的結果。

輸出格式：
{
  "district": "地區名稱（如果有的話）",
  "time_window": {
    "from": "開始時間（HH:MM格式，如果有的話）",
    "to": "結束時間（HH:MM格式，如果有的話）"
  },
  "keywords": ["關鍵字陣列"],
  "query_type": "garbage_truck_eta"
}

範例：
輸入：「我晚上七點前在台北市大安區哪裡倒垃圾？」
輸出：
{
  "district": "台北市大安區",
  "time_window": {
    "from": "",
    "to": "19:00"
  },
  "keywords": ["台北市", "大安區", "倒垃圾", "晚上", "七點"],
  "query_type": "garbage_truck_eta"
}

請分析以下使用者輸入：
「%s」

請只回傳 JSON，不要包含其他說明文字。`, userMessage)

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