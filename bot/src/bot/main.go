package main

import (
	"os"
	"regexp"
	"time"

	log "github.com/Sirupsen/logrus"

	slackbot "github.com/BeepBoopHQ/go-slackbot"
	"github.com/gin-gonic/gin"
	"github.com/nlopes/slack"
	"golang.org/x/net/context"
)

const (
	WithTyping    = slackbot.WithTyping
	WithoutTyping = slackbot.WithoutTyping

	HelpText = "I will respond to the following messages: \n" +
		"`bot hi` for a simple message.\n" +
		"`bot attachment` to see a Slack attachment message.\n" +
		"`hey @<your bot's name>` to demonstrate detecting a mention.\n" +
		"`bot help` to see this again."
)

type MessageStatus string

var (
	NEW                  = MessageStatus("new")
	SENT                 = MessageStatus("sent")
	ACKNOWLEDGED         = MessageStatus("acknowledged")
	CONFIRMED            = MessageStatus("confirmed")
	REJECTED             = MessageStatus("rejected")
	NOTIFY_SECURITY_TEAM = MessageStatus("notify_security_team")
)

var messages = make(map[string]*SecurityAlertWorkflow)

type Log struct {
	Status MessageStatus
	Time   time.Time
}

type SecurityAlert struct {
	Time    time.Time `json:"time"`
	Message string    `json:"message"`
	Host    string    `json:"host"`

	Username string
}

// SecurityAlertWorkflow processes a security message.
type SecurityAlertWorkflow struct {
	User    UserInformation
	Alert   *SecurityAlert
	Status  MessageStatus
	Created time.Time
	Log     []Log

	ChannelID string

	twofactor *TwoFactor
	bot       *slackbot.Bot
}

func NewSecurityAlertWorkflow(user UserInformation, alert *SecurityAlert, bot *slackbot.Bot, twofactor *TwoFactor) *SecurityAlertWorkflow {
	return &SecurityAlertWorkflow{User: user, Alert: alert, Status: NEW, bot: bot, twofactor: twofactor}
}

func (s *SecurityAlertWorkflow) Escalate() {

	params := slack.NewPostMessageParameters()
	params.Username = "deputy"
	s.Status = SENT
	params.Markdown = true
	params.IconEmoji = ":scream:"

	attachments := make([]slack.Attachment, 1)
	attachments[0] = slack.Attachment{
		Pretext: "Security event has been escalated to the team. Please Investigate.",
		Fields: []slack.AttachmentField{
			slack.AttachmentField{
				Title: "User",
				Value: s.User.Name,
			},
			slack.AttachmentField{
				Title: "Message",
				Value: s.Alert.Message,
			},
			slack.AttachmentField{
				Title: "Time",
				Value: s.Alert.Time.String(),
			},
			slack.AttachmentField{
				Title: "Host",
				Value: s.Alert.Host,
			},
		},
	}

	params.Attachments = attachments
	s.bot.Client.PostMessage("#security", "", params)
}

func (s *SecurityAlertWorkflow) Handle2FAResponse(response string) {
	match, _ := regexp.MatchString("yes|y|ack|acknowledge", response)
	if !match {
		log.Warnf("Escalating to the security team: response=%s msg=%+v", response, s.Alert)
		s.Escalate()
	}
}

func (s *SecurityAlertWorkflow) OpenChat() {
	users, err := s.bot.Client.GetUsers()
	if err != nil {
		panic(err)
	}

	userId := ""
	for _, user := range users {
		if user.Name == s.User.Username {
			userId = user.ID
			break
		}
	}
	if userId == "" {
		panic("User not found")
	}

	_, _, channel, err := s.bot.Client.OpenIMChannel(userId)
	if err != nil {
		panic(err)
	}
	log.Printf("Opened %s with channel=%s", s.User.Username, channel)
	s.ChannelID = channel
}

func (s *SecurityAlertWorkflow) NotifyUser() {
	params := slack.NewPostMessageParameters()
	params.Username = "deputy"
	s.Status = SENT
	params.Markdown = true
	params.IconEmoji = ":scream:"

	attachments := make([]slack.Attachment, 2)
	attachments[0] = slack.Attachment{
		Pretext: "A suspicious command was ran.",
		Fields: []slack.AttachmentField{
			slack.AttachmentField{
				Title: "Message",
				Value: s.Alert.Message,
			},
			slack.AttachmentField{
				Title: "Time",
				Value: s.Alert.Time.String(),
			},
			slack.AttachmentField{
				Title: "Host",
				Value: s.Alert.Host,
			},
		},
	}
	attachments[1] = slack.Attachment{
		Color:   "#36a64f",
		Pretext: "Was this you? [yes/no]?",
	}

	params.Attachments = attachments
	s.bot.Client.PostMessage(s.ChannelID, "", params)
}

func (s *SecurityAlertWorkflow) Acknowledge(evt *slack.MessageEvent) {
	dmMsg := "Acknowledged. Confirming via 2FA."
	s.bot.Reply(evt, dmMsg, WithoutTyping)
	s.twofactor.Send(s)
}

func main() {
	bot := slackbot.New(os.Getenv("SLACK_TOKEN"))

	url := "https://deputy.ngrok.io"
	var twofactor = NewTwoFactor(url + "/confirm")

	Setup()

	bot.Hear("acknowledge|ack|ok|yes").MessageHandler(AcknowledgeHandler)
	bot.Hear("reject|no").MessageHandler(RejectHandler)

	go bot.Run()
	RunWebserver(bot, twofactor)
}

func RunWebserver(bot *slackbot.Bot, twofactor *TwoFactor) {
	r := gin.Default()
	// TODO: you'd want to auth this
	r.POST("/confirm", twofactor.HandleResponses())
	r.POST("/alerts", LookForNewSecurityAlerts(bot, twofactor))
	r.Run() // listen and server on 0.0.0.0:8080
}

func LookForNewSecurityAlerts(bot *slackbot.Bot, twofactor *TwoFactor) gin.HandlerFunc {
	return func(c *gin.Context) {
		msg := &SecurityAlert{}
		err := c.BindJSON(msg)

		if err != nil {
			log.Errorf("Unable to parse msg: %s", err)
			c.JSON(400, gin.H{"status": "error"})
			return
		}

		username := "jandre"
		user, ok := users[username]
		if ok {
			s := NewSecurityAlertWorkflow(user, msg, bot, twofactor)
			s.OpenChat()
			s.NotifyUser()
			messages[s.ChannelID] = s
			c.JSON(200, gin.H{"status": "ok"})
		}
	}
}

func AcknowledgeHandler(ctx context.Context, bot *slackbot.Bot, evt *slack.MessageEvent) {
	if slackbot.IsDirectMessage(evt) {
		msg, ok := messages[evt.Channel]
		if ok && msg.Status == SENT {
			msg.Acknowledge(evt)
		}
	}
}

func RejectHandler(ctx context.Context, bot *slackbot.Bot, evt *slack.MessageEvent) {
	if slackbot.IsDirectMessage(evt) {
		msg, ok := messages[evt.Channel]
		if ok && msg.Status == SENT {
			dmMsg := "Rejected. Escalating message to the security team."
			bot.Reply(evt, dmMsg, WithoutTyping)
			msg.Status = REJECTED
			msg.Escalate()
		}
	}
}
