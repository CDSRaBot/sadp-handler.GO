package ui

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/CDSRaBot/sadp-handler.GO/internal/fault"
	"github.com/CDSRaBot/sadp-handler.GO/internal/route"
)

func parseISAPIUser(args []string) (Command, error) {
	command := ISAPIUserCommand{}
	connection := []string{"isapi-discover"}
	seen := make(map[string]bool)
	for i := 1; i < len(args); i++ {
		option := args[i]
		switch option {
		case "--create", "--update", "--yes":
			if seen[option] {
				return nil, errors.New("duplicate ISAPI user option")
			}
			seen[option] = true
			switch option {
			case "--create":
				command.Operation = route.ISAPIUserCreate
			case "--update":
				command.Operation = route.ISAPIUserUpdate
			case "--yes":
				command.Confirm = true
			}
		case "--user-id", "--user-name":
			if seen[option] || i+1 >= len(args) {
				return nil, errors.New("invalid ISAPI user option")
			}
			seen[option] = true
			i++
			if option == "--user-id" {
				command.ID = args[i]
			} else {
				command.Name = args[i]
			}
		default:
			connection = append(connection, option)
			if option == "--address" || option == "--device" || option == "--username" || option == "--port" {
				if i+1 >= len(args) {
					return nil, errors.New("ISAPI connection option requires a value")
				}
				i++
				connection = append(connection, args[i])
			}
		}
	}
	if seen["--create"] && seen["--update"] {
		return nil, errors.New("ISAPI user operations conflict")
	}
	parsed, err := parseISAPIDiscover(connection)
	if err != nil {
		return nil, err
	}
	command.Connection = parsed.(ISAPIDiscoverCommand)
	if err := validateISAPIUser(command); err != nil {
		return nil, err
	}
	return command, nil
}

func validateISAPIUser(command ISAPIUserCommand) error {
	if err := validateISAPIDiscover(command.Connection); err != nil {
		return err
	}
	if !command.Confirm {
		return errors.New("ISAPI user operation requires confirmation")
	}
	return route.ValidateISAPIUserIdentity(command.Operation, command.ID, command.Name)
}

func dispatchISAPIUser(ctx context.Context, command ISAPIUserCommand, dependencies Dependencies) (CommandResult, error) {
	if err := validateISAPIUser(command); err != nil {
		return dispatchFailure("isapi-user", FailureInvalidArgument, "invalid ISAPI user command", err)
	}
	if !dependencies.Config.Once || dependencies.Config.Mode != "no" {
		return dispatchFailure("isapi-user", FailureUnsupported, "ISAPI user management requires one-shot CLI mode", errors.New("ISAPI user management requires one-shot CLI mode"))
	}
	if dependencies.ISAPIUser == nil {
		return dispatchUnsupported("isapi-user")
	}
	user, err := dependencies.ISAPIUser.ConfigureISAPIUser(ctx, command)
	public := ISAPIUserResult{
		Address: command.Connection.Address.String(), MAC: command.Connection.MAC.String(),
		Operation: command.Operation, ID: command.ID, Attempted: user.Attempted,
	}
	if err != nil {
		code := fault.Classify(err)
		message := "ISAPI user operation failed; if attempted, the device state is unknown"
		for _, inputErr := range []error{errPasswordInputUnavailable, errPasswordInputTerminal, errPasswordInputRead, errPasswordInputInvalid} {
			if errors.Is(err, inputErr) {
				code = FailureInvalidArgument
				message = "ISAPI user credential input is invalid or unavailable"
				break
			}
		}
		if code == FailureUnsupported && !public.Attempted {
			message = "ISAPI user management is disabled: a verified device profile is required"
		}
		result, dispatchErr := dispatchFailure("isapi-user", code, message, err)
		result.ISAPIUser = &public
		return result, dispatchErr
	}
	if user.Address != public.Address || user.MAC != public.MAC || user.Operation != public.Operation ||
		user.ID != public.ID || !user.Attempted || !user.Verified {
		result, dispatchErr := dispatchFailure("isapi-user", fault.MalformedResponse, "invalid ISAPI user result", errors.New("invalid ISAPI user result"))
		result.ISAPIUser = &public
		return result, dispatchErr
	}
	public.Verified = true
	return CommandResult{Command: "isapi-user", OK: true, ISAPIUser: &public}, nil
}

