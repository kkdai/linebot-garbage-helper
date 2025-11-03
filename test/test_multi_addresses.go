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
	geminiClient, err := gemini.NewGeminiClient(ctx, geminiAPIKey, "gemini-2.0-flash-exp")
	if err != nil {
		log.Fatalf("Failed to create Gemini client: %v", err)
	}
	defer geminiClient.Close()

	// 測試多個地址案例
	testAddresses := []string{
		"新北市三重區仁義街",
		"台北市中正區重慶南路一段122號",
	}

	for i, testAddress := range testAddresses {
		fmt.Printf("\n🧪 測試案例 %d: %s\n", i+1, testAddress)
		fmt.Println(strings.Repeat("=", 60))

		// 測試意圖分析
		fmt.Println("1️⃣ 測試意圖分析...")
		intent, err := geminiClient.AnalyzeIntent(ctx, testAddress)
		if err != nil {
			fmt.Printf("❌ 意圖分析失敗: %v\n", err)
		} else {
			fmt.Printf("✅ 意圖分析成功:\n")
			fmt.Printf("   District: '%s'\n", intent.District)
			fmt.Printf("   Keywords: %v\n", intent.Keywords)
			fmt.Printf("   QueryType: '%s'\n", intent.QueryType)

			// 檢查是否正確提取了區域
			if strings.Contains(testAddress, intent.District) && intent.District != "" {
				fmt.Println("🎯 完美！正確提取了區域資訊")
			} else if intent.District != "" {
				fmt.Printf("⚠️ 提取了部分資訊: '%s'\n", intent.District)
			} else {
				fmt.Println("❌ 未能提取區域資訊")
			}
		}

		fmt.Println("\n2️⃣ 測試地址提取...")
		extractedLocation, err := geminiClient.ExtractLocationFromText(ctx, testAddress)
		if err != nil {
			fmt.Printf("❌ 地址提取失敗: %v\n", err)
		} else {
			fmt.Printf("✅ 地址提取成功: '%s'\n", extractedLocation)

			// 檢查是否包含完整地址
			if extractedLocation == testAddress {
				fmt.Println("🎯 完美！提取了完整地址")
			} else if extractedLocation != "" && strings.TrimSpace(extractedLocation) != "" {
				fmt.Printf("⚠️ 提取了部分地址: '%s'\n", extractedLocation)
			} else {
				fmt.Println("❌ 未能提取地址")
			}
		}
	}

	fmt.Println("\n" + strings.Repeat("=", 60))
	fmt.Println("✨ 所有測試完成！")
}
