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
	log.Printf("Received webhook request from %s", r.RemoteAddr)
	
	cb, err := webhook.ParseRequest(h.channelSecret, r)
	if err != nil {
		log.Printf("Cannot parse request: %v", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	log.Printf("Successfully parsed webhook, processing %d events", len(cb.Events))

	for i, event := range cb.Events {
		log.Printf("Processing event %d/%d, type: %T", i+1, len(cb.Events), event)
		
		switch e := event.(type) {
		case webhook.MessageEvent:
			log.Printf("Handling MessageEvent")
			h.handleMessageEvent(r.Context(), e)
		case webhook.PostbackEvent:
			log.Printf("Handling PostbackEvent")
			h.handlePostbackEvent(r.Context(), e)
		default:
			log.Printf("Unhandled event type: %T", event)
		}
	}

	log.Printf("Webhook processing completed successfully")
	w.WriteHeader(http.StatusOK)
}

func (h *Handler) getUserID(source webhook.SourceInterface) string {
	switch s := source.(type) {
	case webhook.UserSource:
		log.Printf("User source detected, User ID: %s", s.UserId)
		return s.UserId
	case webhook.GroupSource:
		log.Printf("Group source detected, Group ID: %s", s.GroupId)
		// For group messages, we could potentially handle them differently
		// For now, we ignore group messages
		return ""
	case webhook.RoomSource:
		log.Printf("Room source detected, Room ID: %s", s.RoomId)
		// For room messages, we could potentially handle them differently
		// For now, we ignore room messages
		return ""
	default:
		log.Printf("Unknown source type: %T", source)
		return ""
	}
}

func (h *Handler) handleMessageEvent(ctx context.Context, event webhook.MessageEvent) {
	log.Printf("Processing MessageEvent")
	log.Printf("Message type: %T", event.Message)
	log.Printf("Source type: %T", event.Source)
	
	// First check if we can handle this message type
	switch message := event.Message.(type) {
	case webhook.TextMessageContent:
		log.Printf("Text message received: %s", message.Text)
		// Now get the user ID for text messages
		userID := h.getUserID(event.Source)
		if userID == "" {
			log.Printf("Cannot get user ID from source type %T, ignoring text message", event.Source)
			return
		}
		h.handleTextMessage(ctx, userID, message.Text)
		
	case webhook.LocationMessageContent:
		log.Printf("Location message received: lat=%f, lng=%f, address=%s", message.Latitude, message.Longitude, message.Address)
		// Now get the user ID for location messages
		userID := h.getUserID(event.Source)
		if userID == "" {
			log.Printf("Cannot get user ID from source type %T, ignoring location message", event.Source)
			return
		}
		h.handleLocationMessage(ctx, userID, message.Latitude, message.Longitude, message.Address)
		
	default:
		log.Printf("Unhandled message type: %T", event.Message)
	}
}

func (h *Handler) handleTextMessage(ctx context.Context, userID, text string) {
	log.Printf("Processing text message from user %s: %s", userID, text)
	
	if strings.HasPrefix(text, "/") {
		log.Printf("Command detected: %s", text)
		h.handleCommand(ctx, userID, text)
		return
	}

	// Handle common greetings
	lowerText := strings.ToLower(strings.TrimSpace(text))
	if lowerText == "hi" || lowerText == "hello" || lowerText == "你好" || lowerText == "哈囉" {
		log.Printf("Greeting detected: %s", text)
		welcomeMsg := `👋 您好！歡迎使用垃圾車助手！

🚀 快速開始：
📍 點擊下方「+」按鈕 → 選擇「位置」→「即時位置」
💬 或直接輸入地址，例如：「台北市信義區」

我會幫您找到最近的垃圾車站點和時間！

輸入 /help 查看更多功能`
		h.replyMessage(ctx, userID, welcomeMsg)
		return
	}

	log.Printf("Analyzing intent for text: %s", text)
	intent, err := h.geminiClient.AnalyzeIntent(ctx, text)
	if err != nil {
		log.Printf("Error analyzing intent for user %s: %v", userID, err)
		h.replyMessage(ctx, userID, "抱歉，我無法理解您的訊息。\n\n💡 您可以：\n📍 分享您的位置\n💬 輸入地址\n❓ 輸入 /help 查看使用說明")
		return
	}
	
	log.Printf("Intent analysis result: %+v", intent)

	// 首先檢查是否是收藏地點名稱
	favorite := h.findUserFavoriteByName(ctx, userID, text)
	if favorite != nil {
		log.Printf("Found favorite location '%s' for user %s: lat=%f, lng=%f", text, userID, favorite.Lat, favorite.Lng)
		h.searchNearbyGarbageTrucks(ctx, userID, favorite.Lat, favorite.Lng, intent)
		return
	}
	
	// 檢查是否有時間窗口查詢但沒有地址  
	if intent != nil && (intent.TimeWindow.From != "" || intent.TimeWindow.To != "") && intent.District == "" {
		log.Printf("Time window query detected without specific location: %s", text)
		h.handleTimeQueryWithoutLocation(ctx, userID, intent)
		return
	}

	// 嘗試多種方式提取地址
	var addressToGeocode string
	
	// 方法1：使用 Gemini 解析的 District
	if intent.District != "" {
		addressToGeocode = intent.District
		log.Printf("Using district from intent: %s", addressToGeocode)
	} else {
		// 方法2：使用 Gemini 提取地址
		extractedLocation, err := h.geminiClient.ExtractLocationFromText(ctx, text)
		if err == nil && extractedLocation != "" {
			addressToGeocode = extractedLocation
			log.Printf("Extracted location from text: %s", addressToGeocode)
		} else {
			// 方法3：直接使用原始文字作為地址
			addressToGeocode = text
			log.Printf("Using original text as address: %s", addressToGeocode)
		}
	}
	
	// 進行地理編碼
	log.Printf("Geocoding address: %s", addressToGeocode)
	location, err := h.geoClient.GeocodeAddress(ctx, addressToGeocode)
	if err != nil {
		log.Printf("Error geocoding address '%s' for user %s: %v", addressToGeocode, userID, err)
		h.replyMessage(ctx, userID, fmt.Sprintf("抱歉，我找不到「%s」的位置資訊。\n\n💡 請嘗試：\n📍 分享您的位置\n💬 輸入更具體的地址（如：台北市信義區忠孝東路）", text))
		return
	}
	
	log.Printf("Geocoded successfully: %+v", location)
	h.searchNearbyGarbageTrucks(ctx, userID, location.Lat, location.Lng, intent)
}

func (h *Handler) handleTimeQueryWithoutLocation(ctx context.Context, userID string, intent *gemini.IntentResult) {
	fromTime, toTime, err := h.geminiClient.ParseTimeWindow(intent.TimeWindow)
	if err != nil {
		log.Printf("Error parsing time window: %v", err)
		h.replyMessage(ctx, userID, "抱歉，無法理解您指定的時間。")
		return
	}

	var timeDesc string
	if !toTime.IsZero() {
		timeDesc = fmt.Sprintf("%s前", toTime.Format("15:04"))
	} else if !fromTime.IsZero() {
		timeDesc = fmt.Sprintf("%s後", fromTime.Format("15:04"))
	} else {
		timeDesc = "指定時間內"
	}

	// 檢查用戶是否有收藏地點
	user, err := h.store.GetUser(ctx, userID)
	if err == nil && len(user.Favorites) > 0 {
		// 用戶有收藏地點，提供選項
		message := fmt.Sprintf("🕐 您想查詢%s的垃圾車資訊\n\n您可以：\n", timeDesc)
		message += "📍 分享您的即時位置\n"
		message += "❤️ 選擇收藏地點：\n"
		
		for i, fav := range user.Favorites {
			if i >= 3 { // 限制顯示前3個收藏
				break
			}
			message += fmt.Sprintf("• %s\n", fav.Name)
		}
		message += "\n請分享位置或輸入收藏地點名稱"
		h.replyMessage(ctx, userID, message)
	} else {
		// 用戶沒有收藏地點
		message := fmt.Sprintf("🕐 您想查詢%s的垃圾車資訊\n\n", timeDesc)
		message += "請提供位置資訊：\n"
		message += "📍 分享您的即時位置，或\n"
		message += "💬 輸入具體地址\n\n"
		message += "💡 您也可以使用 `/favorite 家 台北市大安區xxx` 來收藏常用地點"
		h.replyMessage(ctx, userID, message)
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
	log.Printf("Searching nearby garbage trucks for user %s at coordinates: lat=%f, lng=%f", userID, lat, lng)
	
	garbageData, err := h.garbageAdapter.FetchGarbageData(ctx)
	if err != nil {
		log.Printf("Error fetching garbage data for user %s: %v", userID, err)
		h.replyMessage(ctx, userID, "抱歉，無法取得垃圾車資料。")
		return
	}
	
	log.Printf("Successfully fetched garbage data, %d collection points available", len(garbageData.Result.Results))

	var nearestStops []*garbage.NearestStop

	if intent != nil && (intent.TimeWindow.From != "" || intent.TimeWindow.To != "") {
		log.Printf("Time window query detected: from=%s, to=%s", intent.TimeWindow.From, intent.TimeWindow.To)
		fromTime, toTime, err := h.geminiClient.ParseTimeWindow(intent.TimeWindow)
		if err == nil {
			log.Printf("Parsed time window: from=%v, to=%v", fromTime, toTime)
			timeWindow := garbage.TimeWindow{From: fromTime, To: toTime}
			nearestStops, err = h.garbageAdapter.FindStopsInTimeWindow(lat, lng, garbageData, timeWindow, 2000)
			log.Printf("Found %d stops in time window", len(nearestStops))
		} else {
			log.Printf("Error parsing time window: %v", err)
		}
	}

	if len(nearestStops) == 0 {
		log.Printf("No stops found in time window, searching for nearest stops")
		nearestStops, err = h.garbageAdapter.FindNearestStops(lat, lng, garbageData, 5)
		if err != nil {
			log.Printf("Error finding nearest stops for user %s: %v", userID, err)
			h.replyMessage(ctx, userID, "抱歉，無法找到附近的垃圾車站點。")
			return
		}
		log.Printf("Found %d nearest stops", len(nearestStops))
	}

	if len(nearestStops) == 0 {
		log.Printf("No garbage truck stops found for user %s at coordinates lat=%f, lng=%f", userID, lat, lng)
		h.replyMessage(ctx, userID, "附近沒有找到垃圾車站點。")
		return
	}

	log.Printf("Sending %d garbage truck results to user %s", len(nearestStops), userID)
	h.sendGarbageTruckResults(ctx, userID, nearestStops)
}

func (h *Handler) sendGarbageTruckResults(ctx context.Context, userID string, stops []*garbage.NearestStop) {
	log.Printf("Preparing to send garbage truck results to user %s", userID)
	
	if len(stops) == 0 {
		log.Printf("No stops to send to user %s", userID)
		return
	}

	var bubbles []messaging_api.FlexBubble

	for i, stop := range stops {
		if i >= 3 {
			log.Printf("Limiting results to first 3 stops")
			break
		}

		log.Printf("Creating bubble for stop %d: %s", i+1, stop.Stop.Name)
		bubble := h.createGarbageTruckBubble(stop)
		bubbles = append(bubbles, bubble)
	}

	log.Printf("Created %d bubbles for user %s", len(bubbles), userID)
	
	carousel := messaging_api.FlexCarousel{
		Contents: bubbles,
	}

	flexMessage := messaging_api.FlexMessage{
		AltText:  "垃圾車查詢結果",
		Contents: &carousel,
	}

	log.Printf("Sending flex message with %d bubbles to user %s", len(bubbles), userID)
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
	log.Printf("Processing PostbackEvent")
	log.Printf("Source type: %T", event.Source)
	log.Printf("Postback data: %s", event.Postback.Data)
	
	userID := h.getUserID(event.Source)
	if userID == "" {
		log.Printf("Cannot get user ID from source type %T, ignoring postback event", event.Source)
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

		etaTime := time.Unix(eta, 0)
		notificationTime := etaTime.Add(-10 * time.Minute)
		
		log.Printf("Creating reminder for user %s: stop=%s, ETA=%s, notificationTime=%s", 
			userID, stopName, etaTime.Format("2006-01-02 15:04:05"), notificationTime.Format("2006-01-02 15:04:05"))
		
		reminder := &store.Reminder{
			UserID:         userID,
			StopName:       stopName,
			RouteID:        routeID,
			ETA:            etaTime,
			AdvanceMinutes: 10,
		}

		err = h.store.CreateReminder(ctx, reminder)
		if err != nil {
			log.Printf("Error creating reminder: %v", err)
			h.replyMessage(ctx, userID, "提醒設定失敗")
			return
		}

		log.Printf("Successfully created reminder for user %s, will notify at %s", userID, notificationTime.Format("2006-01-02 15:04:05"))
		h.replyMessage(ctx, userID, fmt.Sprintf("✅ 已設定提醒！\n將在垃圾車抵達 %s 前 10 分鐘通知您。", stopName))
	}
}

func (h *Handler) findUserFavoriteByName(ctx context.Context, userID, name string) *store.Favorite {
	user, err := h.store.GetUser(ctx, userID)
	if err != nil {
		log.Printf("Error getting user %s: %v", userID, err)
		return nil
	}

	// 進行模糊匹配收藏地點名稱
	lowerName := strings.ToLower(strings.TrimSpace(name))
	for _, fav := range user.Favorites {
		lowerFavName := strings.ToLower(strings.TrimSpace(fav.Name))
		// 完全匹配或包含匹配
		if lowerFavName == lowerName || strings.Contains(lowerFavName, lowerName) || strings.Contains(lowerName, lowerFavName) {
			return &fav
		}
	}
	return nil
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
	log.Printf("Sending reply to user %s: %s", userID, text)
	message := messaging_api.TextMessage{
		Text: text,
	}
	h.sendMessage(ctx, userID, &message)
}

func (h *Handler) sendMessage(ctx context.Context, userID string, message messaging_api.MessageInterface) {
	log.Printf("Attempting to send message to user: %s", userID)
	
	req := &messaging_api.PushMessageRequest{
		To:       userID,
		Messages: []messaging_api.MessageInterface{message},
	}

	log.Printf("Calling LINE Messaging API...")
	resp, err := h.messagingAPI.PushMessage(req, "")
	if err != nil {
		log.Printf("Error sending message to user %s: %v", userID, err)
		return
	}
	
	log.Printf("Message sent successfully to user %s. Response: %+v", userID, resp)
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