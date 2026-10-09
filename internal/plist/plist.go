// Package plist reads Apple property lists.
//
// Two encodings are in the wild and both are read here: the XML one, and the
// binary one (`bplist00`) that macOS actually writes for a bundle's
// Info.plist. Only reading is implemented, and only the value types a
// property list can hold: dictionaries, arrays, strings, integers, reals,
// booleans, dates and data.
//
// The decoder is deliberately small and strict: a malformed document is an
// error, never a partly-filled map, because a caller (an application's
// display name, say) would rather fall back than show half a name.
package plist

import (
	"bytes"
	"encoding/binary"
	"encoding/xml"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"
)

// ErrInvalid is what a document that is not a property list reports.
var ErrInvalid = errors.New("plist: not a property list")

// Parse reads a property list in either encoding. The value is one of
// map[string]any, []any, string, int64, float64, bool, time.Time or []byte.
func Parse(data []byte) (any, error) {
	trimmed := bytes.TrimSpace(data)
	switch {
	case bytes.HasPrefix(trimmed, []byte("bplist")):
		return parseBinary(trimmed)
	case bytes.HasPrefix(trimmed, []byte("<?xml")), bytes.HasPrefix(trimmed, []byte("<plist")), bytes.HasPrefix(trimmed, []byte("<!")):
		return parseXML(trimmed)
	default:
		return nil, fmt.Errorf("%w: unknown header", ErrInvalid)
	}
}

// String reads a dictionary's string value, empty when it is missing or is
// not a string.
func String(value any, key string) string {
	record, ok := value.(map[string]any)
	if !ok {
		return ""
	}
	text, _ := record[key].(string)
	return text
}

// ---------------------------------------------------------------- XML

// parseXML reads the XML encoding: a plist element holding exactly one value.
func parseXML(data []byte) (any, error) {
	decoder := xml.NewDecoder(bytes.NewReader(data))
	decoder.Strict = false
	for {
		token, err := decoder.Token()
		if err != nil {
			return nil, fmt.Errorf("%w: %v", ErrInvalid, err)
		}
		start, ok := token.(xml.StartElement)
		if !ok {
			continue
		}
		if start.Name.Local != "plist" {
			continue
		}
		return readXMLElement(decoder)
	}
}

// readXMLElement reads the next value element.
func readXMLElement(decoder *xml.Decoder) (any, error) {
	for {
		token, err := decoder.Token()
		if err != nil {
			return nil, fmt.Errorf("%w: %v", ErrInvalid, err)
		}
		start, ok := token.(xml.StartElement)
		if !ok {
			continue
		}
		switch start.Name.Local {
		case "dict":
			return readXMLDict(decoder)
		case "array":
			return readXMLArray(decoder)
		case "string":
			return readXMLText(decoder)
		case "integer":
			text, err := readXMLText(decoder)
			if err != nil {
				return nil, err
			}
			number, err := strconv.ParseInt(strings.TrimSpace(text), 10, 64)
			if err != nil {
				return nil, fmt.Errorf("%w: integer %q", ErrInvalid, text)
			}
			return number, nil
		case "real":
			text, err := readXMLText(decoder)
			if err != nil {
				return nil, err
			}
			number, err := strconv.ParseFloat(strings.TrimSpace(text), 64)
			if err != nil {
				return nil, fmt.Errorf("%w: real %q", ErrInvalid, text)
			}
			return number, nil
		case "true":
			if err := skipXMLElement(decoder); err != nil {
				return nil, err
			}
			return true, nil
		case "false":
			if err := skipXMLElement(decoder); err != nil {
				return nil, err
			}
			return false, nil
		case "date":
			text, err := readXMLText(decoder)
			if err != nil {
				return nil, err
			}
			parsed, err := time.Parse(time.RFC3339, strings.TrimSpace(text))
			if err != nil {
				return nil, fmt.Errorf("%w: date %q", ErrInvalid, text)
			}
			return parsed, nil
		case "data":
			text, err := readXMLText(decoder)
			if err != nil {
				return nil, err
			}
			return []byte(strings.TrimSpace(text)), nil
		}
	}
}

