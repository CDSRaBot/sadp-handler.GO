package ui

import (
	"bufio"
	"context"
	"errors"
	"io"
	"os"
	"strings"
	"unicode/utf8"

	"golang.org/x/term"
)

const maxOneShotPasswordBytes = 4096

var (
	errPasswordInputUnavailable = errors.New("password input is unavailable")
	errPasswordInputTerminal    = errors.New("password input must be redirected")
	errPasswordInputRead        = errors.New("password input could not be read")
	errPasswordInputInvalid     = errors.New("password input is invalid")
)

type oneShotPasswordProvider struct {
	reader   io.ReadCloser
	terminal bool
}

func NewOneShotPasswordProvider(file *os.File) PasswordProvider {
	if file == nil {
		return oneShotPasswordProvider{}
	}
	return oneShotPasswordProvider{reader: file, terminal: term.IsTerminal(int(file.Fd()))}
}

func (p oneShotPasswordProvider) Password(ctx context.Context) (string, error) {
	if ctx == nil || p.reader == nil {
		return "", errPasswordInputUnavailable
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if p.terminal {
		return "", errPasswordInputTerminal
	}
	type readResult struct {
		password string
		err      error
	}
	results := make(chan readResult, 1)
	go func() {
		line, err := bufio.NewReader(io.LimitReader(p.reader, maxOneShotPasswordBytes+2)).ReadString('\n')
		if err != nil && !errors.Is(err, io.EOF) {
			results <- readResult{err: errPasswordInputRead}
			return
		}
		line = strings.TrimSuffix(line, "\n")
		line = strings.TrimSuffix(line, "\r")
		if len(line) == 0 || len(line) > maxOneShotPasswordBytes || !utf8.ValidString(line) || strings.ContainsAny(line, "\x00\r") {
			results <- readResult{err: errPasswordInputInvalid}
			return
		}
		results <- readResult{password: line}
	}()
	select {
	case result := <-results:
		if err := ctx.Err(); err != nil {
			return "", err
		}
		return result.password, result.err
	case <-ctx.Done():
		if err := p.reader.Close(); err != nil {
			return "", errors.Join(ctx.Err(), errors.New("password input could not be closed"))
		}
		return "", ctx.Err()
	}
}
