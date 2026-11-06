package media

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"time"
)

// wavPart is one parsed RIFF/WAVE file: its fmt chunk and its sample data.
type wavPart struct {
	format []byte
	data   []byte
}

// joinWAV joins WAV files of one format into a single file, with gap of silence
// between them. Appending the files' bytes would leave every header but the
// first inside the audio, and players stop at the first file's declared length.
func joinWAV(parts [][]byte, gap time.Duration) ([]byte, error) {
	if len(parts) == 0 {
		return nil, errors.New("wav: nothing to join")
	}
	parsed := make([]wavPart, 0, len(parts))
	for i, part := range parts {
		p, err := parseWAV(part)
		if err != nil {
			return nil, fmt.Errorf("wav: part %d: %w", i, err)
		}
		if i > 0 && !bytes.Equal(p.format[:16], parsed[0].format[:16]) {
			return nil, fmt.Errorf("wav: part %d has a different sample format", i)
		}
		parsed = append(parsed, p)
	}

	format := parsed[0].format
	byteRate := binary.LittleEndian.Uint32(format[8:12])
	blockAlign := int(binary.LittleEndian.Uint16(format[12:14]))
	bitsPerSample := binary.LittleEndian.Uint16(format[14:16])
	silence := silenceBytes(byteRate, blockAlign, bitsPerSample, gap)

	var data bytes.Buffer
	for i, p := range parsed {
		if i > 0 {
			data.Write(silence)
		}
		data.Write(p.data)
	}
	if data.Len() > math.MaxUint32-64 {
		return nil, errors.New("wav: joined audio is too long")
	}

	var out bytes.Buffer
	riffSize := 4 + (8 + len(format)) + (8 + data.Len())
	out.WriteString("RIFF")
	writeUint32(&out, uint32(riffSize)) //nolint:gosec // G115: bounded by the check above
	out.WriteString("WAVE")
	out.WriteString("fmt ")
	writeUint32(&out, uint32(len(format))) //nolint:gosec // G115: a fmt chunk is a few dozen bytes
	out.Write(format)
	out.WriteString("data")
	writeUint32(&out, uint32(data.Len())) //nolint:gosec // G115: bounded by the check above
	out.Write(data.Bytes())
	return out.Bytes(), nil
}

func parseWAV(file []byte) (wavPart, error) {
	if len(file) < 12 || string(file[0:4]) != "RIFF" || string(file[8:12]) != "WAVE" {
		return wavPart{}, errors.New("not a RIFF/WAVE file")
	}
	var part wavPart
	offset := 12
	for offset+8 <= len(file) {
		id := string(file[offset : offset+4])
		size := int(binary.LittleEndian.Uint32(file[offset+4 : offset+8]))
		start := offset + 8
		// A streaming writer may leave the size unset; the chunk then runs to the end.
		end := start + size
		if size < 0 || end > len(file) || end < start {
			end = len(file)
		}
		switch id {
		case "fmt ":
			part.format = file[start:end]
		case "data":
			part.data = file[start:end]
		}
		offset = end + (end-start)%2
	}
	if len(part.format) < 16 {
		return wavPart{}, errors.New("no fmt chunk")
	}
	if part.data == nil {
		return wavPart{}, errors.New("no data chunk")
	}
	return part, nil
}

// silenceBytes is gap of silence in the format: zeros for signed PCM, 0x80 for 8-bit.
func silenceBytes(byteRate uint32, blockAlign int, bitsPerSample uint16, gap time.Duration) []byte {
	if blockAlign <= 0 || gap <= 0 {
		return nil
	}
	n := int(int64(byteRate) * int64(gap) / int64(time.Second))
	n -= n % blockAlign
	silence := make([]byte, n)
	if bitsPerSample == 8 {
		for i := range silence {
			silence[i] = 0x80
		}
	}
	return silence
}

func writeUint32(buf *bytes.Buffer, v uint32) {
	var b [4]byte
	binary.LittleEndian.PutUint32(b[:], v)
	buf.Write(b[:])
}
