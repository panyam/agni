package odb

import (
	"errors"
)

// uncompress decodes the Unix compress(1) format (`.Z`). Valor-lineage ODB++ producers use it for a
// job's larger members (`features.z`). It is LZW over variable-width codes. Codes start at 9 bits and
// grow to the header's maximum, and in block mode code 256 clears the table. Go's compress/lzw
// speaks a different framing (GIF and TIFF), so this is the classic decoder written out.
func uncompress(b []byte) ([]byte, error) {
	if len(b) < 3 || b[0] != 0x1f || b[1] != 0x9d {
		return nil, errors.New("not compress(1) data")
	}
	maxBits := int(b[2] & 0x1f)
	block := b[2]&0x80 != 0
	if maxBits < 9 || maxBits > 16 {
		return nil, errors.New("compress(1): bad maximum code width")
	}
	data := b[3:]

	const clearCode = 256
	prefix := make([]int, 1<<maxBits)
	suffix := make([]byte, 1<<maxBits)
	for i := 0; i < 256; i++ {
		suffix[i] = byte(i)
	}
	first := 256
	if block {
		first = 257
	}
	next := first
	bits := 9
	// maxCode is the largest code the current width holds before it grows. compress(1) grows the
	// width when the next free entry passes maxCode. At the top width it sets maxCode to 1<<maxBits,
	// and no entry passes that. Comparing against maxCode rather than against maxBits matters at -b
	// 9. There the table fills at 512, which passes 511. So the codes grow to 10 bits, though the
	// table never does. For the same reason the starting width is never treated as the top one.
	maxCode := 1<<bits - 1
	setWidth := func(b int) {
		bits = b
		if bits == maxBits {
			maxCode = 1 << maxBits
		} else {
			maxCode = 1<<bits - 1
		}
	}
	var out []byte
	stack := make([]byte, 0, 1<<maxBits)
	old := -1
	var finChar byte

	// Codes are read in groups of `bits` codes, because compress(1) pads its output that way. When
	// the width changes or the table clears, the rest of the current group's bytes are skipped.
	bitPos := 0
	groupStart := 0
	readCode := func() (int, bool) {
		if bitPos+bits > len(data)*8 {
			return 0, false
		}
		code := 0
		for i := 0; i < bits; i++ {
			byteIdx := (bitPos + i) >> 3
			if data[byteIdx]&(1<<uint((bitPos+i)&7)) != 0 {
				code |= 1 << uint(i)
			}
		}
		bitPos += bits
		return code, true
	}
	skipGroup := func() {
		// Advance to the end of the current group of `bits` codes (bits bytes).
		used := bitPos - groupStart
		groupBits := bits * 8
		if rem := used % groupBits; rem != 0 {
			bitPos += groupBits - rem
		}
		groupStart = bitPos
	}

	for {
		if next > maxCode {
			skipGroup()
			setWidth(bits + 1)
		}
		code, ok := readCode()
		if !ok {
			break
		}
		if old == -1 {
			if code >= 256 {
				return nil, errors.New("compress(1): bad first code")
			}
			old = code
			finChar = suffix[code]
			out = append(out, finChar)
			continue
		}
		if block && code == clearCode {
			// compress(1) keeps the previous code across a clear and sets the next free entry to
			// 256. So the first code after the clear writes a throwaway entry there, and real
			// entries resume at 257.
			skipGroup()
			next = first - 1
			maxCode = 1<<9 - 1
			bits = 9
			continue
		}
		in := code
		stack = stack[:0]
		if code >= next {
			if code > next {
				return nil, errors.New("compress(1): corrupt data")
			}
			stack = append(stack, finChar)
			code = old
		}
		for code >= 256 {
			stack = append(stack, suffix[code])
			code = prefix[code]
		}
		finChar = suffix[code]
		stack = append(stack, finChar)
		for i := len(stack) - 1; i >= 0; i-- {
			out = append(out, stack[i])
		}
		if len(out) > maxArchiveBytes {
			return nil, errors.New("compress(1): expands past 1 GiB")
		}
		if next < 1<<maxBits {
			prefix[next] = old
			suffix[next] = finChar
			next++
		}
		old = in
	}
	return out, nil
}
