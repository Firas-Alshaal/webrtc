package main

import (
	"github.com/flutter-webrtc/flutter-webrtc-server/pkg/signaler"
	"github.com/flutter-webrtc/flutter-webrtc-server/pkg/turn"
	"github.com/flutter-webrtc/flutter-webrtc-server/pkg/websocket"
	"gopkg.in/ini.v1"
	"context"
	"encoding/json"
	"log"
	"fmt"
	"net/http"
    // "github.com/google/uuid"

	firebase "firebase.google.com/go"
	"firebase.google.com/go/messaging"
	"github.com/sideshow/apns2"
	"github.com/sideshow/apns2/token"
	"google.golang.org/api/option"

)

// User represents the user data
type User struct {
	UUID          string `json:"uuid"`
	PhoneNumber   string `json:"caller_id"`
	Name          string `json:"caller_name"`
	FirebaseToken string `json:"fcm_token"`
}

// Message represents the message data
type Message struct {
	UUID        string `json:"uuid"`
	Id        string `json:"id"`
	CallerID    string `json:"caller_id"`
	CallerName  string `json:"caller_name"`
	CallerIDType string `json:"caller_id_type"`
	HasVideo    bool `json:"has_video"`
	RoomID      string `json:"room_id"`
	FCMToken    string `json:"fcm_token"`
	SenderFCMToken    string `json:"sender_fcm_token"`
	VoIPToken      string `json:"voip_token"`
}

type CancelMessage struct {
	UUID string `json:"uuid"`
	FCMToken       string `json:"fcm_token"`
	Id       string `json:"id"`
}

type DeclineMessage struct {
	UUID     string `json:"uuid"`
	FCMToken string `json:"fcm_token"`
}


type IncomingPayload struct {
	Extra                 ExtraData           `json:"extra"`
	NameCaller            string              `json:"nameCaller"`
	AppName               string              `json:"appName"`
	Type                  int                 `json:"type"`
	IOS                   IOSData             `json:"ios"`
	Handle                string              `json:"handle"`
	Duration              int                 `json:"duration"`
	Avatar                string              `json:"avatar"`
	UUID                  string              `json:"uuid"`
	ID                    string              `json:"id"`
	Android               AndroidData         `json:"android"`
	MissedCallNotification MissedCallNotif    `json:"missedCallNotification"`
	NormalHandle          interface{}         `json:"normalHandle"`
	TextDecline           string              `json:"textDecline"`
}

type ExtraData struct {
	HasVideo       string   `json:"has_video"`
	UserID         string `json:"userId"`
	SenderFCMToken string `json:"sender_fcm_token"`
	UUID           string `json:"uuid"`
}

type IOSData struct {
	AudioSessionMode                 string  `json:"audioSessionMode"`
	MaximumCallsPerCallGroup         int     `json:"maximumCallsPerCallGroup"`
	IconName                         string  `json:"iconName"`
	SupportsVideo                    bool    `json:"supportsVideo"`
	MaximumCallGroups                int     `json:"maximumCallGroups"`
	SupportsUngrouping               bool    `json:"supportsUngrouping"`
	ConfigureAudioSession            bool    `json:"configureAudioSession"`
	HandleType                       string  `json:"handleType"`
	SupportsDTMF                     bool    `json:"supportsDTMF"`
	SupportsHolding                  bool    `json:"supportsHolding"`
	AudioSessionPreferredSampleRate  float64 `json:"audioSessionPreferredSampleRate"`
	AudioSessionPreferredIOBufferDur float64 `json:"audioSessionPreferredIOBufferDuration"`
	AudioSessionActive               bool    `json:"audioSessionActive"`
	IncludesCallsInRecents           bool    `json:"includesCallsInRecents"`
	SupportsGrouping                 bool    `json:"supportsGrouping"`
	RingtonePath                     string  `json:"ringtonePath"`
}

type AndroidData struct {
	IsShowFullLockedScreen           interface{} `json:"isShowFullLockedScreen"`
	RingtonePath                     string      `json:"ringtonePath"`
	BackgroundColor                  string      `json:"backgroundColor"`
	BackgroundUrl                    string      `json:"backgroundUrl"`
	TextColor                        string      `json:"textColor"`
	ActionColor                      string      `json:"actionColor"`
	MissedCallNotificationChannelName interface{} `json:"missedCallNotificationChannelName"`
	IsCustomSmallExNotification      interface{} `json:"isCustomSmallExNotification"`
	IncomingCallNotificationChannelName interface{} `json:"incomingCallNotificationChannelName"`
	IsCustomNotification             int         `json:"isCustomNotification"`
	IsShowLogo                       int         `json:"isShowLogo"`
	IsShowCallID                     interface{} `json:"isShowCallID"`
}

