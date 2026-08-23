package idgen

import (
	"crypto/rand"
	"fmt"
	"log/slog"
)

func GenerateUUID() string {
	b := make([]byte, 16)
	n, err := rand.Read(b)
	if err != nil {
		slog.Error("generate uuid rand read failed", "error", err)
	}
	if n != 16 {
		slog.Error("generate uuid short read", "n", n)
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}
