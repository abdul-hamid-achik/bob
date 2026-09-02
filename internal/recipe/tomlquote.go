package recipe

import (
	"strings"
	"unicode/utf8"
)

// tomlQuote renders s as a TOML basic string. strconv.Quote (registered in
// the template FuncMap as "quote") emits Go-only escapes such as \a, \v, and
// lowercase \xHH/\u forms that TOML basic strings do not accept — TOML only
// allows \b \t \n \f \r \" \\ \uXXXX \UXXXXXXXX. Templates that interpolate
// manifest strings (e.g. product.description, which YAML lets contain \a or
// \v) into a TOML document must use tomlQuote instead of quote so the
// rendered file stays valid TOML.
//
// For printable ASCII input — the common case, and every manifest string
// produced before this helper existed — tomlQuote is byte-identical to
// strconv.Quote, so no previously published recipe output changes for any
// existing manifest.
func tomlQuote(s string) string {
	var b strings.Builder
	b.Grow(len(s) + 2)
	b.WriteByte('"')
	for i := 0; i < len(s); {
		r, width := utf8.DecodeRuneInString(s[i:])
		if r == utf8.RuneError && width == 1 {
			b.WriteRune(0xFFFD)
			i++
			continue
		}
		switch r {
		case '"':
			b.WriteString(`\"`)
		case '\\':
			b.WriteString(`\\`)
		case '\b':
			b.WriteString(`\b`)
		case '\t':
			b.WriteString(`\t`)
		case '\n':
			b.WriteString(`\n`)
		case '\f':
			b.WriteString(`\f`)
		case '\r':
			b.WriteString(`\r`)
		default:
			if r <= 0x1F || r == 0x7F {
				b.WriteString(`\u`)
				b.WriteString(tomlQuoteUpperHex4(uint32(r)))
			} else {
				b.WriteRune(r)
			}
		}
		i += width
	}
	b.WriteByte('"')
	return b.String()
}

// tomlQuoteUpperHex4 renders v as four uppercase hex digits, as required by
// TOML's \uXXXX escape.
func tomlQuoteUpperHex4(v uint32) string {
	const hex = "0123456789ABCDEF"
	return string([]byte{
		hex[(v>>12)&0xF],
		hex[(v>>8)&0xF],
		hex[(v>>4)&0xF],
		hex[v&0xF],
	})
}