type MissedCallNotif struct {
	ID             interface{} `json:"id"`
	ShowNotification int         `json:"showNotification"`
	Count          interface{} `json:"count"`
	Subtitle       string      `json:"subtitle"`
	CallbackText   string      `json:"callbackText"`
	IsShowCallback int         `json:"isShowCallback"`
}

type CustomTransport struct {
	http.RoundTripper
}

func (t *CustomTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req.Header.Set("apns-push-type", "voip")
	req.Header.Set("apns-expiration", "0")
	req.Header.Set("apns-priority", "10")
	req.Header.Set("apns-topic", "com.atlascrisis.webeoc.voip")

	return t.RoundTripper.RoundTrip(req)
}

func main() {
	
	opt := option.WithCredentialsFile("/Users/atlas/flutter-webrtc-server/serviceAccountKey.json")
	configFirebase := &firebase.Config{ProjectID: "bubbly-shield-408505"}

	app, err := firebase.NewApp(context.Background(), configFirebase, opt)
	if err != nil {
		log.Fatalf("error initializing app: %v", err)
	}


	authKey, err := token.AuthKeyFromFile("/Users/atlas/flutter-webrtc-server/AuthKey_B695C9HCAN.p8")
    if err != nil {
        log.Fatalf("AuthKey error: %v", err)
    }

	apnsToken := &token.Token{
		AuthKey:   authKey,
		KeyID:     "B695C9HCAN",
		TeamID:    "87U75J6869",
	}
	
	clientVoip := apns2.NewTokenClient(apnsToken).Development() // Use .Production() for production
	clientVoip.HTTPClient.Transport = &CustomTransport{RoundTripper: clientVoip.HTTPClient.Transport}


	client, err := app.Messaging(context.Background())
	if err != nil {
		log.Fatalf("error getting Firebase Messaging client: %v", err)
	}

	http.HandleFunc("/send-message", func(w http.ResponseWriter, r *http.Request) {

		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		// Handle preflight requests
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}


		// Parse JSON request body
		var message Message
		errMessage := json.NewDecoder(r.Body).Decode(&message)
		if errMessage != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		// hasVideo := 0
		// if message.HasVideo {
		// 	hasVideo = 1
		// }

		// var hasVideoStr string
		// if message.HasVideo {
		// 	hasVideoStr = "true"
		// } else {
		// 	hasVideoStr = "false"
		// }

		// uuid := uuid.New().String()


		payload := fmt.Sprintf(`{
			"extra": {
			  "has_video": "true",
			  "userId": "%s",
			  "sender_fcm_token": "%s",
			  "uuid": "%s"
			},
			"nameCaller": "%s",
			"appName": "Atlas",
			"type": 1,
			"ios": {
			  "audioSessionMode": "default",
			  "maximumCallsPerCallGroup": 1,
			  "iconName": "CallKitLogo",
			  "supportsVideo": true,
			  "maximumCallGroups": 2,
			  "supportsUngrouping": false,
			  "configureAudioSession": true,
			  "handleType": "generic",
			  "supportsDTMF": true,
			  "supportsHolding": true,
			  "audioSessionPreferredSampleRate": 44100.0,
			  "audioSessionPreferredIOBufferDuration": 0.005,
			  "audioSessionActive": true,
			  "includesCallsInRecents": true,
			  "supportsGrouping": false,
			  "ringtonePath": "system_ringtone_default"
			},
			"handle": "0123456789",
			"avatar": "https://t4.ftcdn.net/jpg/05/89/93/27/360_F_589932782_vQAEAZhHnq1QCGu5ikwrYaQD0Mmurm0N.jpg",
			"uuid": "%s",
			"id": "%s",
			"android": {
			  "isShowFullLockedScreen": false,
			  "ringtonePath": "system_ringtone_default",
			  "backgroundColor": "#0955fa",
			  "backgroundUrl": "assets/test.png",
			  "textColor": "#ffffff",
			  "actionColor": "#4CAF50",
			  "missedCallNotificationChannelName": null,
			  "isCustomSmallExNotification": null,
			  "incomingCallNotificationChannelName": null,
			  "isCustomNotification": 1,
			  "isShowLogo": 0,
			  "isShowCallID": null
			},
			"duration": 30000,
			"missedCallNotification": {
			  "id": null,
			  "showNotification": 0,
			  "count": null,
			  "subtitle": "Missed call",
			  "callbackText": "Call back",
			  "isShowCallback": 0
			},
			"normalHandle": null,
			"textDecline": "Decline",
			"textAccept": "Accept"
		  }`, message.CallerID, message.SenderFCMToken, message.UUID, message.CallerName, message.Id, message.Id)


		  var incomingData IncomingPayload
		  err := json.Unmarshal([]byte(payload), &incomingData)
		  if err != nil {
			  log.Fatalf("Error parsing JSON: %v", err)
		  }

		  if message.VoIPToken != "" {
			notification := &apns2.Notification{
				Topic: "com.atlascrisis.webeoc.voip",
				Payload: map[string]interface{}{
						"extra": incomingData.Extra,
						"nameCaller": incomingData.NameCaller,
						"appName": incomingData.AppName,
						"type": incomingData.Type,
						"ios": incomingData.IOS,
						"handle": incomingData.Handle,
						"duration": incomingData.Duration,
						"avatar": incomingData.Avatar,
						"uuid": incomingData.UUID,
						"id": incomingData.UUID,
						"android": incomingData.Android,
						"missedCallNotification": incomingData.MissedCallNotification,
						"normalHandle": incomingData.NormalHandle,
						"textDecline": incomingData.TextDecline,
				},
				DeviceToken: message.VoIPToken,
				PushType: 	apns2.PushTypeBackground,
			}
	
			res, err := clientVoip.Push(notification)
			if err!= nil {
				log.Printf("Error sending VoIP notification: %v", err)
				http.Error(w, "Error sending VoIP notification", http.StatusInternalServerError)
				return
			}
	
			w.WriteHeader(http.StatusOK)
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(res)
			fmt.Println(res)
		} else {
				// Create a new FCM message
				msg := &messaging.Message{
					Data: map[string]string{
						"uuid":          message.UUID,
						"caller_id":     message.CallerID,
						"caller_name":   message.CallerName,
						"caller_id_type": message.CallerIDType,
						"has_video":     fmt.Sprintf("%t", message.HasVideo),
						"room_id":       message.RoomID,
						"sender_fcm_token":       message.SenderFCMToken,
						"id":       message.Id,
					},
					Token: message.FCMToken,
				}

				// Send the message
				response, errMessage := client.Send(context.Background(), msg)
				if errMessage != nil {
					log.Printf("error sending message: %v\n", errMessage)
					http.Error(w, "Error sending message", http.StatusInternalServerError)
					return
				}

				// Write response
				w.WriteHeader(http.StatusOK)
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(response)
		}
	})

	http.HandleFunc("/cancel-message", func(w http.ResponseWriter, r *http.Request) {


		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		// Handle preflight requests
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}



		var cancelMessage CancelMessage
		err := json.NewDecoder(r.Body).Decode(&cancelMessage)
		if err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		msg := &messaging.Message{
			Data: map[string]string{
				"type":            "cancel_call",
				"uuid": cancelMessage.UUID,
				"id": cancelMessage.Id,
			},
			Token: cancelMessage.FCMToken,
		}

		response, err := client.Send(context.Background(), msg)
		if err != nil {
			log.Printf("error sending cancel message: %v\n", err)
			http.Error(w, "Error sending cancel message", http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusOK)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	})


	http.HandleFunc("/decline-call", func(w http.ResponseWriter, r *http.Request) {


		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		// Handle preflight requests
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}



		var declineMessage DeclineMessage
		err := json.NewDecoder(r.Body).Decode(&declineMessage)
		if err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		msg := &messaging.Message{
			Data: map[string]string{
				"type": "decline_call",
				"uuid": declineMessage.UUID,
			},
			Token: declineMessage.FCMToken,
		}

		response, err := client.Send(context.Background(), msg)
		if err != nil {
			log.Printf("error sending decline message: %v\n", err)
			http.Error(w, "Error sending decline message", http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusOK)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	})

	
	cfg, err := ini.Load("configs/config.ini")

	publicIP := cfg.Section("turn").Key("public_ip").String()
	stunPort, err := cfg.Section("turn").Key("port").Int()
	if err != nil {
		stunPort = 3478
	}
	realm := cfg.Section("turn").Key("realm").String()

	turnConfig := turn.DefaultConfig()
	turnConfig.PublicIP = publicIP
	turnConfig.Port = stunPort
	turnConfig.Realm = realm
	turn := turn.NewTurnServer(turnConfig)

	signaler := signaler.NewSignaler(turn)
	wsServer := websocket.NewWebSocketServer(signaler.HandleNewWebSocket, signaler.HandleTurnServerCredentials)

	sslCert := cfg.Section("general").Key("cert").String()
	sslKey := cfg.Section("general").Key("key").String()
	bindAddress := cfg.Section("general").Key("bind").String()

	port, err := cfg.Section("general").Key("port").Int()
	if err != nil {
		port = 8086
	}

	htmlRoot := cfg.Section("general").Key("html_root").String()

	config := websocket.DefaultConfig()
	config.Host = bindAddress
	config.Port = port
	config.CertFile = sslCert
	config.KeyFile = sslKey
	config.HTMLRoot = htmlRoot

	wsServer.Bind(config)
}