// readXMLDict reads a dictionary element.
func readXMLDict(decoder *xml.Decoder) (any, error) {
	dict := map[string]any{}
	key := ""
	for {
		token, err := decoder.Token()
		if err != nil {
			return nil, fmt.Errorf("%w: %v", ErrInvalid, err)
		}
		switch element := token.(type) {
		case xml.StartElement:
			if element.Name.Local == "key" {
				text, err := readXMLText(decoder)
				if err != nil {
					return nil, err
				}
				key = text
				continue
			}
			value, err := readXMLValue(decoder, element)
			if err != nil {
				return nil, err
			}
			if key != "" {
				dict[key] = value
				key = ""
			}
		case xml.EndElement:
			if element.Name.Local == "dict" {
				return dict, nil
			}
		}
	}
}

// readXMLValue reads a value element whose start tag the caller has already
// consumed.
func readXMLValue(decoder *xml.Decoder, start xml.StartElement) (any, error) {
	switch start.Name.Local {
	case "dict":
		return readXMLDict(decoder)
	case "array":
		return readXMLArray(decoder)
	case "string":
		return readXMLText(decoder)
	default:
		return readXMLElementAfter(decoder, start)
	}
}

// readXMLElementAfter reads a scalar element whose start tag the caller has.
func readXMLElementAfter(decoder *xml.Decoder, start xml.StartElement) (any, error) {
	switch start.Name.Local {
	case "integer":
		text, err := readXMLText(decoder)
		if err != nil {
			return nil, err
		}
		number, err := strconv.ParseInt(strings.TrimSpace(text), 10, 64)
		if err != nil {
			return nil, fmt.Errorf("%w: integer %q", ErrInvalid, text)
		}
		return number, nil
	case "real":
		text, err := readXMLText(decoder)
		if err != nil {
			return nil, err
		}
		number, err := strconv.ParseFloat(strings.TrimSpace(text), 64)
		if err != nil {
			return nil, fmt.Errorf("%w: real %q", ErrInvalid, text)
		}
		return number, nil
	case "true":
		return true, skipXMLElement(decoder)
	case "false":
		return false, skipXMLElement(decoder)
	case "date":
		text, err := readXMLText(decoder)
		if err != nil {
			return nil, err
		}
		parsed, err := time.Parse(time.RFC3339, strings.TrimSpace(text))
		if err != nil {
			return nil, fmt.Errorf("%w: date %q", ErrInvalid, text)
		}
		return parsed, nil
	case "data":
		text, err := readXMLText(decoder)
		if err != nil {
			return nil, err
		}
		return []byte(strings.TrimSpace(text)), nil
	default:
		return nil, fmt.Errorf("%w: element %q", ErrInvalid, start.Name.Local)
	}
}

// readXMLArray reads an array element.
func readXMLArray(decoder *xml.Decoder) (any, error) {
	array := []any{}
	for {
		token, err := decoder.Token()
		if err != nil {
			return nil, fmt.Errorf("%w: %v", ErrInvalid, err)
		}
		switch element := token.(type) {
		case xml.StartElement:
			value, err := readXMLValue(decoder, element)
			if err != nil {
				return nil, err
			}
			array = append(array, value)
		case xml.EndElement:
			if element.Name.Local == "array" {
				return array, nil
			}
		}
	}
}

// readXMLText reads the character data of the current element, up to its end.
func readXMLText(decoder *xml.Decoder) (string, error) {
	var builder strings.Builder
	for {
		token, err := decoder.Token()
		if err != nil {
			return "", fmt.Errorf("%w: %v", ErrInvalid, err)
		}
		switch element := token.(type) {
		case xml.CharData:
			builder.Write(element)
		case xml.EndElement:
			return builder.String(), nil
		}
	}
}

