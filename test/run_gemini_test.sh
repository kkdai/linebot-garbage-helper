#!/bin/bash

# Gemini 地址處理測試腳本

echo "===================="
echo "Gemini 地址處理測試"
echo "===================="

# 檢查是否設定了必要的環境變數
if [ -z "$GEMINI_API_KEY" ]; then
    echo "❌ 請設定 GEMINI_API_KEY 環境變數"
    echo ""
    echo "使用方式："
    echo "export GEMINI_API_KEY='your_gemini_api_key_here'"
    echo "./test/run_gemini_test.sh"
    exit 1
fi

echo "✅ GEMINI_API_KEY 已設定"
echo ""

# 進入專案根目錄
cd "$(dirname "$0")/.."

# 編譯並執行測試
echo "🔨 編譯測試程式..."
go build -o test/gemini_test_main test/gemini_test_main.go

if [ $? -ne 0 ]; then
    echo "❌ 編譯失敗"
    exit 1
fi

echo "✅ 編譯成功"
echo ""

echo "🧪 開始執行 Gemini 測試..."
echo ""
./test/gemini_test_main

# 清理
rm -f test/gemini_test_main

echo ""
echo "測試完成！"