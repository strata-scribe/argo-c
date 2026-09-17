package webhook

import (
	"bufio"
	"compress/gzip"
	"errors"
	"io"
	"net/http"
	"strings"
)

// ErrPayloadTooLarge indicates the payload exceeds the configured maximum size.
var ErrPayloadTooLarge = errors.New("payload too large")

// PayloadValidator enforces payload size constraints and handles gzip decompression safely.
type PayloadValidator struct {
	MaxSizeBytes int64
}

// NewPayloadValidator creates a new PayloadValidator with the given maximum size in bytes.
func NewPayloadValidator(maxSizeBytes int64) *PayloadValidator {
	return &PayloadValidator{
		MaxSizeBytes: maxSizeBytes,
	}
}

// ValidateAndRead validates the payload size and reads the request body.
// It handles gzip decompression and protects against zip bombs by enforcing the size limit on decompressed data.
func (pv *PayloadValidator) ValidateAndRead(r *http.Request) ([]byte, error) {
	// 1. Fast path: check Content-Length header
	if r.ContentLength > pv.MaxSizeBytes {
		return nil, ErrPayloadTooLarge
	}

	// 2. Read first few bytes to sniff for gzip magic header (0x1f 0x8b)
	br := bufio.NewReader(r.Body)
	header, err := br.Peek(2)
	isGzipSniffed := err == nil && len(header) >= 2 && header[0] == 0x1f && header[1] == 0x8b

	// Also check Content-Encoding header
	isGzipHeader := strings.EqualFold(r.Header.Get("Content-Encoding"), "gzip")

	// 3. Limit the underlying reader to MaxSizeBytes + 1 to detect over-sized requests
	//    without reading the entire potentially massive body into memory.
	limitedUnderlyingReader := io.LimitReader(br, pv.MaxSizeBytes+1)

	var readerToUse io.Reader = limitedUnderlyingReader

	// 4. Handle GZIP decompression if detected
	if isGzipHeader || isGzipSniffed {
		gzr, err := gzip.NewReader(limitedUnderlyingReader)
		if err != nil {
			return nil, err // Invalid gzip payload
		}
		defer gzr.Close()

		// Wrap gzip reader with ANOTHER LimitReader to protect against zip bombs
		// where a small compressed payload expands to a massive uncompressed payload.
		readerToUse = io.LimitReader(gzr, pv.MaxSizeBytes+1)
	}

	// 5. Read the body
	payload, err := io.ReadAll(readerToUse)
	if err != nil && err != io.EOF {
		return nil, err
	}

	// 6. Check if we hit the limit
	if int64(len(payload)) > pv.MaxSizeBytes {
		return nil, ErrPayloadTooLarge
	}

	return payload, nil
}