// skipXMLElement consumes the current element's end tag.
func skipXMLElement(decoder *xml.Decoder) error {
	for {
		token, err := decoder.Token()
		if err != nil {
			return fmt.Errorf("%w: %v", ErrInvalid, err)
		}
		if _, ok := token.(xml.EndElement); ok {
			return nil
		}
	}
}

// ---------------------------------------------------------------- binary

// parseBinary reads the `bplist00` encoding.
func parseBinary(data []byte) (any, error) {
	if len(data) < 40 || !bytes.HasPrefix(data, []byte("bplist00")) {
		return nil, fmt.Errorf("%w: bad binary header", ErrInvalid)
	}
	trailer := data[len(data)-32:]
	offsetSize := int(trailer[6])
	refSize := int(trailer[7])
	numObjects := int(binary.BigEndian.Uint64(trailer[8:16]))
	topObject := int(binary.BigEndian.Uint64(trailer[16:24]))
	tableOffset := int(binary.BigEndian.Uint64(trailer[24:32]))
	if offsetSize < 1 || offsetSize > 8 || refSize < 1 || refSize > 8 || numObjects <= 0 {
		return nil, fmt.Errorf("%w: bad binary trailer", ErrInvalid)
	}
	if tableOffset < 8 || tableOffset+numObjects*offsetSize > len(data) {
		return nil, fmt.Errorf("%w: offset table out of range", ErrInvalid)
	}
	offsets := make([]int, numObjects)
	for i := 0; i < numObjects; i++ {
		start := tableOffset + i*offsetSize
		offsets[i] = int(readBigEndian(data[start : start+offsetSize]))
	}
	if topObject < 0 || topObject >= numObjects {
		return nil, fmt.Errorf("%w: top object out of range", ErrInvalid)
	}
	reader := &binaryReader{data: data, offsets: offsets, refSize: refSize}
	return reader.object(topObject)
}

// binaryReader reads objects out of one document.
type binaryReader struct {
	data    []byte
	offsets []int
	refSize int
}

// object reads the object at an offset table index.
func (r *binaryReader) object(index int) (any, error) {
	if index < 0 || index >= len(r.offsets) {
		return nil, fmt.Errorf("%w: object index %d", ErrInvalid, index)
	}
	return r.value(r.offsets[index])
}

