// Copyright 2022 Harness Inc. All rights reserved.
// Use of this source code is governed by the Polyform Free Trial License
// that can be found in the LICENSE.md file for this repository.

package server

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/harness/gitness/events"
	"github.com/harness/gitness/gitrpc"
	"github.com/harness/gitness/gitrpc/server"
	"github.com/harness/gitness/internal/services/webhook"
	"github.com/harness/gitness/types"

	"github.com/kelseyhightower/envconfig"
)

// LoadConfig returns the system configuration from the
// host environment.
func LoadConfig() (*types.Config, error) {
	config := new(types.Config)
	err := envconfig.Process("", config)
	if err != nil {
		return nil, err
	}

	config.InstanceID, err = getSanitizedMachineName()
	if err != nil {
		return nil, fmt.Errorf("unable to ensure that instance ID is set in config: %w", err)
	}

	return config, nil
}

// getSanitizedMachineName gets the name of the machine and returns it in sanitized format.
func getSanitizedMachineName() (string, error) {
	// use the hostname as default id of the instance
	hostName, err := os.Hostname()
	if err != nil {
		return "", err
	}

	// Always cast to lower and remove all unwanted chars
	// NOTE: this could theoretically lead to overlaps, then it should be passed explicitly
	// NOTE: for k8s names/ids below modifications are all noops
	// https://kubernetes.io/docs/concepts/overview/working-with-objects/names/
	hostName = strings.ToLower(hostName)
	hostName = strings.Map(func(r rune) rune {
		switch {
		case 'a' <= r && r <= 'z':
			return r
		case '0' <= r && r <= '9':
			return r
		case r == '-', r == '.':
			return r
		default:
			return '_'
		}
	}, hostName)

	return hostName, nil
}

// ProvideGitRPCServerConfig loads the gitrpc server config from the environment.
// It backfills certain config elements to work with cmdone.
func ProvideGitRPCServerConfig() (server.Config, error) {
	config := server.Config{}
	err := envconfig.Process("", &config)
	if err != nil {
		return server.Config{}, fmt.Errorf("failed to load gitrpc server config: %w", err)
	}
	if config.GitHookPath == "" {
		var executablePath string
		executablePath, err = os.Executable()
		if err != nil {
			return server.Config{}, fmt.Errorf("failed to get path of current executable: %w", err)
		}

		config.GitHookPath = executablePath
	}
	if config.GitRoot == "" {
		var homedir string
		homedir, err = os.UserHomeDir()
		if err != nil {
			return server.Config{}, err
		}

		config.GitRoot = filepath.Join(homedir, ".gitness")
	}

	return config, nil
}

// ProvideGitRPCClientConfig loads the gitrpc client config from the environment.
func ProvideGitRPCClientConfig() (gitrpc.Config, error) {
	config := gitrpc.Config{}
	err := envconfig.Process("", &config)
	if err != nil {
		return gitrpc.Config{}, fmt.Errorf("failed to load gitrpc client config: %w", err)
	}

	return config, nil
}

// ProvideEventsConfig loads the events config from the environment.
func ProvideEventsConfig() (events.Config, error) {
	config := events.Config{}
	err := envconfig.Process("", &config)
	if err != nil {
		return events.Config{}, fmt.Errorf("failed to load events config: %w", err)
	}

	return config, nil
}

// ProvideWebhookConfig loads the webhook config from the environment.
// It backfills certain config elements if required.
func ProvideWebhookConfig() (webhook.Config, error) {
	config := webhook.Config{}
	err := envconfig.Process("", &config)
	if err != nil {
		return webhook.Config{}, fmt.Errorf("failed to load events config: %w", err)
	}

	if config.EventReaderName == "" {
		config.EventReaderName, err = getSanitizedMachineName()
		if err != nil {
			return webhook.Config{}, fmt.Errorf("failed to get sanitized machine name: %w", err)
		}
	}

	return config, nil
}
