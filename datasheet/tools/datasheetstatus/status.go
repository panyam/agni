package main

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
)

// PDFStatus classifies one datasheet PDF's Stage-A (PDF -> doc-IR) extraction freshness (WS13-009).
type PDFStatus string

const (
	// NotExtracted means the PDF has no doc-IR sibling yet, a normal starting state.
	NotExtracted PDFStatus = "not-extracted"
	// Fresh means the doc-IR's stored source hash matches the PDF bytes and, when the current
	// toolchain is known, that toolchain produced it.
	Fresh PDFStatus = "fresh"
	// StaleSource means the PDF bytes no longer match the doc-IR's stored content_hash.
	StaleSource PDFStatus = "stale-source"
	// StaleToolchain means the PDF is unchanged but a different toolchain version produced the
	// doc-IR, so re-extraction may improve it. Reported only when the current toolchain is known.
	StaleToolchain PDFStatus = "stale-toolchain"
)

// needsExtraction reports whether pdf2doc-all should (re)run pdf2doc on a PDF in this status.
// stale-toolchain is excluded, so a toolchain bump re-extracts the whole corpus only when asked to.
func (s PDFStatus) needsExtraction() bool {
	return s == NotExtracted || s == StaleSource
}

// rank orders statuses most-needs-attention first, so a part's rollup takes the worst of its PDFs.
func (s PDFStatus) rank() int {
	switch s {
	case NotExtracted:
		return 0
	case StaleSource:
		return 1
	case StaleToolchain:
		return 2
	default: // Fresh
		return 3
	}
}

// classify decides a PDF's status from its bytes hash and its doc-IR sibling's stored facts.
// pdfHash and storedHash are both in the "sha256:..." form, and storedProducer is the doc-IR's
// Document.producer. curToolchain is the producer the installed toolchain would stamp now (e.g.
// "docling/2.5.1"); empty means unknown, which disables toolchain drift but not hash freshness.
func classify(docExists bool, pdfHash, storedHash, storedProducer, curToolchain string) PDFStatus {
	if !docExists {
		return NotExtracted
	}
	if pdfHash != storedHash {
		return StaleSource
	}
	if curToolchain != "" && storedProducer != curToolchain {
		return StaleToolchain
	}
	return Fresh
}

// hashPDF returns the "sha256:"-prefixed hex digest of a file's bytes. It must match how
// tools/pdf2doc stamps Document.content_hash, or every extracted file reads stale-source.
func hashPDF(path string) (string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(b)
	return "sha256:" + hex.EncodeToString(sum[:]), nil
}
