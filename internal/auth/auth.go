package auth

import (
	"fmt"

	"github.com/zalando/go-keyring"
)

const (
	service    = "daily-work"
	githubUser = "github-pat"
	geminiUser = "gemini-api-key"
)

func StoreGitHubPAT(token string) error {
	if token == "" {
		return fmt.Errorf("GitHub PAT cannot be empty")
	}
	return keyring.Set(service, githubUser, token)
}

func GetGitHubPAT() (string, error) {
	token, err := keyring.Get(service, githubUser)
	if err != nil {
		if err == keyring.ErrNotFound {
			return "", fmt.Errorf("GitHub authentication required.\n\nPlease authenticate with GitHub.\n\nRun:\n\n  daily-work auth")
		}
		return "", err
	}
	return token, nil
}

func DeleteGitHubPAT() error {
	err := keyring.Delete(service, githubUser)
	if err != nil && err != keyring.ErrNotFound {
		return err
	}
	return nil
}

func StoreGeminiKey(key string) error {
	if key == "" {
		return fmt.Errorf("Gemini API key cannot be empty")
	}
	return keyring.Set(service, geminiUser, key)
}

func GetGeminiKey() (string, error) {
	key, err := keyring.Get(service, geminiUser)
	if err != nil {
		if err == keyring.ErrNotFound {
			return "", fmt.Errorf("Gemini API key required.\n\nRun:\n\n  daily-work auth")
		}
		return "", err
	}
	return key, nil
}

func DeleteGeminiKey() error {
	err := keyring.Delete(service, geminiUser)
	if err != nil && err != keyring.ErrNotFound {
		return err
	}
	return nil
}
