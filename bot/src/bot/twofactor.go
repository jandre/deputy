package main

import (
	"fmt"
	"net/http"
	"os"

	log "github.com/Sirupsen/logrus"
	"github.com/gin-gonic/gin"

	"bitbucket.org/ckvist/twilio/twirest"
)

// Manages two factor confirmations.
type TwoFactor struct {
	client      *twirest.TwilioClient
	from        string
	callbackURL string

	pendingResponses map[string]*SecurityAlertWorkflow
}

func NewTwoFactor(callbackURL string) *TwoFactor {
	sid := os.Getenv("TWILIO_SID")
	token := os.Getenv("TWILIO_TOKEN")
	phone := os.Getenv("TWILIO_PHONE")

	client := twirest.NewClient(sid, token)
	return &TwoFactor{client: client,
		from:             phone,
		callbackURL:      callbackURL,
		pendingResponses: make(map[string]*SecurityAlertWorkflow),
	}
}

// HandleResponses handles any SMS responses
func (t *TwoFactor) HandleResponses() gin.HandlerFunc {
	return func(c *gin.Context) {
		resp := c.Writer

		sender := c.PostForm("From")
		body := c.PostForm("Body")

		var err error
		if err != nil {
			resp.WriteHeader(http.StatusBadRequest)
			return
		}
		resp.WriteHeader(http.StatusOK)
		alert, ok := t.pendingResponses[sender]
		if ok {
			alert.Handle2FAResponse(body)
			delete(t.pendingResponses, sender)
		}

	}
}

// Send will send a message
func (t *TwoFactor) Send(s *SecurityAlertWorkflow) error {
	to := s.User.PhoneNumber
	body := fmt.Sprintf(`Confirming acknowledgement for:  

%s

Type [y] to confirm, [n] to reject.`, s.Alert.Message)

	msg := twirest.SendMessage{
		From: t.from,
		To:   to,
		Text: body,
	}

	resp, err := t.client.Request(msg)
	if err == nil {
		key := "+1" + msg.To
		t.pendingResponses[key] = s
	}

	log.Debugf("Received response: %s from message sent to=%s body=%s", resp, to, body)

	return err
}
