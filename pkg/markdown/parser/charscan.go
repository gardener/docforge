package parser

import "bytes"

// like skipChar but only skips up to max characters
func skipCharN(data []byte, i int, c byte, max int) int {
	n := len(data)
	for i < n && max > 0 && data[i] == c {
		i++
		max--
	}
	return i
}

func skipAlnum(data []byte, i int) int {
	n := len(data)
	for i < n && isAlnum(data[i]) {
		i++
	}
	return i
}

func skipSpace(data []byte, i int) int {
	n := len(data)
	for i < n && isSpace(data[i]) {
		i++
	}
	return i
}

// skipUntilChar advances i as long as data[i] != c
func skipUntilChar(data []byte, i int, c byte) int {
	n := len(data)
	for i < n && data[i] != c {
		i++
	}
	return i
}

// skipUntilCharBackwards traces back i as long as data[i] != c
func skipUntilCharBackwards(data []byte, i int, c byte) int {
	for i >= 0 && data[i] != c {
		i--
	}
	return i
}

// isSpace returns true if c is a white-space character
func isSpace(c byte) bool {
	return c == ' ' || c == '\t' || c == '\n' || c == '\r' || c == '\f' || c == '\v'
}

// isLetter returns true if c is ascii letter
func isLetter(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
}

// isAlnum returns true if c is a digit or letter
// TODO: check when this is looking for ASCII alnum and when it should use unicode
func isAlnum(c byte) bool {
	return (c >= '0' && c <= '9') || isLetter(c)
}

func unescapeText(ob *bytes.Buffer, src []byte) {
	i := 0
	for i < len(src) {
		org := i
		for i < len(src) && src[i] != '\\' {
			i++
		}

		if i > org {
			ob.Write(src[org:i])
		}

		if i+1 >= len(src) {
			break
		}

		ob.WriteByte(src[i+1])
		i += 2
	}
}
