package line

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/line/line-bot-sdk-go/v8/linebot/messaging_api"
	"github.com/line/line-bot-sdk-go/v8/linebot/webhook"

	"linebot-garbage-helper/internal/garbage"
	"linebot-garbage-helper/internal/gemini"
	"linebot-garbage-helper/internal/geo"
	"linebot-garbage-helper/internal/store"
)

type Handler struct {
	messagingAPI    *messaging_api.MessagingApiAPI
	store           *store.FirestoreClient
	geoClient       *geo.GeocodeClient
	garbageAdapter  *garbage.GarbageAdapter
	geminiClient    *gemini.GeminiClient
	channelSecret   string
}

func NewHandler(
	channelToken, channelSecret string,
	store *store.FirestoreClient,
	geoClient *geo.GeocodeClient,
	garbageAdapter *garbage.GarbageAdapter,
	geminiClient *gemini.GeminiClient,
) (*Handler, error) {
	messagingAPI, err := messaging_api.NewMessagingApiAPI(channelToken)
	if err != nil {
		return nil, err
	}

	return &Handler{
		messagingAPI:   messagingAPI,
		store:          store,
		geoClient:      geoClient,
		garbageAdapter: garbageAdapter,
		geminiClient:   geminiClient,
		channelSecret:  channelSecret,
	}, nil
}

