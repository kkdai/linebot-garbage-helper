package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"regexp"
	"strings"

	"linebot-garbage-helper/internal/gemini"
	"linebot-garbage-helper/internal/geo"
)

func main() {
	// 從環境變數獲取 API keys
	geminiAPIKey := os.Getenv("GEMINI_API_KEY")
	googleAPIKey := os.Getenv("GOOGLE_MAPS_API_KEY")
	
	if geminiAPIKey == "" {
		log.Fatal("請設定 GEMINI_API_KEY 環境變數")
	}
	if googleAPIKey == "" {
		log.Fatal("請設定 GOOGLE_MAPS_API_KEY 環境變數")
	}

	ctx := context.Background()

	// 初始化客戶端
	geminiClient, err := gemini.NewGeminiClient(ctx, geminiAPIKey, "gemini-2.0-flash-exp")
	if err != nil {
		log.Fatalf("Failed to create Gemini client: %v", err)
	}
	defer geminiClient.Close()

	geoClient, err := geo.NewGeocodeClient(googleAPIKey)
	if err != nil {
		log.Fatalf("Failed to create Geo client: %v", err)
	}

	// 測試地址
	testAddresses := []string{
		"台北市中正區重慶南路一段122號",
		"台北市大安區忠孝東路四段",
		"新北市板橋區縣民大道二段7號",
		"高雄市左營區博愛二路777號",
		"台中市西屯區文華路100號",
		"桃園市中壢區中大路300號",
	}

	fmt.Println("開始測試地址處理邏輯...")
	fmt.Println("=" * 60)

	for i, address := range testAddresses {
		fmt.Printf("\n測試 %d: %s\n", i+1, address)
		fmt.Println("-" * 40)
		
		success := testAddressProcessing(ctx, geminiClient, geoClient, address)
		if success {
			fmt.Printf("✅ 測試成功: %s\n", address)
		} else {
			fmt.Printf("❌ 測試失敗: %s\n", address)
		}
	}

	fmt.Println("\n" + "=" * 60)
	fmt.Println("測試完成")
}

func testAddressProcessing(ctx context.Context, geminiClient *gemini.GeminiClient, geoClient *geo.GeocodeClient, text string) bool {
	fmt.Printf("📍 原始輸入: %s\n", text)

	// Step 1: Gemini 意圖分析
	fmt.Println("\n1️⃣ 嘗試 Gemini 意圖分析...")
	intent, err := geminiClient.AnalyzeIntent(ctx, text)
	if err != nil {
		fmt.Printf("   ❌ Gemini 意圖分析失敗: %v\n", err)
		intent = nil
	} else {
		fmt.Printf("   ✅ Gemini 意圖分析成功: %+v\n", intent)
	}

	// Step 2: 地址提取邏輯
	var addressToGeocode string
	var addressMethod string
	
	fmt.Println("\n2️⃣ 地址提取...")
	
	// 方法1：使用 Gemini 解析的 District
	if intent != nil && intent.District != "" {
		addressToGeocode = intent.District
		addressMethod = "intent.District"
		fmt.Printf("   ✅ Method 1 - 使用意圖分析的 District: %s\n", addressToGeocode)
	} else {
		// 方法2：使用 Gemini 提取地址
		fmt.Println("   🔄 Method 2 - 嘗試 Gemini 地址提取...")
		extractedLocation, err := geminiClient.ExtractLocationFromText(ctx, text)
		if err == nil && extractedLocation != "" && strings.TrimSpace(extractedLocation) != "" {
			addressToGeocode = strings.TrimSpace(extractedLocation)
			addressMethod = "gemini.ExtractLocation"
			fmt.Printf("   ✅ Method 2 - 提取地址成功: %s\n", addressToGeocode)
		} else {
			// 方法3：直接使用原始文字作為地址
			addressToGeocode = text
			addressMethod = "original.text"
			fmt.Printf("   ⚠️ Method 3 - 使用原始文字: %s\n", addressToGeocode)
			if err != nil {
				fmt.Printf("   ❌ Gemini 地址提取失敗: %v\n", err)
			}
		}
	}

	// Step 3: 地理編碼
	fmt.Println("\n3️⃣ 地理編碼...")
	fmt.Printf("   🔄 嘗試地理編碼: '%s' (方法: %s)\n", addressToGeocode, addressMethod)
	
	location, err := geoClient.GeocodeAddress(ctx, addressToGeocode)
	if err != nil {
		fmt.Printf("   ❌ 地理編碼失敗: %v\n", err)
		
		// Fallback 1: 使用原始文字
		if addressMethod != "original.text" {
			fmt.Println("   🔄 Fallback 1 - 嘗試原始文字...")
			location, err = geoClient.GeocodeAddress(ctx, text)
			if err == nil {
				fmt.Printf("   ✅ Fallback 1 成功: %+v\n", location)
				return true
			}
			fmt.Printf("   ❌ Fallback 1 失敗: %v\n", err)
		}
		
		// Fallback 2: 簡化地址
		simplifiedAddress := extractSimplifiedAddress(text)
		if simplifiedAddress != "" && simplifiedAddress != addressToGeocode && simplifiedAddress != text {
			fmt.Printf("   🔄 Fallback 2 - 嘗試簡化地址: %s\n", simplifiedAddress)
			location, err = geoClient.GeocodeAddress(ctx, simplifiedAddress)
			if err == nil {
				fmt.Printf("   ✅ Fallback 2 成功: %+v\n", location)
				return true
			}
			fmt.Printf("   ❌ Fallback 2 失敗: %v\n", err)
		}
		
		return false
	}
	
	fmt.Printf("   ✅ 地理編碼成功: %+v\n", location)
	return true
}

func extractSimplifiedAddress(text string) string {
	// 嘗試提取縣市區的模式
	patterns := []string{
		`(台北市|新北市|桃園市|台中市|台南市|高雄市|基隆市|新竹市|嘉義市)[^市]*?(區|市)`,
		`(新竹縣|苗栗縣|彰化縣|南投縣|雲林縣|嘉義縣|屏東縣|宜蘭縣|花蓮縣|台東縣|澎湖縣|金門縣|連江縣)[^縣]*?(鄉|鎮|市)`,
	}
	
	for _, pattern := range patterns {
		re := regexp.MustCompile(pattern)
		if match := re.FindString(text); match != "" {
			fmt.Printf("   📍 提取簡化地址: %s (使用模式: %s)\n", match, pattern)
			return match
		}
	}
	
	// 如果沒有匹配，嘗試提取縣市
	cityPattern := `(台北市|新北市|桃園市|台中市|台南市|高雄市|基隆市|新竹市|嘉義市|新竹縣|苗栗縣|彰化縣|南投縣|雲林縣|嘉義縣|屏東縣|宜蘭縣|花蓮縣|台東縣|澎湖縣|金門縣|連江縣)`
	re := regexp.MustCompile(cityPattern)
	if match := re.FindString(text); match != "" {
		fmt.Printf("   📍 提取縣市: %s\n", match)
		return match
	}
	
	return ""
}