// value reads the object starting at an offset.
func (r *binaryReader) value(offset int) (any, error) {
	if offset < 0 || offset >= len(r.data) {
		return nil, fmt.Errorf("%w: object offset %d", ErrInvalid, offset)
	}
	marker := r.data[offset]
	kind, size, next, err := r.length(marker, offset+1)
	if err != nil {
		return nil, err
	}
	switch kind {
	case 0x0:
		switch marker {
		case 0x08:
			return false, nil
		case 0x09:
			return true, nil
		default:
			return nil, nil // null and fill bytes
		}
	case 0x1: // integer
		raw, err := r.slice(next, 1<<size)
		if err != nil {
			return nil, err
		}
		switch len(raw) {
		case 1:
			return int64(raw[0]), nil
		case 2:
			return int64(binary.BigEndian.Uint16(raw)), nil
		case 4:
			return int64(binary.BigEndian.Uint32(raw)), nil
		case 8:
			return int64(binary.BigEndian.Uint64(raw)), nil
		default:
			return nil, fmt.Errorf("%w: integer of %d bytes", ErrInvalid, len(raw))
		}
	case 0x2: // real
		raw, err := r.slice(next, 1<<size)
		if err != nil {
			return nil, err
		}
		switch len(raw) {
		case 4:
			return float64(math.Float32frombits(binary.BigEndian.Uint32(raw))), nil
		case 8:
			return math.Float64frombits(binary.BigEndian.Uint64(raw)), nil
		default:
			return nil, fmt.Errorf("%w: real of %d bytes", ErrInvalid, len(raw))
		}
	case 0x3: // date: seconds since 2001-01-01
		raw, err := r.slice(next, 8)
		if err != nil {
			return nil, err
		}
		seconds := math.Float64frombits(binary.BigEndian.Uint64(raw))
		return time.Unix(int64(seconds)+978307200, 0).UTC(), nil
	case 0x4: // data
		raw, err := r.slice(next, size)
		if err != nil {
			return nil, err
		}
		return append([]byte(nil), raw...), nil
	case 0x5: // ASCII string
		raw, err := r.slice(next, size)
		if err != nil {
			return nil, err
		}
		return string(raw), nil
	case 0x6: // UTF-16BE string
		raw, err := r.slice(next, size*2)
		if err != nil {
			return nil, err
		}
		runes := make([]rune, 0, size)
		for i := 0; i+1 < len(raw); i += 2 {
			runes = append(runes, rune(binary.BigEndian.Uint16(raw[i:i+2])))
		}
		return string(runes), nil
	case 0x8: // UID
		raw, err := r.slice(next, size+1)
		if err != nil {
			return nil, err
		}
		return int64(readBigEndian(raw)), nil
	case 0xa, 0xc: // array, set
		array := make([]any, 0, size)
		for i := 0; i < size; i++ {
			index, err := r.reference(next + i*r.refSize)
			if err != nil {
				return nil, err
			}
			item, err := r.object(index)
			if err != nil {
				return nil, err
			}
			array = append(array, item)
		}
		return array, nil
	case 0xd: // dictionary
		dict := make(map[string]any, size)
		keysAt := next
		valuesAt := next + size*r.refSize
		for i := 0; i < size; i++ {
			keyIndex, err := r.reference(keysAt + i*r.refSize)
			if err != nil {
				return nil, err
			}
			key, err := r.object(keyIndex)
			if err != nil {
				return nil, err
			}
			name, ok := key.(string)
			if !ok {
				return nil, fmt.Errorf("%w: a dictionary key is not a string", ErrInvalid)
			}
			valueIndex, err := r.reference(valuesAt + i*r.refSize)
			if err != nil {
				return nil, err
			}
			value, err := r.object(valueIndex)
			if err != nil {
				return nil, err
			}
			dict[name] = value
		}
		return dict, nil
	default:
		return nil, fmt.Errorf("%w: object type %#x", ErrInvalid, kind)
	}
}

// length reads a marker's kind and size, returning where the payload starts.
// A size of 15 means the real size follows as an integer object.
func (r *binaryReader) length(marker byte, at int) (kind byte, size int, next int, err error) {
	kind = marker >> 4
	size = int(marker & 0x0f)
	if size != 0x0f {
		return kind, size, at, nil
	}
	if at >= len(r.data) {
		return 0, 0, 0, fmt.Errorf("%w: size out of range", ErrInvalid)
	}
	lengthMarker := r.data[at]
	if lengthMarker>>4 != 0x1 {
		return 0, 0, 0, fmt.Errorf("%w: size is not an integer", ErrInvalid)
	}
	width := 1 << (lengthMarker & 0x0f)
	raw, err := r.slice(at+1, width)
	if err != nil {
		return 0, 0, 0, err
	}
	return kind, int(readBigEndian(raw)), at + 1 + width, nil
}

// reference reads an object reference.
func (r *binaryReader) reference(at int) (int, error) {
	raw, err := r.slice(at, r.refSize)
	if err != nil {
		return 0, err
	}
	return int(readBigEndian(raw)), nil
}

// slice returns n bytes at at.
func (r *binaryReader) slice(at, n int) ([]byte, error) {
	if n < 0 || at < 0 || at+n > len(r.data) {
		return nil, fmt.Errorf("%w: read of %d bytes at %d", ErrInvalid, n, at)
	}
	return r.data[at : at+n], nil
}

// readBigEndian reads an unsigned big-endian integer of any width.
func readBigEndian(raw []byte) uint64 {
	var value uint64
	for _, b := range raw {
		value = value<<8 | uint64(b)
	}
	return value
}
