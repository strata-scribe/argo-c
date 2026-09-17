package webhook

import (
	"bytes"
	"compress/gzip"
	"net/http"
	"strings"
	"testing"
)

func TestPayloadValidator_ValidateAndRead(t *testing.T) {
	const maxSize = 100

	validator := NewPayloadValidator(maxSize)

	tests := []struct {
		name          string
		payload       []byte
		contentLength int64
		gzipCompress  bool
		setGzipHeader bool
		wantErr       error
		wantBody      []byte
	}{
		{
			name:          "Normal uncompressed within limits",
			payload:       []byte("hello world"),
			contentLength: -1, // Simulate missing content length
			wantErr:       nil,
			wantBody:      []byte("hello world"),
		},
		{
			name:          "Uncompressed exceeding limits (content length check)",
			payload:       make([]byte, maxSize+10),
			contentLength: maxSize + 10,
			wantErr:       ErrPayloadTooLarge,
			wantBody:      nil,
		},
		{
			name:          "Uncompressed exceeding limits (read check)",
			payload:       make([]byte, maxSize+10),
			contentLength: -1,
			wantErr:       ErrPayloadTooLarge,
			wantBody:      nil,
		},
		{
			name:          "Compressed within limits",
			payload:       []byte("this is a compressed test payload"),
			contentLength: -1,
			gzipCompress:  true,
			wantErr:       nil,
			wantBody:      []byte("this is a compressed test payload"),
		},
		{
			name:          "Compressed exceeding limits (zip bomb)",
			payload:       make([]byte, maxSize+10), // The decompressed payload is larger than limit
			contentLength: -1,
			gzipCompress:  true,
			wantErr:       ErrPayloadTooLarge,
			wantBody:      nil,
		},
		{
			name:          "Invalid gzip payload (but has header)",
			payload:       []byte("this is not gzip"),
			contentLength: -1,
			setGzipHeader: true,
			wantErr:       gzip.ErrHeader,
			wantBody:      nil,
		},
		{
			name:          "Sniffed gzip",
			payload:       []byte("this is a compressed test payload"),
			contentLength: -1,
			gzipCompress:  true, // Will create valid gzip, sniff should catch it even without header
			setGzipHeader: false,
			wantErr:       nil,
			wantBody:      []byte("this is a compressed test payload"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var bodyBytes []byte
			if tt.gzipCompress {
				var buf bytes.Buffer
				zw := gzip.NewWriter(&buf)
				_, err := zw.Write(tt.payload)
				if err != nil {
					t.Fatalf("failed to compress payload: %v", err)
				}
				if err := zw.Close(); err != nil {
					t.Fatalf("failed to close gzip writer: %v", err)
				}
				bodyBytes = buf.Bytes()
			} else {
				bodyBytes = tt.payload
			}

			req, err := http.NewRequest(http.MethodPost, "/", bytes.NewReader(bodyBytes))
			if err != nil {
				t.Fatalf("failed to create request: %v", err)
			}

			if tt.contentLength >= 0 {
				req.ContentLength = tt.contentLength
			}

			if tt.setGzipHeader {
				req.Header.Set("Content-Encoding", "gzip")
			}

			gotBody, err := validator.ValidateAndRead(req)

			if tt.wantErr != nil {
				// We expect an error
				if err == nil {
					t.Fatalf("expected error %v, got nil", tt.wantErr)
				}
				// For gzip.ErrHeader it might be wrapped or just matched. Let's just check strings if it's not exact match.
				if err != tt.wantErr && !strings.Contains(err.Error(), tt.wantErr.Error()) {
					t.Errorf("expected error %v, got %v", tt.wantErr, err)
				}
			} else {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if !bytes.Equal(gotBody, tt.wantBody) {
					t.Errorf("expected body %q, got %q", tt.wantBody, gotBody)
				}
			}
		})
	}
}

// Ensure the validator doesn't consume unbounded memory on a true zip bomb
func TestPayloadValidator_ZipBombMitigation(t *testing.T) {
	const maxSize = 2000 // Increased maxSize so we can fit a highly compressed block (around 1055 bytes)
	validator := NewPayloadValidator(maxSize)

	// Create a payload of zeros that compresses very well
	largeUncompressed := make([]byte, 1024*1024) // 1MB

	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	_, _ = zw.Write(largeUncompressed)
	_ = zw.Close()

	compressedBytes := buf.Bytes()

	// Ensure the compressed size is actually small
	if int64(len(compressedBytes)) > maxSize {
		t.Fatalf("Compressed payload is too large for this test: %d bytes", len(compressedBytes))
	}

	req, _ := http.NewRequest(http.MethodPost, "/", bytes.NewReader(compressedBytes))
	req.Header.Set("Content-Encoding", "gzip")

	_, err := validator.ValidateAndRead(req)
	if err != ErrPayloadTooLarge {
		t.Errorf("expected ErrPayloadTooLarge, got %v", err)
	}
}
