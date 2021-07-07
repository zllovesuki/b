package app

import (
	"encoding/binary"
	"io"
	"time"

	"github.com/pkg/errors"
)

// here we define the header wire format
const (
	headerSize   = 32
	versionByte  = 0
	createdStart = 1
	createdEnd   = 16
	ttlStart     = 16
	ttlEnd       = 24
	reserved     = 25
)

// WriteTTL will insert ttl info into current position of io.Writer.
// Using this method for unified wire format is strongly preferred
func WriteTTL(w io.Writer, ttl time.Duration) error {
	head := make([]byte, headerSize)
	head[versionByte] = 0

	now, err := time.Now().UTC().MarshalBinary()
	if err != nil {
		return errors.Wrap(err, "error marshalling time into binary")
	}

	copy(head[createdStart:createdEnd], now)
	binary.LittleEndian.PutUint64(head[ttlStart:ttlEnd], uint64(ttl))

	if _, err := w.Write(head); err != nil {
		return errors.Wrap(err, "cannot write expiration data")
	}

	return nil
}

// TTLExceeded will read the ttl info from current position of io.Reader.
// Using this method for unified wire format is strongly preferred
func TTLExceeded(r io.Reader) (bool, error) {
	head := make([]byte, headerSize)
	switch head[versionByte] {
	case 0:
		if _, err := r.Read(head); err != nil {
			return false, errors.Wrap(err, "cannot read expiration data")
		}

		ttl := int64(binary.LittleEndian.Uint64(head[ttlStart:ttlEnd]))

		if ttl == 0 {
			return false, nil
		}

		var ref time.Time
		if err := ref.UnmarshalBinary(head[createdStart:createdEnd]); err != nil {
			return false, errors.Wrap(err, "error unmarshalling binary into time")
		}

		return time.Now().After(ref.Add(time.Duration(ttl))), nil
	default:
		return false, errors.Errorf("uncognized header version: %d", head[versionByte])
	}
}
