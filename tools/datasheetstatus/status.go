package main

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
)

// PDFStatus classifies one datasheet PDF's Stage-A (PDF -> doc-IR) extraction freshness. It is
// only about the pdf2doc extraction step; recipe/patch changes are a derive-stage concern and do
// not make a doc-IR stale (WS13-009).
type PDFStatus string

const (
	// NotExtracted: the PDF has no doc-IR sibling yet. A normal starting state, not an error.
	NotExtracted PDFStatus = "not-extracted"
	// Fresh: a doc-IR exists, its stored source hash still matches the PDF bytes, and (when the
	// current toolchain is known) it was produced by that toolchain.
	Fresh PDFStatus = "fresh"
	// StaleSource: a doc-IR exists but the PDF bytes changed since it was produced (re-hash
	// disagrees with the stored content_hash), so the extraction no longer describes the file.
	StaleSource PDFStatus = "stale-source"
	// StaleToolchain: the PDF is unchanged but the doc-IR was produced by a different toolchain
	// version than the one installed now, so re-extraction may improve it. Only reported when the
	// current toolchain is known.
	StaleToolchain PDFStatus = "stale-toolchain"
)

// needsExtraction reports whether pdf2doc-all should (re)run pdf2doc on a PDF in this status.
// stale-toolchain is deliberately excluded: a toolchain bump re-running the whole corpus should
// be an explicit choice, not an implicit consequence of a status sweep.
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

// classify decides a PDF's status from its on-disk bytes hash and its doc-IR sibling's stored
// facts. docExists is false when no sibling doc-IR was found. pdfHash and storedHash are both the
// "sha256:..." prefixed form; storedProducer is the doc-IR Document.producer. curToolchain is the
// producer string the installed toolchain would stamp now (e.g. "docling/2.5.1"); an empty
// curToolchain means the version is unknown, so toolchain drift is not reported while hash
// freshness still is.
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

// hashPDF returns the "sha256:"-prefixed hex digest of a file's bytes, matching exactly how
// tools/pdf2doc stamps Document.content_hash (sha256 over the raw source bytes). The prefix and
// algorithm must stay in lock-step with pdf2doc or every extracted file would read stale-source.
func hashPDF(path string) (string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(b)
	return "sha256:" + hex.EncodeToString(sum[:]), nil
}
