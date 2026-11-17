package route

import (
	"errors"
	"net/http"
	"strings"
	"unicode/utf8"
)

func (t *ISAPIHTTPTransport) authorizationForChallenge(challenges []string, request *http.Request) (string, error) {
	if request == nil || request.URL == nil || t.Username == "" || t.Password == "" {
		return "", errors.New("ISAPI authentication credentials are incomplete")
	}
	var basic string
	hasDigest := false
	for _, challenge := range challenges {
		challenge = strings.TrimSpace(challenge)
		fields := strings.Fields(challenge)
		if len(fields) == 0 {
			continue
		}
		switch {
		case strings.EqualFold(fields[0], "Digest"):
			hasDigest = true
		case strings.EqualFold(fields[0], "Basic"):
			basic = strings.TrimSpace(challenge[len(fields[0]):])
		}
	}
	if hasDigest {
		return digestAuthorization(challenges, request, t.Username, t.Password)
	}
	if basic == "" || !t.AllowBasic || request.URL.Scheme != "https" {
		return "", errors.New("supported ISAPI authentication challenge is required")
	}
	params, err := parseDigestParameters(basic)
	if err != nil {
		return "", errors.New("ISAPI Basic challenge is malformed")
	}
	if _, ok := params["realm"]; !ok || (params["charset"] != "" && !strings.EqualFold(params["charset"], "UTF-8")) {
		return "", errors.New("ISAPI Basic challenge is unsupported")
	}
	if strings.Contains(t.Username, ":") || !validBasicText(t.Username, params["charset"]) || !validBasicText(t.Password, params["charset"]) {
		return "", errors.New("ISAPI Basic credentials are unsupported")
	}
	temporary := request.Clone(request.Context())
	temporary.SetBasicAuth(t.Username, t.Password)
	return temporary.Header.Get("Authorization"), nil
}

func validBasicText(value, charset string) bool {
	if !utf8.ValidString(value) {
		return false
	}
	for _, character := range value {
		if character < 0x20 || character == 0x7f || (character > 0x7f && !strings.EqualFold(charset, "UTF-8")) {
			return false
		}
	}
	return true
}
