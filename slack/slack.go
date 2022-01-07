package slack

import (
	"fmt"
	"os"

	"github.com/cantara/vili/whydah"
)

type slackMessage struct {
	SlackId string `json:"recepientId"`
	Message string `json:"message"`
	//	Username    string   `json:"username"`
	Pinned bool `json:"pinned"`
	//	Attachments []string `json:"attachments"`
}

func SendChannel(message, slackId string) (err error) {
	return whydah.PostAuth(os.Getenv("entraos_api_uri")+"/slack/api/message", slackMessage{
		SlackId: slackId,
		Message: message,
		Pinned:  false,
	}, nil)
}

func Send(message string) (err error) {
	return SendChannel(message, os.Getenv("slack_channel"))
}

func Sendf(format string, a ...interface{}) (err error) {
	return SendChannel(fmt.Sprintf(format, a...), os.Getenv("slack_channel"))
}
