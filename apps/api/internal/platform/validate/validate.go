// SPDX-License-Identifier: AGPL-3.0-only
// Copyright (C) 2026 madarlevi

// Package validate collects field errors in the reason vocabulary of the contract
// (ErrorDetail.reason) and normalizes user-supplied text.
package validate

import (
	"strings"
	"unicode"
	"unicode/utf8"

	"golang.org/x/text/unicode/norm"

	"github.com/madarlevi/kolikaja/apps/api/internal/platform/apierror"
)

// Collector accumulates field errors.
type Collector struct {
	details []apierror.Detail
}

// Add records a field error.
func (c *Collector) Add(field, reason string) {
	c.details = append(c.details, apierror.Detail{Field: field, Reason: reason})
}

// Err returns a 422 error when any field error was recorded, otherwise nil.
func (c *Collector) Err() error {
	if len(c.details) == 0 {
		return nil
	}
	return apierror.Validation(c.details...)
}

// Text normalizes s to NFC and trims surrounding white space.
func Text(s string) string {
	return strings.TrimSpace(norm.NFC.String(s))
}

// Length checks the length of s in Unicode code points. Control characters other than
// newlines and tabs are rejected when multiline is true; all control characters when false.
func (c *Collector) Length(field, s string, minLen, maxLen int, multiline bool) {
	n := utf8.RuneCountInString(s)
	switch {
	case !utf8.ValidString(s):
		c.Add(field, "invalid_format")
	case n < minLen:
		c.Add(field, "too_short")
	case n > maxLen:
		c.Add(field, "too_long")
	default:
		for _, r := range s {
			if invisible(r) || (unicode.IsControl(r) && (!multiline || (r != '\n' && r != '\r' && r != '\t'))) {
				c.Add(field, "invalid_format")
				return
			}
		}
	}
}

// OneOf checks that s is one of the allowed values.
func (c *Collector) OneOf(field, s string, allowed ...string) {
	for _, a := range allowed {
		if s == a {
			return
		}
	}
	c.Add(field, "invalid_value")
}

// invisible reports bidirectional controls and zero-width characters that can make text
// look different from what it is (spoofed names). U+200D (zero-width joiner) stays allowed
// because emoji sequences need it.
func invisible(r rune) bool {
	switch {
	case r >= 0x202A && r <= 0x202E, r >= 0x2066 && r <= 0x2069:
		return true
	case r == 0x061C, r == 0x200B, r == 0x200E, r == 0x200F, r == 0x2060, r == 0xFEFF:
		return true
	}
	return false
}
