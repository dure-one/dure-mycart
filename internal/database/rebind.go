package database

import (
	"strconv"
	"strings"
)

// rebindDollar rewrites `?` placeholders into PostgreSQL's numbered `$1..$n`
// form, preserving the left-to-right order that matches the argument slice.
//
// This is a state machine rather than a regexp because a `?` can legitimately
// appear inside a string literal, a quoted identifier or a comment, and
// rewriting those would corrupt the query. The query text itself is never
// changed for SQLite, whose driver consumes `?` natively.
func rebindDollar(query string) string {
	var b strings.Builder
	b.Grow(len(query) + 8)

	n := 0
	for i := 0; i < len(query); i++ {
		c := query[i]

		switch {
		case c == '\'' || c == '"' || c == '`':
			i = copyQuoted(&b, query, i)

		case strings.HasPrefix(query[i:], "--"):
			i = copyLineComment(&b, query, i)

		case strings.HasPrefix(query[i:], "/*"):
			i = copyBlockComment(&b, query, i)

		case c == '?':
			n++
			b.WriteByte('$')
			b.WriteString(strconv.Itoa(n))

		default:
			b.WriteByte(c)
		}
	}

	return b.String()
}

// copyQuoted copies verbatim the quoted region that opens at query[i] — a string
// literal, a quoted identifier, or a backtick-quoted one — and returns the index
// of its closing quote. A doubled quote is an escaped quote and does not end the
// region.
func copyQuoted(b *strings.Builder, query string, i int) int {
	quote := query[i]
	b.WriteByte(quote)

	for i++; i < len(query); i++ {
		b.WriteByte(query[i])
		if query[i] != quote {
			continue
		}
		if i+1 < len(query) && query[i+1] == quote {
			i++
			b.WriteByte(query[i])
			continue
		}
		break
	}

	return i
}

// copyLineComment copies the `--` comment that starts at query[i], up to and
// including the newline that ends it, and returns the index it stopped at.
func copyLineComment(b *strings.Builder, query string, i int) int {
	for ; i < len(query) && query[i] != '\n'; i++ {
		b.WriteByte(query[i])
	}
	if i < len(query) {
		b.WriteByte(query[i])
	}
	return i
}

// copyBlockComment copies the `/* ... */` comment that starts at query[i] and
// returns the index of the slash that closes it.
func copyBlockComment(b *strings.Builder, query string, i int) int {
	b.WriteString("/*")
	i += 2
	for i < len(query) && !(query[i] == '*' && i+1 < len(query) && query[i+1] == '/') {
		b.WriteByte(query[i])
		i++
	}
	if i < len(query) {
		b.WriteString("*/")
		i++
	}
	return i
}
