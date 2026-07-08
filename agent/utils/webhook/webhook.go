package webhook

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/1Panel-dev/1Panel/agent/constant"
)

type weComMessage struct {
	MsgType string `json:"msgtype"`
	Text    struct {
		Content string `json:"content"`
	} `json:"text"`
}

type dingTalkMessage struct {
	MsgType string `json:"msgtype"`
	Text    struct {
		Content string `json:"content"`
	} `json:"text"`
}

type feiShuMessage struct {
	MsgType string `json:"msg_type"`
	Content struct {
		Text string `json:"text"`
	} `json:"content"`
}

func Send(url, title, content, method string, transport *http.Transport) error {
	message := title
	if content != "" {
		message = fmt.Sprintf("%s\n%s", title, content)
	}

	var body []byte
	var err error
	switch method {
	case constant.WeCom:
		var msg weComMessage
		msg.MsgType = "text"
		msg.Text.Content = message
		body, err = json.Marshal(msg)
	case constant.DingTalk:
		var msg dingTalkMessage
		msg.MsgType = "text"
		msg.Text.Content = message
		body, err = json.Marshal(msg)
	case constant.FeiShu:
		var msg feiShuMessage
		msg.MsgType = "text"
		msg.Content.Text = message
		body, err = json.Marshal(msg)
	default:
		return fmt.Errorf("unsupported webhook method: %s", method)
	}
	if err != nil {
		return err
	}

	client := &http.Client{Transport: transport}
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("webhook request failed, status code: %d", resp.StatusCode)
	}
	return nil
}
