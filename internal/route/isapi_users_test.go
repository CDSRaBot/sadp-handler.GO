package route

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

type syntheticUserProfile struct {
	key        []byte
	deny       bool
	verifyFail bool
	modifyMAC  bool
	badBody    bool
}

func (p syntheticUserProfile) VerifyUserDevice(_ context.Context, target Target, info DeviceInfo, _ ISAPIUserOperation) error {
	if p.modifyMAC {
		target.MAC[0] = 0xff
	}
	if p.deny || info.Model != "Synthetic" {
		return errors.New("synthetic-secret")
	}
	return nil
}

func (p syntheticUserProfile) EncryptionKey(context.Context, *ISAPIHTTPTransport, Target, ISAPIUserOperation) (ISAPIUserSecurity, []byte, error) {
	return ISAPIUserSecurityAES128, p.key, nil
}

func (p syntheticUserProfile) BuildUserMutation(request ISAPIUserRequest, encrypted ISAPIEncryptedField) (string, []byte, error) {
	if request.Password != "" {
		return "", nil, errors.New("plaintext password was passed to schema")
	}
	if p.badBody {
		return http.MethodPut, []byte("<User/>"), nil
	}
	body, err := xml.Marshal(struct {
		XMLName  xml.Name `xml:"User"`
		ID       string   `xml:"id"`
		Name     string   `xml:"userName"`
		Password string   `xml:"password"`
	}{ID: request.ID, Name: request.Name, Password: encrypted.Ciphertext})
	if request.Operation == ISAPIUserCreate {
		return http.MethodPost, body, err
	}
	return http.MethodPut, body, err
}

func (p syntheticUserProfile) VerifyUserState(request ISAPIUserRequest, state []byte) error {
	if p.verifyFail || request.Password != "" || string(state) != "<User><id>2</id><userName>synthetic</userName></User>" {
		return errors.New("synthetic-secret")
	}
	return nil
}

