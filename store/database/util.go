// Copyright 2022 Harness Inc. All rights reserved.
// Use of this source code is governed by the Polyform Free Trial License
// that can be found in the LICENSE.md file for this repository.

package database

import (
	"database/sql"
	"fmt"

	"github.com/harness/gitness/store"

	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
)

// default query range limit.
const defaultLimit = 100

// limit returns the page size to a sql limit.
func Limit(size int) uint64 {
	if size == 0 {
		size = defaultLimit
	}
	return uint64(size)
}

// offset converts the page to a sql offset.
func Offset(page, size int) uint64 {
	if page == 0 {
		page = 1
	}
	if size == 0 {
		size = defaultLimit
	}
	page--
	return uint64(page * size)
}

// Logs the error and message, returns either the provided message or a gitrpc equivalent if possible.
// Always logs the full message with error as warning.
//
//nolint:unparam // revisit error processing
func ProcessSQLErrorf(err error, format string, args ...interface{}) error {
	// create fallback error returned if we can't map it
	fallbackErr := fmt.Errorf(format, args...)

	// always log internal error together with message.
	log.Debug().Msgf("%v: [SQL] %v", fallbackErr, err)

	// If it's a known error, return converted error instead.
	switch {
	case errors.Is(err, sql.ErrNoRows):
		return store.ErrResourceNotFound
	case isSQLUniqueConstraintError(err):
		return store.ErrDuplicate
	default:
		return fallbackErr
	}
}