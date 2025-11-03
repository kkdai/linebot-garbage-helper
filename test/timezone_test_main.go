package main

import (
	"fmt"
	"time"

	"linebot-garbage-helper/internal/utils"
)

func main() {
	fmt.Println("時區修復測試")
	fmt.Println("====================")

	// 測試目前時間
	now := time.Now()
	nowTaiwan := utils.NowInTaiwan()
	
	fmt.Printf("系統時間 (可能是 UTC): %s\n", now.Format("2006-01-02 15:04:05 MST"))
	fmt.Printf("台灣時間: %s\n", nowTaiwan.Format("2006-01-02 15:04:05 MST"))
	
	// 測試時區轉換
	utcTime := time.Date(2025, 11, 3, 5, 0, 0, 0, time.UTC) // 假設 UTC 05:00
	taiwanTime := utils.ToTaiwan(utcTime)
	
	fmt.Printf("\nUTC 05:00 轉換為台灣時間: %s\n", taiwanTime.Format("2006-01-02 15:04:05 MST"))
	
	// 測試垃圾車時間解析
	fmt.Println("\n垃圾車時間解析測試：")
	testTimes := []string{"1900", "19:00", "0830", "08:30"}
	
	for _, timeStr := range testTimes {
		taipeiTZ := utils.GetTaiwanTimezone()
		parsedTime, err := parseTimeToToday(timeStr, taipeiTZ)
		if err != nil {
			fmt.Printf("解析 %s 失敗: %v\n", timeStr, err)
		} else {
			fmt.Printf("解析 %s -> %s\n", timeStr, parsedTime.Format("2006-01-02 15:04:05 MST"))
		}
	}
	
	// 測試提醒時間計算
	fmt.Println("\n提醒時間計算測試：")
	
	// 假設垃圾車 ETA 是今晚 19:00 (台灣時間)
	today := nowTaiwan
	eta := time.Date(today.Year(), today.Month(), today.Day(), 19, 0, 0, 0, utils.GetTaiwanTimezone())
	
	// 提前 10 分鐘提醒
	reminderTime := eta.Add(-10 * time.Minute)
	timeUntilETA := eta.Sub(nowTaiwan)
	timeUntilReminder := reminderTime.Sub(nowTaiwan)
	
	fmt.Printf("垃圾車 ETA: %s\n", eta.Format("2006-01-02 15:04:05 MST"))
	fmt.Printf("提醒時間: %s\n", reminderTime.Format("2006-01-02 15:04:05 MST"))
	fmt.Printf("距離 ETA: %.0f 分鐘\n", timeUntilETA.Minutes())
	fmt.Printf("距離提醒: %.0f 分鐘\n", timeUntilReminder.Minutes())
	
	if timeUntilReminder.Minutes() > 0 {
		fmt.Printf("✅ 提醒將在 %.0f 分鐘後發送\n", timeUntilReminder.Minutes())
	} else if timeUntilReminder.Minutes() > -10 {
		fmt.Printf("🔔 現在應該發送提醒（距離提醒時間 %.1f 分鐘）\n", -timeUntilReminder.Minutes())
	} else {
		fmt.Printf("⏰ 提醒時間已過\n")
	}
}

func parseTimeToToday(timeStr string, tz *time.Location) (time.Time, error) {
	now := time.Now().In(tz)
	
	if len(timeStr) == 4 {
		layout := "1504"
		t, err := time.Parse(layout, timeStr)
		if err != nil {
			return time.Time{}, err
		}
		return time.Date(now.Year(), now.Month(), now.Day(), t.Hour(), t.Minute(), 0, 0, tz), nil
	}
	
	layout := "15:04"
	t, err := time.Parse(layout, timeStr)
	if err != nil {
		return time.Time{}, err
	}
	
	return time.Date(now.Year(), now.Month(), now.Day(), t.Hour(), t.Minute(), 0, 0, tz), nil
}