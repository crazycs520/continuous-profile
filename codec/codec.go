package codec

import (
	"encoding/binary"
	"errors"
	"fmt"
)

var (
	ProfileKeyPrefix           = []byte("p")
	ProfileKeyPrefixLen        = 1
	ProfileKeySep         byte = 255
	ProfileKeyPrefixIDLen int  = 9
)

type ProfileKey struct {
	Ts       int64
	Job      string
	Tp       string // profile type
	Instance string
}

func (key *ProfileKey) Encode() []byte {
	buf := make([]byte, 0, 16)
	buf = append(buf, ProfileKeyPrefix...)
	buf = EncodeInt(buf, key.Ts)
	buf = appendString(buf, key.Tp)
	buf = appendString(buf, key.Job)
	buf = appendString(buf, key.Instance)
	return buf
}

func (key *ProfileKey) EncodeForRangeQuery() []byte {
	buf := make([]byte, 0, 16)
	buf = append(buf, ProfileKeyPrefix...)
	if key.Ts == 0 {
		return buf
	}
	buf = EncodeInt(buf, key.Ts)
	if len(key.Tp) == 0 {
		return buf
	}
	buf = appendString(buf, key.Tp)
	if len(key.Job) == 0 {
		return buf
	}
	buf = appendString(buf, key.Job)
	if len(key.Instance) == 0 {
		return buf
	}
	buf = appendString(buf, key.Instance)
	return buf
}

func DecodeProfileKey(key []byte) (*ProfileKey, error) {
	if len(key) < ProfileKeyPrefixIDLen {
		return nil, fmt.Errorf("profile key %b is invalid", key)
	}
	key = key[ProfileKeyPrefixLen:]
	key, ts, err := DecodeInt(key)
	if err != nil {
		return nil, err
	}

	fields := decodeStrings(key)
	if len(fields) != 3 {
		return nil, fmt.Errorf("profile key %b is invalid", key)
	}
	return &ProfileKey{
		Ts:       ts,
		Tp:       fields[0],
		Job:      fields[1],
		Instance: fields[2],
	}, nil
}

func appendString(key []byte, v string) []byte {
	if len(key) > 0 {
		key = append(key, ProfileKeySep)
	}
	return append(key, []byte(v)...)
}

func decodeStrings(key []byte) []string {
	result := make([]string, 0, 4)
	w := []byte{}
	for _, b := range key {
		if b == ProfileKeySep {
			if len(w) > 0 {
				result = append(result, string(w))
				w = w[:0]
			}
			continue
		}
		w = append(w, b)
	}
	if len(w) > 0 {
		result = append(result, string(w))
	}
	return result
}

func EncodeInt(b []byte, v int64) []byte {
	var data [8]byte
	binary.BigEndian.PutUint64(data[:], uint64(v))
	return append(b, data[:]...)
}

func DecodeInt(b []byte) ([]byte, int64, error) {
	if len(b) < 8 {
		return nil, 0, errors.New("insufficient bytes to decode value")
	}

	u := binary.BigEndian.Uint64(b[:8])
	b = b[8:]
	return b, int64(u), nil
}
