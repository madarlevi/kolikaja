// SPDX-License-Identifier: AGPL-3.0-only
// Copyright (C) 2026 madarlevi

// Package security is a black-box harness that sends SQL injection and XSS payloads to
// every operation of the OpenAPI contract and checks that the API answers safely.
// See docs/SECURITY-PLAN.md. The request tests are skipped until a running API is
// available (KOLIKAJA_API_URL); the inventory tests always run.
package security

// Kind classifies a payload.
type Kind string

// Payload kinds.
const (
	SQLInjection Kind = "sqli"
	XSS          Kind = "xss"
)

// Payload is one malicious input value.
type Payload struct {
	Kind  Kind
	Name  string
	Value string
}

// Payloads lists the inputs sent to every injection point. They target PostgreSQL
// (classic tautologies, stacked queries, comments, time-based and error-based probes,
// casts) and HTML/JavaScript contexts (script tags, event handlers, URLs, templates).
var Payloads = []Payload{
	{SQLInjection, "tautology-quote", `' OR '1'='1`},
	{SQLInjection, "tautology-comment", `' OR 1=1--`},
	{SQLInjection, "tautology-double-quote", `" OR ""="`},
	{SQLInjection, "numeric-tautology", `1 OR 1=1`},
	{SQLInjection, "stacked-drop", `'; DROP TABLE users; --`},
	{SQLInjection, "union-select", `' UNION SELECT NULL, version()--`},
	{SQLInjection, "time-based", `'; SELECT pg_sleep(5)--`},
	{SQLInjection, "error-based-cast", `' AND 1=CAST((SELECT version()) AS int)--`},
	{SQLInjection, "comment-terminator", `admin'/*`},
	{SQLInjection, "dollar-quote", `$$; SELECT 1; $$`},
	{SQLInjection, "uuid-tail", `00000000-0000-7000-8000-000000000000' OR '1'='1`},
	{SQLInjection, "null-byte", "abc\x00' OR '1'='1"},
	{SQLInjection, "backslash-quote", `\' OR 1=1 --`},
	{XSS, "script-tag", `<script>alert(1)</script>`},
	{XSS, "img-onerror", `"><img src=x onerror=alert(1)>`},
	{XSS, "svg-onload", `<svg/onload=alert(1)>`},
	{XSS, "javascript-url", `javascript:alert(1)`},
	{XSS, "attribute-break", `' autofocus onfocus=alert(1) x='`},
	{XSS, "iframe-srcdoc", `<iframe srcdoc="<script>alert(1)</script>">`},
	{XSS, "template-expression", `{{constructor.constructor('alert(1)')()}}`},
	{XSS, "encoded-script", `%3Cscript%3Ealert(1)%3C%2Fscript%3E`},
	{XSS, "unicode-escape", `<script>alert(1)</script>`},
}

// SQLErrorMarkers are fragments that must never appear in a response body; their
// presence means database details leaked (CLAUDE.md, "Errors").
var SQLErrorMarkers = []string{
	"SQLSTATE",
	"syntax error at or near",
	"unterminated quoted string",
	"pq: ",
	"pgx",
	"pgconn",
	"ERROR: ",
	"invalid input syntax for type",
	"panic:",
	"goroutine ",
	".go:",
}
