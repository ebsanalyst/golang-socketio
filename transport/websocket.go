package transport

import (
	"errors"
	"github.com/gorilla/websocket"
	"io/ioutil"
	"crypto/tls"
	"net/http"
	"time"
)

const (
	upgradeFailed     = "Upgrade failed: "

	WsDefaultPingInterval   = 30 * time.Second
	WsDefaultPingTimeout    = 60 * time.Second
	WsDefaultReceiveTimeout = 60 * time.Second
	WsDefaultSendTimeout    = 60 * time.Second
	WsDefaultBufferSize     = 1024 * 32
)

var (
	ErrorBinaryMessage     = errors.New("Binary messages are not supported")
	ErrorBadBuffer         = errors.New("Buffer error")
	ErrorPacketWrong       = errors.New("Wrong packet type error")
	ErrorMethodNotAllowed  = errors.New("Method not allowed")
	ErrorHttpUpgradeFailed = errors.New("Http upgrade failed")
)

type WebsocketConnection struct {
	socket    *websocket.Conn
	transport *WebsocketTransport
}

type WebsocketTransportParams struct {
	Headers         http.Header
	TLSClientConfig *tls.Config
}
func (wsc *WebsocketConnection) GetMessage() (message string, err error) {
	wsc.socket.SetReadDeadline(time.Now().Add(wsc.transport.ReceiveTimeout))
	msgType, reader, err := wsc.socket.NextReader()
	if err != nil {
		return "", err
	}

	//support only text messages exchange
	if msgType != websocket.TextMessage {
		return "", ErrorBinaryMessage
	}

	data, err := ioutil.ReadAll(reader)
	if err != nil {
		return "", ErrorBadBuffer
	}
	text := string(data)

	//empty messages are not allowed
	if len(text) == 0 {
		return "", ErrorPacketWrong
	}

	return text, nil
}

func (wsc *WebsocketConnection) WriteMessage(message string) error {
	wsc.socket.SetWriteDeadline(time.Now().Add(wsc.transport.SendTimeout))
	writer, err := wsc.socket.NextWriter(websocket.TextMessage)
	if err != nil {
		return err
	}

	if _, err := writer.Write([]byte(message)); err != nil {
		return err
	}
	if err := writer.Close(); err != nil {
		return err
	}
	return nil
}

func (wsc *WebsocketConnection) Close() {
	wsc.socket.Close()
}

func (wsc *WebsocketConnection) PingParams() (interval, timeout time.Duration) {
	return wsc.transport.PingInterval, wsc.transport.PingTimeout
}

type WebsocketTransport struct {
	PingInterval     time.Duration
	PingTimeout      time.Duration
	ReceiveTimeout   time.Duration
	SendTimeout      time.Duration
	BufferSize       int
        Headers          http.Header
	TLSClientConfig  *tls.Config
}

// Connect to the given url
func (t *WebsocketTransport) Connect(url string) (Connection, error) {
	dialer := websocket.Dialer{TLSClientConfig: t.TLSClientConfig}
	socket, _, err := dialer.Dial(url, t.Headers)
	if err != nil {
		return nil, err
	}
	return &WebsocketConnection{socket, t}, nil
}

// HandleConnection
func (t *WebsocketTransport) HandleConnection(w http.ResponseWriter, r *http.Request) (Connection, error) {
	if r.Method != http.MethodGet {
		http.Error(w, upgradeFailed+errMethodNotAllowed.Error(), http.StatusServiceUnavailable)
		return nil, errMethodNotAllowed
	}

	socket, err := (&websocket.Upgrader{
		ReadBufferSize:  t.BufferSize,
		WriteBufferSize: t.BufferSize,
	}).Upgrade(w, r, nil)
	if err != nil {
		http.Error(w, upgradeFailed+err.Error(), http.StatusServiceUnavailable)
		return nil, errHttpUpgradeFailed
	}

	return &WebsocketConnection{socket, t}, nil
}

/**
Websocket connection do not require any additional processing
*/
func (wst *WebsocketTransport) Serve(w http.ResponseWriter, r *http.Request) {}

/**
Returns websocket connection with default params
*/
func GeDefaultWebsocketTransport() *WebsocketTransport {
	return &WebsocketTransport{
		PingInterval:   WsDefaultPingInterval,
		PingTimeout:    WsDefaultPingTimeout,
		ReceiveTimeout: WsDefaultReceiveTimeout,
		SendTimeout:    WsDefaultSendTimeout,
		BufferSize:     WsDefaultBufferSize,
	}
}


func TlsWebsocketTransport(Headers  http.Header, TLSClientConfig *tls.Config ) *WebsocketTransport {
	tr := GetDefaultWebsocketTransport()
	tr.Headers = Headers
	tr.TLSClientConfig = TLSClientConfig
	return tr
}
