// Package latin1 decodes single-byte text, as binary EDA formats (Allegro, Altium) store names, into
// a Go string. Each byte is its own code point, which is ISO-8859-1.
package latin1

// Decode turns ISO-8859-1 bytes into a string.
func Decode(b []byte) string {
	r := make([]rune, len(b))
	for i, c := range b {
		r[i] = rune(c)
	}
	return string(r)
}