func TestISAPIUserWorkflow(t *testing.T) {
	for _, tt := range []struct {
		name       string
		operation  ISAPIUserOperation
		deny       bool
		modifyMAC  bool
		disconnect bool
		wrongMAC   bool
		verifyFail bool
		badBody    bool
		wantErr    ErrorKind
		attempted  bool
		verified   bool
	}{
		{name: "create", operation: ISAPIUserCreate, attempted: true, verified: true},
		{name: "update", operation: ISAPIUserUpdate, attempted: true, verified: true},
		{name: "unverified profile", operation: ISAPIUserUpdate, deny: true, wantErr: ErrorUnsupported},
		{name: "gate changes MAC", operation: ISAPIUserUpdate, modifyMAC: true, wantErr: ErrorVerification},
		{name: "schema omitted ciphertext", operation: ISAPIUserUpdate, badBody: true, wantErr: ErrorInvalidInput},
		{name: "unknown delivery", operation: ISAPIUserUpdate, disconnect: true, attempted: true, wantErr: ErrorTransport},
		{name: "identity changed", operation: ISAPIUserUpdate, wrongMAC: true, attempted: true, wantErr: ErrorVerification},
		{name: "verification fails", operation: ISAPIUserUpdate, verifyFail: true, attempted: true, wantErr: ErrorVerification},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var puts atomic.Int32
			server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				user, password, ok := r.BasicAuth()
				if !ok || user != "admin" || password != "synthetic-admin" {
					w.Header().Set("WWW-Authenticate", `Basic realm="synthetic"`)
					w.WriteHeader(http.StatusUnauthorized)
					return
				}
				if r.Method == http.MethodPut || r.Method == http.MethodPost {
					puts.Add(1)
					expectedPath := "/ISAPI/Security/users/2"
					if tt.operation == ISAPIUserCreate {
						expectedPath = "/ISAPI/Security/users"
					}
					if r.URL.Path != expectedPath || r.URL.Query().Get("security") != "1" {
						t.Error("unexpected user mutation path or security scheme")
					}
					iv, err := base64.StdEncoding.DecodeString(r.URL.Query().Get("iv"))
					if err != nil || len(iv) != 16 {
						t.Error("invalid encoded request IV")
					}
					body, err := io.ReadAll(r.Body)
					if err != nil || bytes.Contains(body, []byte("SyntheticUser1!")) || !bytes.Contains(body, []byte("<password>")) {
						t.Error("user password was not safely encrypted")
					}
					if tt.disconnect {
						connection, _, err := w.(http.Hijacker).Hijack()
						if err != nil {
							t.Error(err)
							return
						}
						_ = connection.Close()
						return
					}
					_, _ = fmt.Fprint(w, "<ResponseStatus><statusCode>1</statusCode></ResponseStatus>")
					return
				}
				switch r.URL.Path {
				case "/ISAPI/System/deviceInfo":
					mac := "00:01:02:03:04:05"
					if tt.wrongMAC && puts.Load() != 0 {
						mac = "00:01:02:03:04:06"
					}
					_, _ = fmt.Fprintf(w, "<DeviceInfo><macAddress>%s</macAddress><model>Synthetic</model></DeviceInfo>", mac)
				case "/ISAPI/Security/users/2":
					_, _ = fmt.Fprint(w, "<User><id>2</id><userName>synthetic</userName></User>")
				default:
					t.Errorf("unexpected authenticated request: %s %s", r.Method, r.URL.Path)
					w.WriteHeader(http.StatusNotFound)
				}
			}))
			defer server.Close()
			key := bytes.Repeat([]byte{7}, 16)
			transport := &ISAPIHTTPTransport{BaseURL: server.URL, Client: server.Client(), Username: "admin",
				Password: "synthetic-admin", AllowBasic: true}
			workflow := ISAPIUserWorkflow{Transport: transport, Profile: syntheticUserProfile{
				key: key, deny: tt.deny, modifyMAC: tt.modifyMAC, verifyFail: tt.verifyFail, badBody: tt.badBody,
			}}
			result, err := workflow.Configure(context.Background(), serverTarget(t, server), ISAPIUserRequest{
				Operation: tt.operation, ID: "2", Name: "synthetic", Password: "SyntheticUser1!",
			})
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
			} else {
				var routeErr *Error
				if !errors.As(err, &routeErr) || routeErr.Kind != tt.wantErr || strings.Contains(err.Error(), "synthetic-secret") {
					t.Fatalf("error=%v, want %s without secret", err, tt.wantErr)
				}
			}
			if result.MutationAttempted != tt.attempted || result.Verified != tt.verified ||
				(puts.Load() != 0) != tt.attempted {
				t.Fatalf("result=%+v PUTs=%d", result, puts.Load())
			}
			if !tt.deny && !tt.modifyMAC && !bytes.Equal(key, make([]byte, len(key))) {
				t.Fatal("derived key not cleared")
			}
		})
	}
}

func TestISAPIUserRequestValidation(t *testing.T) {
	valid := ISAPIUserRequest{Operation: ISAPIUserUpdate, ID: "2", Name: "synthetic", Password: "SyntheticUser1!"}
	if err := ValidateISAPIUserRequest(valid); err != nil {
		t.Fatal(err)
	}
	for _, modify := range []func(*ISAPIUserRequest){
		func(r *ISAPIUserRequest) { r.Operation = "delete" },
		func(r *ISAPIUserRequest) { r.ID = "../2" },
		func(r *ISAPIUserRequest) { r.ID = "02" },
		func(r *ISAPIUserRequest) { r.ID = "4294967296" },
		func(r *ISAPIUserRequest) { r.Name = "invalid\nname" },
		func(r *ISAPIUserRequest) { r.Password = "" },
		func(r *ISAPIUserRequest) { r.Password = string([]byte{0xff}) },
	} {
		request := valid
		modify(&request)
		if err := ValidateISAPIUserRequest(request); err == nil {
			t.Error("invalid user request accepted")
		}
	}
}
