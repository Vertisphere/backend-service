package net

import (
	"encoding/base64"
	"net/http"
	"os"
	"regexp"

	"github.com/Vertisphere/backend-service/internal/config"
	"github.com/rs/zerolog/log"
	"gopkg.in/square/go-jose.v2"

	"github.com/sendgrid/sendgrid-go"
	"github.com/sendgrid/sendgrid-go/helpers/mail"
)

func logHttpError(err error, msg string, statusCode int, w *http.ResponseWriter) {
	log.Error().Err(err).Msg(msg)
	http.Error(*w, msg, statusCode)
}

func qbToE164Phone(phone string) string {
	// Remove all non-digit characters
	reg := regexp.MustCompile(`[^0-9]`)
	cleaned := reg.ReplaceAllString(phone, "")

	// Validate length (10 digits for US numbers)
	if len(cleaned) == 10 {
		return "+1" + cleaned
	}
	if len(cleaned) == 11 && cleaned[0] == '1' {
		return "+" + cleaned
	}
	log.Error().Msgf("Couldn't convert qb phone number to E164 %s", phone)
	return ""
}

func encryptToken(token string) (string, error) {
	encKey := config.LoadConfigs().JWEKey
	rawKey, err := base64.StdEncoding.DecodeString(encKey)
	if err != nil {
		return "", err
	}
	encrypter, err := jose.NewEncrypter(jose.A256GCM, jose.Recipient{Algorithm: jose.DIRECT, Key: rawKey}, nil)
	if err != nil {
		return "", err
	}
	object, err := encrypter.Encrypt([]byte(token))
	if err != nil {
		return "", err
	}
	encryptedToken, err := object.CompactSerialize()
	if err != nil {
		return "", err
	}
	return encryptedToken, nil
}

// Send email via sendgrid
func sendEmail(fromName string, toName string, emails map[string]struct{}, subject string, content *mail.Content, attachments []*mail.Attachment) {
	// Initialize mail
	m := mail.NewV3Mail()

	from := mail.NewEmail(fromName, "verification@ordrport.com")

	personalization := mail.NewPersonalization()
	// For each key in the map (we use it like a map), we add to recipient list
	personalization.Subject = subject
	for email := range emails {
		if email == "" {
			continue
		}
		to := mail.NewEmail(toName, email)
		personalization.AddTos(to)
	}

	m.SetFrom(from)
	m.AddContent(content)
	m.AddPersonalizations(personalization)
	// TODO: add template id

	for _, attachment := range attachments {
		m.AddAttachment(attachment)
	}

	client := sendgrid.NewSendClient(os.Getenv("SENDGRID_API_KEY"))

	resp, err := client.Send(m)
	if err != nil {
		log.Error().Err(err).Msg("error sending email")
	}

	// TODO: IS this 202?
	if resp.StatusCode != http.StatusAccepted {
		log.Error().Interface("response", resp).Msg("reset email wasn't a 202")
	}
}
