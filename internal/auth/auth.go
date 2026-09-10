package auth

import (
	"fmt"

	"github.com/zalando/go-keyring"
)

const (
	service    = "daily-work"
	githubUser = "github-pat"
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