func (h *Handler) HandleWebhook(w http.ResponseWriter, r *http.Request) {
	cb, err := webhook.ParseRequest(h.channelSecret, r)
	if err != nil {
		log.Printf("Cannot parse request: %v", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	for _, event := range cb.Events {
		switch e := event.(type) {
		case webhook.MessageEvent:
			h.handleMessageEvent(r.Context(), e)
		case webhook.PostbackEvent:
			h.handlePostbackEvent(r.Context(), e)
		}
	}

	w.WriteHeader(http.StatusOK)
}

func (h *Handler) handleMessageEvent(ctx context.Context, event webhook.MessageEvent) {
	var userID string
	switch source := event.Source.(type) {
	case *webhook.UserSource:
		userID = source.UserId
	default:
		return
	}

	switch message := event.Message.(type) {
	case webhook.TextMessageContent:
		h.handleTextMessage(ctx, userID, message.Text)
	case webhook.LocationMessageContent:
		h.handleLocationMessage(ctx, userID, message.Latitude, message.Longitude, message.Address)
	}
}

func (h *Handler) handleTextMessage(ctx context.Context, userID, text string) {
	if strings.HasPrefix(text, "/") {
		h.handleCommand(ctx, userID, text)
		return
	}

	// Handle common greetings
	lowerText := strings.ToLower(strings.TrimSpace(text))
	if lowerText == "hi" || lowerText == "hello" || lowerText == "你好" || lowerText == "哈囉" {
		welcomeMsg := `👋 您好！歡迎使用垃圾車助手！

🚀 快速開始：
📍 點擊下方「+」按鈕 → 選擇「位置」→「即時位置」
💬 或直接輸入地址，例如：「台北市信義區」

我會幫您找到最近的垃圾車站點和時間！

輸入 /help 查看更多功能`
		h.replyMessage(ctx, userID, welcomeMsg)
		return
	}

	intent, err := h.geminiClient.AnalyzeIntent(ctx, text)
	if err != nil {
		log.Printf("Error analyzing intent: %v", err)
		h.replyMessage(ctx, userID, "抱歉，我無法理解您的訊息。\n\n💡 您可以：\n📍 分享您的位置\n💬 輸入地址\n❓ 輸入 /help 查看使用說明")
		return
	}

	if intent.District != "" {
		location, err := h.geoClient.GeocodeAddress(ctx, intent.District)
		if err != nil {
			log.Printf("Error geocoding address: %v", err)
			h.replyMessage(ctx, userID, "抱歉，我找不到這個地址的位置資訊。")
			return
		}
		h.searchNearbyGarbageTrucks(ctx, userID, location.Lat, location.Lng, intent)
	} else {
		extractedLocation, err := h.geminiClient.ExtractLocationFromText(ctx, text)
		if err != nil || extractedLocation == "" {
			h.replyMessage(ctx, userID, "請提供具體的地址或分享您的位置，我幫您查詢附近的垃圾車。")
			return
		}

		location, err := h.geoClient.GeocodeAddress(ctx, extractedLocation)
		if err != nil {
			log.Printf("Error geocoding extracted location: %v", err)
			h.replyMessage(ctx, userID, "抱歉，我找不到這個地址的位置資訊。")
			return
		}
		h.searchNearbyGarbageTrucks(ctx, userID, location.Lat, location.Lng, intent)
	}
}

func (h *Handler) handleLocationMessage(ctx context.Context, userID string, lat, lng float64, address string) {
	log.Printf("Received location from user %s: lat=%f, lng=%f, address=%s", userID, lat, lng, address)
	
	// If no address provided by LINE, try reverse geocoding
	if address == "" {
		location, err := h.geoClient.ReverseGeocode(ctx, lat, lng)
		if err != nil {
			log.Printf("Error reverse geocoding location: %v", err)
			// Continue with empty address - we still have coordinates
		} else {
			address = location.Address
			log.Printf("Reverse geocoded address: %s", address)
		}
	}
	
	// Send a friendly confirmation message with the address
	var confirmMsg string
	if address != "" {
		confirmMsg = fmt.Sprintf("📍 收到您的位置：%s\n\n正在為您查詢附近的垃圾車...", address)
	} else {
		confirmMsg = "📍 收到您的位置\n\n正在為您查詢附近的垃圾車..."
	}
	h.replyMessage(ctx, userID, confirmMsg)
	
	// Search for nearby garbage trucks
	h.searchNearbyGarbageTrucks(ctx, userID, lat, lng, nil)
}

func (h *Handler) handleCommand(ctx context.Context, userID, command string) {
	parts := strings.Split(command, " ")
	cmd := parts[0]

	switch cmd {
	case "/help":
		helpText := `歡迎使用垃圾車助手！

功能說明：
🗑️ 查詢垃圾車：發送位置或輸入地址
⏰ 設定提醒：點擊查詢結果中的「提醒我」按鈕
❤️ 收藏地點：使用 /favorite 指令
📋 查看收藏：使用 /list 指令

使用方式：
📍 分享位置：點擊「+」→「位置」→「即時位置」
💬 輸入地址：「台北市大安區忠孝東路」
🕐 時間查詢：「我晚上七點前在哪裡倒垃圾？」

系統會自動為您找到最近的垃圾車站點！`
		h.replyMessage(ctx, userID, helpText)

	case "/favorite":
		if len(parts) < 2 {
			h.replyMessage(ctx, userID, "請使用：/favorite [地點名稱] [地址]")
			return
		}
		name := parts[1]
		address := strings.Join(parts[2:], " ")
		h.addFavorite(ctx, userID, name, address)

	case "/list":
		h.listFavorites(ctx, userID)

	default:
		h.replyMessage(ctx, userID, "未知指令。請使用 /help 查看可用指令。")
	}
}

func (h *Handler) searchNearbyGarbageTrucks(ctx context.Context, userID string, lat, lng float64, intent *gemini.IntentResult) {
	garbageData, err := h.garbageAdapter.FetchGarbageData(ctx)
	if err != nil {
		log.Printf("Error fetching garbage data: %v", err)
		h.replyMessage(ctx, userID, "抱歉，無法取得垃圾車資料。")
		return
	}

	var nearestStops []*garbage.NearestStop

	if intent != nil && (intent.TimeWindow.From != "" || intent.TimeWindow.To != "") {
		fromTime, toTime, err := h.geminiClient.ParseTimeWindow(intent.TimeWindow)
		if err == nil {
			timeWindow := garbage.TimeWindow{From: fromTime, To: toTime}
			nearestStops, err = h.garbageAdapter.FindStopsInTimeWindow(lat, lng, garbageData, timeWindow, 2000)
		}
	}

	if len(nearestStops) == 0 {
		nearestStops, err = h.garbageAdapter.FindNearestStops(lat, lng, garbageData, 5)
		if err != nil {
			log.Printf("Error finding nearest stops: %v", err)
			h.replyMessage(ctx, userID, "抱歉，無法找到附近的垃圾車站點。")
			return
		}
	}

	if len(nearestStops) == 0 {
		h.replyMessage(ctx, userID, "附近沒有找到垃圾車站點。")
		return
	}

	h.sendGarbageTruckResults(ctx, userID, nearestStops)
}

func (h *Handler) sendGarbageTruckResults(ctx context.Context, userID string, stops []*garbage.NearestStop) {
	if len(stops) == 0 {
		return
	}

	var bubbles []messaging_api.FlexBubble

	for i, stop := range stops {
		if i >= 3 {
			break
		}

		bubble := h.createGarbageTruckBubble(stop)
		bubbles = append(bubbles, bubble)
	}

	carousel := messaging_api.FlexCarousel{
		Contents: bubbles,
	}

	flexMessage := messaging_api.FlexMessage{
		AltText:  "垃圾車查詢結果",
		Contents: &carousel,
	}

	h.sendMessage(ctx, userID, &flexMessage)
}

func (h *Handler) createGarbageTruckBubble(stop *garbage.NearestStop) messaging_api.FlexBubble {
	timeStr := stop.ETA.Format("15:04")
	distanceStr := geo.FormatDistance(stop.Distance)
	directionsURL := h.geoClient.GetDirectionsURL(stop.Stop.Lat, stop.Stop.Lng)

	reminderData := fmt.Sprintf("route=%s&stop=%s&eta=%d", 
		stop.Route.ID, stop.Stop.Name, stop.ETA.Unix())

	body := messaging_api.FlexBox{
		Layout: "vertical",
		Contents: []messaging_api.FlexComponentInterface{
			&messaging_api.FlexText{
				Text:   stop.Stop.Name,
				Weight: "bold",
				Size:   "lg",
			},
			&messaging_api.FlexText{
				Text: fmt.Sprintf("下一班：%s", timeStr),
				Size: "md",
			},
			&messaging_api.FlexText{
				Text:  fmt.Sprintf("距離：%s", distanceStr),
				Size:  "sm",
				Color: "#888888",
			},
			&messaging_api.FlexText{
				Text:  fmt.Sprintf("路線：%s", stop.Route.Name),
				Size:  "sm",
				Color: "#888888",
			},
		},
	}

	footer := messaging_api.FlexBox{
		Layout: "horizontal",
		Contents: []messaging_api.FlexComponentInterface{
			&messaging_api.FlexButton{
				Action: &messaging_api.UriAction{
					Label: "導航",
					Uri:   directionsURL,
				},
			},
			&messaging_api.FlexButton{
				Action: &messaging_api.PostbackAction{
					Label: "提醒我",
					Data:  reminderData,
				},
			},
		},
	}

	return messaging_api.FlexBubble{
		Body:   &body,
		Footer: &footer,
	}
}

func (h *Handler) handlePostbackEvent(ctx context.Context, event webhook.PostbackEvent) {
	var userID string
	switch source := event.Source.(type) {
	case *webhook.UserSource:
		userID = source.UserId
	default:
		return
	}

	data := event.Postback.Data
	params := parsePostbackData(data)

	if routeID, ok := params["route"]; ok {
		stopName := params["stop"]
		etaStr := params["eta"]
		
		eta, err := strconv.ParseInt(etaStr, 10, 64)
		if err != nil {
			h.replyMessage(ctx, userID, "提醒設定失敗：時間格式錯誤")
			return
		}

		reminder := &store.Reminder{
			UserID:         userID,
			StopName:       stopName,
			RouteID:        routeID,
			ETA:            time.Unix(eta, 0),
			AdvanceMinutes: 10,
		}

		err = h.store.CreateReminder(ctx, reminder)
		if err != nil {
			log.Printf("Error creating reminder: %v", err)
			h.replyMessage(ctx, userID, "提醒設定失敗")
			return
		}

		h.replyMessage(ctx, userID, fmt.Sprintf("✅ 已設定提醒！\n將在垃圾車抵達 %s 前 10 分鐘通知您。", stopName))
	}
}

func (h *Handler) addFavorite(ctx context.Context, userID, name, address string) {
	location, err := h.geoClient.GeocodeAddress(ctx, address)
	if err != nil {
		h.replyMessage(ctx, userID, "無法找到該地址的位置資訊")
		return
	}

	favorite := store.Favorite{
		Name:    name,
		Lat:     location.Lat,
		Lng:     location.Lng,
		Address: location.Address,
	}

	err = h.store.AddFavorite(ctx, userID, favorite)
	if err != nil {
		log.Printf("Error adding favorite: %v", err)
		h.replyMessage(ctx, userID, "收藏地點失敗")
		return
	}

	h.replyMessage(ctx, userID, fmt.Sprintf("✅ 已收藏地點：%s", name))
}

func (h *Handler) listFavorites(ctx context.Context, userID string) {
	user, err := h.store.GetUser(ctx, userID)
	if err != nil {
		h.replyMessage(ctx, userID, "無法取得收藏清單")
		return
	}

	if len(user.Favorites) == 0 {
		h.replyMessage(ctx, userID, "您還沒有收藏任何地點")
		return
	}

	var message strings.Builder
	message.WriteString("您的收藏地點：\n\n")
	for i, fav := range user.Favorites {
		message.WriteString(fmt.Sprintf("%d. %s\n   %s\n\n", i+1, fav.Name, fav.Address))
	}

	h.replyMessage(ctx, userID, message.String())
}

func (h *Handler) replyMessage(ctx context.Context, userID, text string) {
	message := messaging_api.TextMessage{
		Text: text,
	}
	h.sendMessage(ctx, userID, &message)
}

func (h *Handler) sendMessage(ctx context.Context, userID string, message messaging_api.MessageInterface) {
	req := &messaging_api.PushMessageRequest{
		To:       userID,
		Messages: []messaging_api.MessageInterface{message},
	}

	_, err := h.messagingAPI.PushMessage(req, "")
	if err != nil {
		log.Printf("Error sending message: %v", err)
	}
}

func (h *Handler) GetMessagingAPI() *messaging_api.MessagingApiAPI {
	return h.messagingAPI
}

func parsePostbackData(data string) map[string]string {
	params := make(map[string]string)
	pairs := strings.Split(data, "&")
	
	for _, pair := range pairs {
		kv := strings.Split(pair, "=")
		if len(kv) == 2 {
			params[kv[0]] = kv[1]
		}
	}
	
	return params
}