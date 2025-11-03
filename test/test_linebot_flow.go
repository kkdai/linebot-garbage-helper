package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"

	"linebot-garbage-helper/internal/gemini"
)

func main() {
	geminiAPIKey := os.Getenv("GEMINI_API_KEY")
	if geminiAPIKey == "" {
		log.Fatal("請設定 GEMINI_API_KEY 環境變數")
	}

	ctx := context.Background()
	
	// 使用正式版本 model (與線上 LINE Bot 相同)
	model := "gemini-2.0-flash"
	fmt.Printf("🤖 使用 Model: %s (LINE Bot 正式版本)\n", model)
	fmt.Println(strings.Repeat("=", 70))
	
	geminiClient, err := gemini.NewGeminiClient(ctx, geminiAPIKey, model)
	if err != nil {
		log.Fatalf("Failed to create Gemini client: %v", err)
	}
	defer geminiClient.Close()

	// 測試兩個問題案例 - 模擬 LINE Bot 的完整處理流程
	testAddresses := []string{
		"新北市三重區仁義街",
		"台北市中正區重慶南路一段122號",
	}

	for i, text := range testAddresses {
		fmt.Printf("\n🧪 測試案例 %d: %s\n", i+1, text)
		fmt.Println(strings.Repeat("-", 70))
		fmt.Println("📋 模擬 LINE Bot handler.go:handleTextMessage() 流程\n")

		// Step 1: Intent Analysis (handler.go:165)
		fmt.Println("Step 1️⃣: AnalyzeIntent")
		intent, err := geminiClient.AnalyzeIntent(ctx, text)
		if err != nil {
			fmt.Printf("   ❌ 意圖分析失敗: %v\n", err)
			intent = nil
		} else {
			fmt.Printf("   ✅ District: '%s'\n", intent.District)
			fmt.Printf("   ✅ Keywords: %v\n", intent.Keywords)
		}

		// Step 2: Address Extraction Logic (handler.go:189-214)
		var addressToGeocode string
		var addressMethod string
		
		fmt.Println("\nStep 2️⃣: Address Extraction")
		
		// Method 1: Using District from Intent (handler.go:194)
		if intent != nil && intent.District != "" {
			addressToGeocode = intent.District
			addressMethod = "intent.District"
			fmt.Printf("   ✅ Method 1 - Using district from intent: '%s'\n", addressToGeocode)
		} else {
			fmt.Println("   ⚠️ Method 1 - No district from intent")
			
			// Method 2: Gemini ExtractLocationFromText (handler.go:200)
			fmt.Println("   🔄 Trying Method 2 - ExtractLocationFromText...")
			extractedLocation, err := geminiClient.ExtractLocationFromText(ctx, text)
			if err == nil && extractedLocation != "" && strings.TrimSpace(extractedLocation) != "" {
				addressToGeocode = strings.TrimSpace(extractedLocation)
				addressMethod = "gemini.ExtractLocation"
				fmt.Printf("   ✅ Method 2 - Extracted location: '%s'\n", addressToGeocode)
			} else {
				// Method 3: Use Original Text (handler.go:207)
				addressToGeocode = text
				addressMethod = "original.text"
				fmt.Printf("   ⚠️ Method 2 failed: %v\n", err)
				fmt.Printf("   ✅ Method 3 - Using original text: '%s'\n", addressToGeocode)
			}
		}
		
		// Step 3: What would be sent to Geocoding (handler.go:218)
		fmt.Println("\nStep 3️⃣: Geocoding Input")
		fmt.Printf("   📍 Address to geocode: '%s'\n", addressToGeocode)
		fmt.Printf("   🔧 Method used: %s\n", addressMethod)
		
		// Analysis
		fmt.Println("\n📊 Analysis:")
		if addressToGeocode == text {
			fmt.Println("   ✅ Good: Will use original text for geocoding")
		} else if intent != nil && intent.District != "" {
			if strings.Contains(text, addressToGeocode) {
				fmt.Println("   ⚠️ Warning: Only using district, may lose specific address info")
				fmt.Printf("      Original: '%s'\n", text)
				fmt.Printf("      Will use: '%s'\n", addressToGeocode)
			}
		}
		
		fmt.Println()
	}

	fmt.Println(strings.Repeat("=", 70))
	fmt.Println("✨ 分析完成！")
	fmt.Println("\n💡 結論:")
	fmt.Println("如果 Google Maps Geocoding API 無法識別縣市+區域的簡化地址,")
	fmt.Println("但可以識別完整地址,則會觸發 Fallback 機制使用原始文字。")
	fmt.Println("問題可能在於 Google Maps API 的地址識別,而非 Gemini model 版本。")
}
