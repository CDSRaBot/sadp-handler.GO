package route

import (
	"crypto/md5"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
)

func digestAuthorization(challenges []string, request *http.Request, username, password string) (string, error) {
	var challenge string
	for _, value := range challenges {
		value = strings.TrimSpace(value)
		if len(value) >= 7 && strings.EqualFold(value[:7], "Digest ") {
			challenge = value[7:]
			break
		}
	}
	if challenge == "" || username == "" || password == "" {
		return "", errors.New("supported Digest challenge and credentials are required")
	}
	params, err := parseDigestParameters(challenge)
	if err != nil {
		return "", err
	}
	if params["realm"] == "" || params["nonce"] == "" {
		return "", errors.New("Digest challenge is incomplete")
	}
	algorithm := strings.ToUpper(params["algorithm"])
	if algorithm == "" {
		algorithm = "MD5"
	}
	if algorithm != "MD5" && algorithm != "SHA-256" {
		return "", errors.New("Digest algorithm is unsupported")
	}
	supportsAuth := false
	for _, qop := range strings.Split(params["qop"], ",") {
		if strings.EqualFold(strings.TrimSpace(qop), "auth") {
			supportsAuth = true
		}
	}
	if !supportsAuth {
		return "", errors.New("Digest qop is unsupported")
	}
	nonceBytes := make([]byte, 16)
	if _, err := io.ReadFull(rand.Reader, nonceBytes); err != nil {
		return "", errors.New("Digest nonce generation failed")
	}
	cnonce := hex.EncodeToString(nonceBytes)
	hash := func(value string) string {
		if algorithm == "SHA-256" {
			sum := sha256.Sum256([]byte(value))
			return hex.EncodeToString(sum[:])
		}
		sum := md5.Sum([]byte(value))
		return hex.EncodeToString(sum[:])
	}
	uri := request.URL.RequestURI()
	ha1 := hash(username + ":" + params["realm"] + ":" + password)
	ha2 := hash(request.Method + ":" + uri)
	response := hash(ha1 + ":" + params["nonce"] + ":00000001:" + cnonce + ":auth:" + ha2)
	quote := func(value string) string {
		return `"` + strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(value) + `"`
	}
	header := fmt.Sprintf("Digest username=%s, realm=%s, nonce=%s, uri=%s, algorithm=%s, response=%s, qop=auth, nc=00000001, cnonce=%s",
		quote(username), quote(params["realm"]), quote(params["nonce"]), quote(uri), algorithm, quote(response), quote(cnonce))
	if opaque, ok := params["opaque"]; ok {
		header += ", opaque=" + quote(opaque)
	}
	return header, nil
}

func parseDigestParameters(value string) (map[string]string, error) {
	params := make(map[string]string)
	for len(strings.TrimSpace(value)) > 0 {
		value = strings.TrimSpace(value)
		index := strings.IndexByte(value, '=')
		if index <= 0 {
			return nil, errors.New("Digest challenge is malformed")
		}
		name := strings.ToLower(strings.TrimSpace(value[:index]))
		if name == "" || strings.ContainsAny(name, " ,\t\r\n") {
			return nil, errors.New("Digest challenge is malformed")
		}
		if _, exists := params[name]; exists {
			return nil, errors.New("Digest challenge contains duplicate parameters")
		}
		value = strings.TrimSpace(value[index+1:])
		var parameter string
		if strings.HasPrefix(value, `"`) {
			value = value[1:]
			var builder strings.Builder
			closed := false
			for len(value) > 0 {
				character := value[0]
				value = value[1:]
				if character == '"' {
					closed = true
					break
				}
				if character == '\\' {
					if len(value) == 0 {
						return nil, errors.New("Digest challenge is malformed")
					}
					character, value = value[0], value[1:]
				}
				if character < 0x20 || character == 0x7f {
					return nil, errors.New("Digest challenge is malformed")
				}
				builder.WriteByte(character)
			}
			if !closed {
				return nil, errors.New("Digest challenge is malformed")
			}
			parameter = builder.String()
		} else {
			index = strings.IndexByte(value, ',')
			if index < 0 {
				index = len(value)
			}
			parameter = strings.TrimSpace(value[:index])
			value = value[index:]
			if parameter == "" || strings.ContainsAny(parameter, " \t\r\n\"") {
				return nil, errors.New("Digest challenge is malformed")
			}
		}
		params[name] = parameter
		value = strings.TrimSpace(value)
		if value != "" {
			if value[0] != ',' {
				return nil, errors.New("Digest challenge is malformed")
			}
			value = value[1:]
			if strings.TrimSpace(value) == "" {
				return nil, errors.New("Digest challenge is malformed")
			}
		}
	}
	return params, nil
}
