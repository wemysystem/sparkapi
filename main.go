package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/url"
	"time"

	"github.com/gorilla/websocket"
)

type WsParam struct {
	APPID     string
	APIKey    string
	APISecret string
	Host      string
	Path      string
	GptURL    string
}

func NewWsParam(APPID, APIKey, APISecret, gptURL string) *WsParam {
	u, _ := url.Parse(gptURL)
	return &WsParam{
		APPID:     APPID,
		APIKey:    APIKey,
		APISecret: APISecret,
		Host:      u.Host,
		Path:      u.Path,
		GptURL:    gptURL,
	}
}

func (p *WsParam) createURL() string {
	now := time.Now().UTC()
	date := now.Format(time.RFC1123)

	signatureOrigin := fmt.Sprintf("host: %s\ndate: %s\nGET %s HTTP/1.1", p.Host, date, p.Path)
	h := hmac.New(sha256.New, []byte(p.APISecret))
	h.Write([]byte(signatureOrigin))
	signature := base64.StdEncoding.EncodeToString(h.Sum(nil))

	authorizationOrigin := fmt.Sprintf(`api_key="%s", algorithm="hmac-sha256", headers="host date request-line", signature="%s"`, p.APIKey, signature)
	authorization := base64.StdEncoding.EncodeToString([]byte(authorizationOrigin))

	v := url.Values{}
	v.Set("authorization", authorization)
	v.Set("date", date)
	v.Set("host", p.Host)

	_url := fmt.Sprintf("%s?%s", p.GptURL, v.Encode())
	return _url
}

func main() {
	appID := ""
	apiKey := ""
	apiSecret := ""
	gptURL := "ws://spark-api.xf-yun.com/v1.1/chat"
	question := "你是谁？你能做什么？"

	wsParam := NewWsParam(appID, apiKey, apiSecret, gptURL)
	_url := wsParam.createURL()

	conn, _, err := websocket.DefaultDialer.Dial(_url, nil)
	if err != nil {
		fmt.Println("### error:", err)
		return
	}

	defer func(conn *websocket.Conn) {
		err := conn.Close()
		if err != nil {
			fmt.Println("Error occurred while closing connection:", err)
		}
	}(conn)

	done := make(chan struct{})

	go func() {
		defer close(done)
		for {
			_, message, err := conn.ReadMessage()
			if err != nil {
				fmt.Println("### closed ###")
				return
			}
			onMessage(message)
		}
	}()

	data := genParams(wsParam.APPID, question)
	err = conn.WriteJSON(data)
	if err != nil {
		fmt.Println("### error:", err)
		return
	}

	<-done
}

func onMessage(message []byte) {
	var data map[string]interface{}
	err := json.Unmarshal(message, &data)
	if err != nil {
		fmt.Println("### error:", err)
		return
	}

	header := data["header"].(map[string]interface{})
	code := int(header["code"].(float64))
	if code != 0 {
		fmt.Printf("请求错误: %d, %s\n", code, data)
		return
	}

	choices := data["payload"].(map[string]interface{})["choices"].(map[string]interface{})
	status := int(choices["status"].(float64))
	content := choices["text"].([]interface{})[0].(map[string]interface{})["content"].(string)
	fmt.Print(content)

	if status == 2 {
		fmt.Println()
	}
}

func genParams(appID, question string) map[string]interface{} {
	data := map[string]interface{}{
		"header": map[string]interface{}{
			"app_id": appID,
			"uid":    "1234",
		},
		"parameter": map[string]interface{}{
			"chat": map[string]interface{}{
				"domain":           "general",
				"random_threshold": 0.5,
				"max_tokens":       2048,
				"auditing":         "default",
			},
		},
		"payload": map[string]interface{}{
			"message": map[string]interface{}{
				"text": []map[string]interface{}{
					{
						"role":    "user",
						"content": question,
					},
				},
			},
		},
	}

	return data
}
