// Copyright 2022 Harness Inc. All rights reserved.
// Use of this source code is governed by the Polyform Free Trial License
// that can be found in the LICENSE.md file for this repository.

package githook

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/harness/gitness/internal/api/controller/githook"

	"github.com/rs/zerolog/log"
)

// CLI represents the githook cli implementation.
type CLI struct {
	payload *Payload
	client  *client
}

// NewCLI creates a new CLI instance from environment variables for githook execution.
func NewCLI() (*CLI, error) {
	payload, err := loadPayloadFromEnvironment()
	if err != nil {
		return nil, fmt.Errorf("failed to load payload: %w", err)
	}

	return &CLI{
		payload: payload,
		client: &client{
			httpClient: http.DefaultClient,
			baseURL:    payload.APIBaseURL,
			requestPreparation: func(r *http.Request) *http.Request {
				// TODO: reference single constant (together with gitness middleware)
				r.Header.Add("X-Request-Id", payload.RequestID)
				return r
			},
		},
	}, nil
}

// PreReceive executes the pre-receive git hook.
func (c *CLI) PreReceive(ctx context.Context) error {
	refUpdates, err := getUpdatedReferencesFromStdIn()
	if err != nil {
		return fmt.Errorf("failed to read updated references from std in: %w", err)
	}

	in := &githook.PreReceiveInput{
		BaseInput: githook.BaseInput{
			RepoID:      c.payload.RepoID,
			PrincipalID: c.payload.PrincipalID,
		},
		RefUpdates: refUpdates,
	}

	out, err := c.client.PreReceive(ctx, in)

	return handleServerHookOutput(out, err)
}

// Update executes the update git hook.
func (c *CLI) Update(ctx context.Context, ref string, oldSHA string, newSHA string) error {
	in := &githook.UpdateInput{
		BaseInput: githook.BaseInput{
			RepoID:      c.payload.RepoID,
			PrincipalID: c.payload.PrincipalID,
		},
		RefUpdate: githook.ReferenceUpdate{
			Ref: ref,
			Old: oldSHA,
			New: newSHA,
		},
	}

	out, err := c.client.Update(ctx, in)

	return handleServerHookOutput(out, err)
}

// PostReceive executes the post-receive git hook.
func (c *CLI) PostReceive(ctx context.Context) error {
	refUpdates, err := getUpdatedReferencesFromStdIn()
	if err != nil {
		return fmt.Errorf("failed to read updated references from std in: %w", err)
	}

	in := &githook.PostReceiveInput{
		BaseInput: githook.BaseInput{
			RepoID:      c.payload.RepoID,
			PrincipalID: c.payload.PrincipalID,
		},
		RefUpdates: refUpdates,
	}

	out, err := c.client.PostReceive(ctx, in)

	return handleServerHookOutput(out, err)
}

func handleServerHookOutput(out *githook.ServerHookOutput, err error) error {
	if err != nil {
		return fmt.Errorf("an error occured when calling the server: %w", err)
	}

	if out == nil {
		return errors.New("the server returned an empty output")
	}

	if out.Error != nil {
		return errors.New(*out.Error)
	}

	return nil
}

// getUpdatedReferencesFromStdIn reads the updated references provided by git from stdin.
// The expected format is "<old-value> SP <new-value> SP <ref-name> LF"
// For more details see https://git-scm.com/docs/githooks#pre-receive
func getUpdatedReferencesFromStdIn() ([]githook.ReferenceUpdate, error) {
	reader := bufio.NewReader(os.Stdin)
	updatedRefs := []githook.ReferenceUpdate{}
	for {
		line, err := reader.ReadString('\n')
		// if end of file is reached, break the loop
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Error().Msgf("Error when reading from standard input - %v", err)
			return nil, err
		}

		if len(line) == 0 {
			return nil, errors.New("ref data from stdin contains empty line - not expected")
		}

		// splitting line of expected form "<old-value> SP <new-value> SP <ref-name> LF"
		splitGitHookData := strings.Split(line[:len(line)-1], " ")
		if len(splitGitHookData) != 3 {
			return nil, fmt.Errorf("received invalid data format or didn't receive enough parameters - %v",
				splitGitHookData)
		}

		updatedRefs = append(updatedRefs, githook.ReferenceUpdate{
			Old: splitGitHookData[0],
			New: splitGitHookData[1],
			Ref: splitGitHookData[2],
		})
	}

	return updatedRefs, nil
}
