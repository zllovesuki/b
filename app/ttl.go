package app

import (
	"encoding/binary"
	"io"
	"time"

	"github.com/pkg/errors"
)

// WriteTTL will insert ttl info into current position of io.Writer.
// Using this method for unified wire format is strongly preferred
func WriteTTL(w io.Writer, ttl time.Duration) error {
	head := make([]byte, 15+8) // 15 bytes for when it was created, 8 bytes for ttl
	now, err := time.Now().UTC().MarshalBinary()
	if err != nil {
		return errors.Wrap(err, "error marshalling time into binary")
	}

	copy(head[:15], now)
	binary.LittleEndian.PutUint64(head[15:23], uint64(ttl))

	if _, err := w.Write(head); err != nil {
		return errors.Wrap(err, "cannot write expiration data")
	}

	return nil
}

// TTLExceeded will read the ttl info from current position of io.Reader.
// Using this method for unified wire format is strongly preferred
func TTLExceeded(r io.Reader) (bool, error) {
	head := make([]byte, 15+8)
	if _, err := r.Read(head); err != nil {
		return false, errors.Wrap(err, "cannot read expiration data")
	}

	ttl := int64(binary.LittleEndian.Uint64(head[15:23]))

	if ttl == 0 {
		return false, nil
	}

	var ref time.Time
	if err := ref.UnmarshalBinary(head[:15]); err != nil {
		return false, errors.Wrap(err, "error unmarshalling binary into time")
	}

	return time.Now().After(ref.Add(time.Duration(ttl))), nil
}