type ISAPIKnownAddressUserService struct {
	Passwords PasswordProvider
	Client    *http.Client
	Profile   route.ISAPIUserProfile
}

func (s ISAPIKnownAddressUserService) ConfigureISAPIUser(ctx context.Context, command ISAPIUserCommand) (ISAPIUserResult, error) {
	connection := command.Connection
	result := ISAPIUserResult{Address: connection.Address.String(), MAC: connection.MAC.String(), Operation: command.Operation, ID: command.ID}
	if ctx == nil || s.Passwords == nil {
		return result, &route.Error{Kind: route.ErrorInvalidInput, Err: errors.New("ISAPI user dependencies are incomplete")}
	}
	if err := validateISAPIUser(command); err != nil {
		return result, &route.Error{Kind: route.ErrorInvalidInput, Err: err}
	}
	if err := ctx.Err(); err != nil {
		return result, err
	}
	if s.Profile == nil {
		return result, &route.Error{Kind: route.ErrorUnsupported, Err: errors.New("verified ISAPI user profile is required")}
	}
	transport := knownAddressTransport(connection.Port, connection.HTTPS, s.Client)
	transport.Username = connection.Username
	transport.AllowBasic = connection.AllowBasic
	target := route.Target{Address: connection.Address, MAC: append([]byte(nil), connection.MAC...)}
	if err := checkKnownAddressRoute(ctx, transport, target); err != nil {
		return result, err
	}
	input, err := s.Passwords.Password(ctx)
	if err != nil {
		return result, err
	}
	adminPassword, userPassword, err := parseISAPIUserCredentials(input)
	if err != nil {
		return result, err
	}
	transport.Password = adminPassword
	workflow := route.ISAPIUserWorkflow{Transport: transport, Profile: s.Profile}
	operation, err := workflow.Configure(ctx, target, route.ISAPIUserRequest{
		Operation: command.Operation, ID: command.ID, Name: command.Name, Password: userPassword,
	})
	result.Attempted = operation.MutationAttempted
	result.Verified = operation.Verified
	return result, err
}

func parseISAPIUserCredentials(input string) (string, string, error) {
	if !utf8.ValidString(input) || len(input) > maxOneShotPasswordBytes {
		return "", "", errPasswordInputInvalid
	}
	decoder := json.NewDecoder(strings.NewReader(input))
	token, err := decoder.Token()
	if err != nil || token != json.Delim('{') {
		return "", "", errPasswordInputInvalid
	}
	values := make(map[string]string, 2)
	for decoder.More() {
		token, err = decoder.Token()
		if err != nil {
			return "", "", errPasswordInputInvalid
		}
		key, ok := token.(string)
		if !ok || (key != "admin_password" && key != "user_password") {
			return "", "", errPasswordInputInvalid
		}
		if _, exists := values[key]; exists {
			return "", "", errPasswordInputInvalid
		}
		var value string
		if err := decoder.Decode(&value); err != nil {
			return "", "", errPasswordInputInvalid
		}
		values[key] = value
	}
	if token, err = decoder.Token(); err != nil || token != json.Delim('}') {
		return "", "", errPasswordInputInvalid
	}
	if _, err = decoder.Token(); err != io.EOF || len(values) != 2 {
		return "", "", errPasswordInputInvalid
	}
	for _, value := range values {
		if value == "" || len(value) > route.MaxISAPIUserFieldBytes || !utf8.ValidString(value) {
			return "", "", errPasswordInputInvalid
		}
		for _, character := range value {
			if unicode.IsControl(character) {
				return "", "", errPasswordInputInvalid
			}
		}
	}
	return values["admin_password"], values["user_password"], nil
}
