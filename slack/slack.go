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

type client struct {
	appIcon string
	env     string
	envIcon string
	service string
}

var Client client

func NewClient(appIcon, envIcon, env, service string) client {
	return client{
		appIcon: appIcon,
		env:     env,
		envIcon: envIcon,
		service: service,
	}
}

func (c client) sendChannel(message, slackId string) (err error) {
	return whydah.PostAuth(os.Getenv("entraos_api_uri")+"/slack/api/message", slackMessage{
		SlackId: slackId,
		Message: fmt.Sprintf("%s[%s%s-%s]%s", c.appIcon, c.envIcon, c.env, c.service, message),
		Pinned:  false,
	}, nil)
}

func (c client) Send(message string) (err error) {
	return c.sendChannel(message, os.Getenv("slack_channel"))
}

func Send(message string) (err error) {
	return Client.Send(message)
}

func (c client) Sendf(format string, a ...interface{}) (err error) {
	return c.sendChannel(fmt.Sprintf(format, a...), os.Getenv("slack_channel"))
}

func Sendf(format string, a ...interface{}) (err error) {
	return Client.Sendf(format, a...)
}
