// Copyright 2022 Harness Inc. All rights reserved.
// Use of this source code is governed by the Polyform Free Trial License
// that can be found in the LICENSE.md file for this repository.

package authn

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/harness/gitness/internal/auth"
	"github.com/harness/gitness/internal/store"
	"github.com/harness/gitness/internal/token"
	"github.com/harness/gitness/types"
	"github.com/harness/gitness/types/enum"

	"github.com/dgrijalva/jwt-go"
	"github.com/rs/zerolog/hlog"
)

var _ Authenticator = (*TokenAuthenticator)(nil)

/*
 * Authenticates a user by checking for an access token in the
 * "Authorization" header or the "access_token" form value.
 */
type TokenAuthenticator struct {
	userStore  store.UserStore
	saStore    store.ServiceAccountStore
	tokenStore store.TokenStore
}

func NewTokenAuthenticator(
	userStore store.UserStore,
	saStore store.ServiceAccountStore,
	tokenStore store.TokenStore) *TokenAuthenticator {
	return &TokenAuthenticator{
		userStore:  userStore,
		saStore:    saStore,
		tokenStore: tokenStore,
	}
}

func (a *TokenAuthenticator) Authenticate(r *http.Request) (*auth.Session, error) {
	ctx := r.Context()
	str := extractToken(r)

	if len(str) == 0 {
		return nil, ErrNoAuthData
	}

	var principal *types.Principal
	var err error
	claims := &token.JWTClaims{}
	parsed, err := jwt.ParseWithClaims(str, claims, func(token_ *jwt.Token) (interface{}, error) {
		principal, err = a.getPrincipal(ctx, claims)
		if err != nil {
			hlog.FromRequest(r).
				Error().Err(err).
				Str("token_type", string(claims.TokenType)).
				Int64("principal_id", claims.PrincipalID).
				Msg("cannot find principal")
			return nil, fmt.Errorf("failed to get principal for token: %w", err)
		}
		return []byte(principal.Salt), nil
	})
	if err != nil {
		return nil, err
	}

	if !parsed.Valid {
		return nil, errors.New("invalid token")
	}

	if _, ok := parsed.Method.(*jwt.SigningMethodHMAC); !ok {
		return nil, errors.New("invalid token")
	}

	// ensure tkn exists
	tkn, err := a.tokenStore.Find(ctx, claims.TokenID)
	if err != nil {
		return nil, fmt.Errorf("token wasn't found: %w", err)
	}

	return &auth.Session{
		Principal: *principal,
		Metadata: &auth.TokenMetadata{
			TokenType: tkn.Type,
			TokenID:   tkn.ID,
			Grants:    tkn.Grants,
		},
	}, nil
}

func (a *TokenAuthenticator) getPrincipal(ctx context.Context, claims *token.JWTClaims) (*types.Principal, error) {
	switch claims.TokenType {
	case enum.TokenTypePAT, enum.TokenTypeSession, enum.TokenTypeOAuth2:
		user, err := a.userStore.Find(ctx, claims.PrincipalID)
		if err != nil {
			return nil, err
		}

		return types.PrincipalFromUser(user), nil

	case enum.TokenTypeSAT:
		sa, err := a.saStore.Find(ctx, claims.PrincipalID)
		if err != nil {
			return nil, err
		}

		return types.PrincipalFromServiceAccount(sa), nil

	default:
		return nil, fmt.Errorf("unsupported token type '%s'", claims.TokenType)
	}
}

func extractToken(r *http.Request) string {
	bearer := r.Header.Get("Authorization")
	if bearer == "" {
		return r.FormValue("access_token")
	}
	// pull/push git operations will require auth using
	// Basic realm
	if strings.HasPrefix(bearer, "Basic") {
		_, tkn, ok := r.BasicAuth()
		if !ok {
			return ""
		}
		return tkn
	}
	bearer = strings.TrimPrefix(bearer, "Bearer ")
	bearer = strings.TrimPrefix(bearer, "IdentityService ")
	return bearer
}
