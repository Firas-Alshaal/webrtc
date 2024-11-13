package websocket

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/flutter-webrtc/flutter-webrtc-server/pkg/logger"
	"github.com/gorilla/websocket"
)

type WebSocketServerConfig struct {
	Host           string
	Port           int
	CertFile       string
	KeyFile        string
	HTMLRoot       string
	WebSocketPath  string
	TurnServerPath string
}

func DefaultConfig() WebSocketServerConfig {
	return WebSocketServerConfig{
		Host:           "0.0.0.0",
		Port:           8086,
		HTMLRoot:       "web",
		WebSocketPath:  "/ws",
		TurnServerPath: "/api/turn",
	}
}

type WebSocketServer struct {
	handleWebSocket  func(ws *WebSocketConn, request *http.Request)
	handleTurnServer func(writer http.ResponseWriter, request *http.Request)
	// Websocket upgrader
	upgrader websocket.Upgrader
}

func NewWebSocketServer(
	wsHandler func(ws *WebSocketConn, request *http.Request),
	turnServerHandler func(writer http.ResponseWriter, request *http.Request)) *WebSocketServer {
	var server = &WebSocketServer{
		handleWebSocket:  wsHandler,
		handleTurnServer: turnServerHandler,
	}
	server.upgrader = websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
	}
	return server
}

func (server *WebSocketServer) handleWebSocketRequest(writer http.ResponseWriter, request *http.Request) {
	responseHeader := http.Header{}
	//responseHeader.Add("Sec-WebSocket-Protocol", "protoo")
	socket, err := server.upgrader.Upgrade(writer, request, responseHeader)
	if err != nil {
		logger.Panicf("%v", err)
	}
	wsTransport := NewWebSocketConn(socket)
	server.handleWebSocket(wsTransport, request)
	wsTransport.ReadMessage()
}

func (server *WebSocketServer) handleTurnServerRequest(writer http.ResponseWriter, request *http.Request) {
	server.handleTurnServer(writer, request)
}

// Bind .
func (server *WebSocketServer) Bind(cfg WebSocketServerConfig) {
	// Websocket handle func
	http.HandleFunc(cfg.WebSocketPath, server.handleWebSocketRequest)
	http.HandleFunc(cfg.TurnServerPath, server.handleTurnServerRequest)
	http.Handle("/", http.FileServer(http.Dir(cfg.HTMLRoot)))

	// Read the server certificate and key
	sslCert := cfg.CertFile // Path to your .crt file
	sslKey := cfg.KeyFile   // Path to your .key file

	// Read the CA certificate if you want to trust your self-signed cert
	caCert, err := os.ReadFile("configs/rtccerts/webrtc.pem")
	if err != nil {
		log.Fatalf("Failed to read CA certificate: %v", err)
	}

	// Create a CertPool and add your custom CA
	certPool := x509.NewCertPool()
	certPool.AppendCertsFromPEM(caCert)

	// Create a custom TLS configuration to skip verification
	tlsConfig := &tls.Config{
		InsecureSkipVerify: true,     // Skip SSL certificate verification
		RootCAs:            certPool, // Add the custom CA (if using self-signed cert)
	}

	// Create a custom HTTP server with TLS
	serverTLS := &http.Server{
		Addr:      fmt.Sprintf("%s:%d", cfg.Host, cfg.Port),
		TLSConfig: tlsConfig,
	}

	logger.Infof("Flutter WebRTC Server listening on: %s:%d", cfg.Host, cfg.Port)
	// http.ListenAndServe(cfg.Host+":"+strconv.Itoa(cfg.Port), nil)
	panic(serverTLS.ListenAndServeTLS(sslCert, sslKey))
}
