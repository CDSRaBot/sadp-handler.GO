package logic_test

import (
	"errors"
	"regexp"
	"strings"
	"sync"
	"testing"

	"github.com/CDSRaBot/sadp-handler.GO/internal/logic"
)

func TestCalculateHash_PositiveVectors(t *testing.T) {
	testCases := []struct {
		name      string
		password  string
		challenge string
		expected  string
	}{
		{
			name:      "simple alphanumeric credentials",
			password:  "admin",
			challenge: "12345678",
			expected:  "3c8872c094682f4c3fcdffdab80a8d351dbeee9b31a29e6ea092b51cf473e732",
		},
		{
			name:      "complex password with punctuation",
			password:  "Password123!",
			challenge: "9876543210",
			expected:  "f63926fee3a9a75a102cc357fb34e1961cbfd18065883cdd4d56dfae2a5cec46",
		},
		{
			name:      "token-style challenge",
			password:  "test_password",
			challenge: "test_challenge_token",
			expected:  "ab4d8b03cb49d3f19f7d12c5442237cd3697b74e2cd1be686d51835a7c38c9b8",
		},
		{
			name:      "passphrase with spaces",
			password:  "correct horse battery staple",
			challenge: "1a2b3c4d5e6f",
			expected:  "965b5b34ef68e76a24fd519b2cbf4f3d55b136bb0e2d0d3fd57254eaa83fbf2f",
		},
		{
			name:      "multibyte unicode utf8 representation",
			password:  "ПримерПароля123",
			challenge: "юникодЧеллендж",
			expected:  "1c84b56b3f6a123d39bead525dad6a8a63948ab28448796c2da1b389c21b4f07",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := logic.CalculateHash(tc.password, tc.challenge)
			if err != nil {
				t.Fatalf("expected no error, got: %v", err)
			}

			if got != tc.expected {
				t.Errorf("digest mismatch:\n got:      %q\n expected: %q", got, tc.expected)
			}

			if len(got) != 64 {
				t.Errorf("expected 64 characters, got %d", len(got))
			}

			if got != strings.ToLower(got) {
				t.Errorf("digest contains uppercase characters: %q", got)
			}
		})
	}
}

func TestCalculateHash_OutputFormatting(t *testing.T) {
	hexRegex := regexp.MustCompile(`^[0-9a-f]{64}$`)

	got, err := logic.CalculateHash("validPassword1", "validChallengeToken2")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !hexRegex.MatchString(got) {
		t.Errorf("result %q does not match expected lowercase 64-hex pattern", got)
	}
}

func TestCalculateHash_OrderSensitivity(t *testing.T) {
	partA := "alphaSegment"
	partB := "betaSegment"

	hashAB, errAB := logic.CalculateHash(partA, partB)
	if errAB != nil {
		t.Fatalf("unexpected error for (A, B): %v", errAB)
	}

	hashBA, errBA := logic.CalculateHash(partB, partA)
	if errBA != nil {
		t.Fatalf("unexpected error for (B, A): %v", errBA)
	}

	if hashAB == hashBA {
		t.Fatalf("expected different hashes for different concatenation orders, got identical: %s", hashAB)
	}
}

func TestCalculateHash_NegativeScenarios(t *testing.T) {
	testCases := []struct {
		name          string
		password      string
		challenge     string
		expectedError error
		expectedField string
	}{
		{
			name:          "empty password with valid challenge",
			password:      "",
			challenge:     "sampleChallenge123",
			expectedError: logic.ErrEmptyPassword,
			expectedField: "password",
		},
		{
			name:          "valid password with empty challenge",
			password:      "samplePassword123",
			challenge:     "",
			expectedError: logic.ErrEmptyChallenge,
			expectedField: "challenge",
		},
		{
			name:          "both arguments empty",
			password:      "",
			challenge:     "",
			expectedError: logic.ErrEmptyPassword,
			expectedField: "password",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := logic.CalculateHash(tc.password, tc.challenge)
			if err == nil {
				t.Fatalf("expected error for %s, got nil and result %q", tc.name, got)
			}

			if got != "" {
				t.Errorf("expected empty string on error, got %q", got)
			}

			if !errors.Is(err, tc.expectedError) {
				t.Errorf("expected error %v, got %v", tc.expectedError, err)
			}

			var valErr *logic.ValidationError
			if !errors.As(err, &valErr) {
				t.Errorf("expected error to be *logic.ValidationError, got %T", err)
			} else if valErr.Field != tc.expectedField {
				t.Errorf("expected validation field %q, got %q", tc.expectedField, valErr.Field)
			}
		})
	}
}

func TestCalculateHash_SecurityNoDisclosures(t *testing.T) {
	canaryPassword := "sensitive-password-canary-secret-12345"
	canaryChallenge := "sensitive-challenge-canary-token-67890"

	t.Run("empty password does not disclose challenge canary", func(t *testing.T) {
		_, err := logic.CalculateHash("", canaryChallenge)
		if err == nil {
			t.Fatal("expected error, got nil")
		}

		errMsg := err.Error()
		if strings.Contains(errMsg, canaryChallenge) {
			t.Errorf("security violation: error message disclosed challenge: %s", errMsg)
		}
	})

	t.Run("empty challenge does not disclose password canary", func(t *testing.T) {
		_, err := logic.CalculateHash(canaryPassword, "")
		if err == nil {
			t.Fatal("expected error, got nil")
		}

		errMsg := err.Error()
		if strings.Contains(errMsg, canaryPassword) {
			t.Errorf("security violation: error message disclosed password: %s", errMsg)
		}
	})
}

func TestCalculateHash_Concurrent(t *testing.T) {
	const goroutines = 50
	const iterations = 100

	var wg sync.WaitGroup
	wg.Add(goroutines)

	for i := 0; i < goroutines; i++ {
		go func(workerID int) {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				digest, err := logic.CalculateHash("concurrentPassword123", "concurrentChallenge456")
				if err != nil {
					t.Errorf("worker %d failed: %v", workerID, err)
					return
				}
				if len(digest) != 64 {
					t.Errorf("worker %d got invalid length: %d", workerID, len(digest))
					return
				}
			}
		}(i)
	}

	wg.Wait()
}
