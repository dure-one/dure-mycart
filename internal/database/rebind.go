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
			// Quoted region: copy verbatim. A doubled quote is an escaped
			// quote, not the end of the region.
			b.WriteByte(c)
			for i++; i < len(query); i++ {
				b.WriteByte(query[i])
				if query[i] != c {
					continue
				}
				if i+1 < len(query) && query[i+1] == c {
					i++
					b.WriteByte(query[i])
					continue
				}
				break
			}

		case c == '-' && i+1 < len(query) && query[i+1] == '-':
			// Line comment.
			for ; i < len(query) && query[i] != '\n'; i++ {
				b.WriteByte(query[i])
			}
			if i < len(query) {
				b.WriteByte(query[i])
			}

		case c == '/' && i+1 < len(query) && query[i+1] == '*':
			// Block comment.
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
