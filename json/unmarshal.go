package json

import (
	"encoding"
	"encoding/base64"
	"fmt"
	"math"
	"reflect"
	"strconv"
	"strings"
	"unicode/utf16"
	"unicode/utf8"

	"github.com/olimci/roundtrip/internal/util/reflectutil"
)

type Unmarshaler interface {
	UnmarshalJSON([]byte) error
}

var unmarshalerType = reflect.TypeFor[Unmarshaler]()
var textUnmarshalerType = reflect.TypeFor[encoding.TextUnmarshaler]()

type decodeOptions struct {
	useNumber       bool
	disallowUnknown bool
	json5           bool
}

func decodeInto(m *Meta, n *node, v any, opts decodeOptions) error {
	rv := reflect.ValueOf(v)
	if !rv.IsValid() {
		return InvalidUnmarshalError{}
	}
	if rv.Kind() != reflect.Pointer || rv.IsNil() {
		return InvalidUnmarshalError{Type: rv.Type()}
	}
	return decodeValue(m, n, rv.Elem(), opts)
}

func decodeValue(m *Meta, n *node, v reflect.Value, opts decodeOptions) error {
	if !v.CanSet() {
		return fmt.Errorf("cannot set %s", v.Type())
	}

	if u, ok := unmarshaler(v); ok {
		return u.UnmarshalJSON(Node{meta: m, node: n}.Bytes())
	}
	if n.Type == NodeTypeString {
		if u, ok := textUnmarshaler(v); ok {
			s, err := decodeString(m, n, opts)
			if err != nil {
				return err
			}
			return u.UnmarshalText([]byte(s))
		}
	}

	if n.Type == NodeTypeNull {
		if reflectutil.Nilable(v.Kind()) {
			v.SetZero()
		}
		return nil
	}
	if v.Type() == numberType {
		if n.Type != NodeTypeNumber {
			return typeError(n, v.Type())
		}
		v.SetString(n.Start.Value.Literal)
		return nil
	}

	switch v.Kind() {
	case reflect.Pointer:
		if v.IsNil() {
			v.Set(reflect.New(v.Type().Elem()))
		}
		return decodeValue(m, n, v.Elem(), opts)
	case reflect.Interface:
		value, err := decodeAny(m, n, opts)
		if err != nil {
			return err
		}
		v.Set(reflect.ValueOf(value))
		return nil
	case reflect.Bool:
		value, err := strconv.ParseBool(n.Start.Value.Literal)
		if err != nil {
			return err
		}
		v.SetBool(value)
		return nil
	case reflect.String:
		if n.Type != NodeTypeString {
			return typeError(n, v.Type())
		}
		value, err := decodeString(m, n, opts)
		if err != nil {
			return err
		}
		v.SetString(value)
		return nil
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		if n.Type != NodeTypeNumber {
			return typeError(n, v.Type())
		}
		value, err := parseIntLiteral(n.Start.Value.Literal, v.Type().Bits(), opts)
		if err != nil {
			return err
		}
		v.SetInt(value)
		return nil
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		if n.Type != NodeTypeNumber {
			return typeError(n, v.Type())
		}
		value, err := parseUintLiteral(n.Start.Value.Literal, v.Type().Bits(), opts)
		if err != nil {
			return err
		}
		v.SetUint(value)
		return nil
	case reflect.Float32, reflect.Float64:
		if n.Type != NodeTypeNumber {
			return typeError(n, v.Type())
		}
		value, err := parseFloatLiteral(n.Start.Value.Literal, v.Type().Bits(), opts)
		if err != nil {
			return err
		}
		v.SetFloat(value)
		return nil
	case reflect.Slice:
		return decodeSlice(m, n, v, opts)
	case reflect.Array:
		return decodeArray(m, n, v, opts)
	case reflect.Map:
		return decodeMap(m, n, v, opts)
	case reflect.Struct:
		return decodeStruct(m, n, v, opts)
	default:
		return fmt.Errorf("cannot decode %v into %v", n.Type, v.Type())
	}
}

func decodeAny(m *Meta, n *node, opts decodeOptions) (any, error) {
	switch n.Type {
	case NodeTypeObject:
		value := make(map[string]any, len(n.Children)/2)
		for keyNode, valueNode := range objectFields(n) {
			key, err := decodeString(m, keyNode, opts)
			if err != nil {
				return nil, err
			}
			child, err := decodeAny(m, valueNode, opts)
			if err != nil {
				return nil, err
			}
			value[key] = child
		}
		return value, nil
	case NodeTypeArray:
		value := make([]any, len(n.Children))
		for i, child := range n.Children {
			item, err := decodeAny(m, child, opts)
			if err != nil {
				return nil, err
			}
			value[i] = item
		}
		return value, nil
	case NodeTypeString:
		return decodeString(m, n, opts)
	case NodeTypeNumber:
		if opts.useNumber {
			return Number(n.Start.Value.Literal), nil
		}
		return parseFloatLiteral(n.Start.Value.Literal, 64, opts)
	case NodeTypeBool:
		return strconv.ParseBool(n.Start.Value.Literal)
	case NodeTypeNull:
		return nil, nil
	default:
		return nil, fmt.Errorf("cannot decode %v", n.Type)
	}
}

func decodeSlice(m *Meta, n *node, v reflect.Value, opts decodeOptions) error {
	if v.Type().Elem().Kind() == reflect.Uint8 && n.Type == NodeTypeString {
		s, err := decodeString(m, n, opts)
		if err != nil {
			return err
		}
		b, err := base64.StdEncoding.DecodeString(s)
		if err != nil {
			return err
		}
		v.SetBytes(b)
		return nil
	}
	if n.Type != NodeTypeArray {
		return fmt.Errorf("cannot decode %v into %v", n.Type, v.Type())
	}

	value := reflect.MakeSlice(v.Type(), len(n.Children), len(n.Children))
	for i, child := range n.Children {
		if err := decodeValue(m, child, value.Index(i), opts); err != nil {
			return err
		}
	}
	v.Set(value)
	return nil
}

func decodeArray(m *Meta, n *node, v reflect.Value, opts decodeOptions) error {
	if n.Type != NodeTypeArray {
		return fmt.Errorf("cannot decode %v into %v", n.Type, v.Type())
	}

	for i := range v.Len() {
		v.Index(i).SetZero()
	}
	for i, child := range n.Children[:min(len(n.Children), v.Len())] {
		if err := decodeValue(m, child, v.Index(i), opts); err != nil {
			return err
		}
	}
	return nil
}

func decodeMap(m *Meta, n *node, v reflect.Value, opts decodeOptions) error {
	if n.Type != NodeTypeObject {
		return fmt.Errorf("cannot decode %v into %v", n.Type, v.Type())
	}
	if !mapKeyTypeSupported(v.Type().Key()) {
		return fmt.Errorf("cannot decode object into %v", v.Type())
	}
	if v.IsNil() {
		v.Set(reflect.MakeMapWithSize(v.Type(), len(n.Children)/2))
	}

	for keyNode, valueNode := range objectFields(n) {
		key, err := decodeString(m, keyNode, opts)
		if err != nil {
			return err
		}

		mapKey, err := decodeMapKey(key, v.Type().Key(), opts)
		if err != nil {
			return err
		}

		value := reflect.New(v.Type().Elem()).Elem()
		if err := decodeValue(m, valueNode, value, opts); err != nil {
			return err
		}
		v.SetMapIndex(mapKey, value)
	}
	return nil
}

func decodeStruct(m *Meta, n *node, v reflect.Value, opts decodeOptions) error {
	if n.Type != NodeTypeObject {
		return fmt.Errorf("cannot decode %v into %v", n.Type, v.Type())
	}

	fields := structFieldIndexes(v.Type())
	for keyNode, valueNode := range objectFields(n) {
		key, err := decodeString(m, keyNode, opts)
		if err != nil {
			return err
		}

		field, ok := fieldIndex(fields, key)
		if !ok {
			if opts.disallowUnknown {
				return fmt.Errorf("json: unknown field %q", key)
			}
			continue
		}
		valueMeta := m
		target, ok := fieldByIndex(v, field.Index, true)
		if !ok {
			return fmt.Errorf("json: cannot set embedded pointer for field %q", field.Name)
		}
		if field.Options.Quoted {
			decoded, err := quotedNode(m, valueNode, opts)
			if err != nil {
				return err
			}
			valueNode = decoded.SST.Root
			valueMeta = decoded
		}
		if err := decodeValue(valueMeta, valueNode, target, opts); err != nil {
			return err
		}
	}
	return nil
}

func unmarshaler(v reflect.Value) (Unmarshaler, bool) {
	if v.Kind() == reflect.Pointer && v.IsNil() {
		v.Set(reflect.New(v.Type().Elem()))
	}
	if v.CanInterface() && v.Type().Implements(unmarshalerType) {
		return v.Interface().(Unmarshaler), true
	}
	if v.CanAddr() && reflect.PointerTo(v.Type()).Implements(unmarshalerType) {
		return v.Addr().Interface().(Unmarshaler), true
	}
	return nil, false
}

func textUnmarshaler(v reflect.Value) (encoding.TextUnmarshaler, bool) {
	if v.Kind() == reflect.Pointer && v.IsNil() {
		v.Set(reflect.New(v.Type().Elem()))
	}
	if v.CanInterface() && v.Type().Implements(textUnmarshalerType) {
		return v.Interface().(encoding.TextUnmarshaler), true
	}
	if v.CanAddr() && reflect.PointerTo(v.Type()).Implements(textUnmarshalerType) {
		return v.Addr().Interface().(encoding.TextUnmarshaler), true
	}
	return nil, false
}

func quotedNode(m *Meta, n *node, opts decodeOptions) (*Meta, error) {
	s, err := decodeString(m, n, opts)
	if err != nil {
		return nil, err
	}
	d := NewDecoder(strings.NewReader(s))
	if opts.json5 {
		d = NewJSON5Decoder(strings.NewReader(s))
	}
	return d.DecodeMeta()
}

func decodeMapKey(s string, typ reflect.Type, opts decodeOptions) (reflect.Value, error) {
	v := reflect.New(typ).Elem()
	if u, ok := textUnmarshaler(v); ok {
		if err := u.UnmarshalText([]byte(s)); err != nil {
			return reflect.Value{}, err
		}
		return v, nil
	}
	switch typ.Kind() {
	case reflect.String:
		v.SetString(s)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		i, err := parseIntLiteral(s, typ.Bits(), opts)
		if err != nil {
			return reflect.Value{}, err
		}
		v.SetInt(i)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		u, err := parseUintLiteral(s, typ.Bits(), opts)
		if err != nil {
			return reflect.Value{}, err
		}
		v.SetUint(u)
	default:
		return reflect.Value{}, fmt.Errorf("cannot decode object key into %v", typ)
	}
	return v, nil
}

func typeError(n *node, typ reflect.Type) error {
	return &UnmarshalTypeError{Value: strings.ToLower(n.Type.String()), Type: typ, Offset: int64(n.Start.Value.Position.Offset)}
}

func decodeString(m *Meta, n *node, opts decodeOptions) (string, error) {
	if n.Type != NodeTypeString {
		return "", fmt.Errorf("cannot decode %v into string", n.Type)
	}
	if n.Start.Value.Type == TokenIdentifier {
		return n.Start.Value.Literal, nil
	}
	return unquoteString(n.Start.Value.Literal, opts.json5)
}

func unquoteJSONString(s string) (string, error) {
	return unquoteStringOptions(s, stringOptions{})
}

func unquoteString(s string, json5 bool) (string, error) {
	return unquoteStringOptions(s, stringOptions{
		allowSingleQuote:      json5,
		allowJSON5Escapes:     json5,
		allowLineContinuation: json5,
	})
}

type stringOptions struct {
	allowSingleQuote      bool
	allowJSON5Escapes     bool
	allowLineContinuation bool
}

func unquoteStringOptions(s string, opts stringOptions) (string, error) {
	if len(s) < 2 || s[0] != '"' || s[len(s)-1] != '"' {
		if !opts.allowSingleQuote || len(s) < 2 || s[0] != '\'' || s[len(s)-1] != '\'' {
			return "", fmt.Errorf("invalid JSON string %q", s)
		}
	}

	var b strings.Builder
	b.Grow(len(s) - 2)
	for i := 1; i < len(s)-1; {
		c := s[i]
		if c == '\\' {
			i++
			if i >= len(s)-1 {
				return "", fmt.Errorf("invalid JSON string escape %q", s)
			}

			if opts.allowLineContinuation {
				if next, ok := json5LineContinuation(s, i); ok {
					i = next
					continue
				}
			}

			switch c = s[i]; c {
			case '"', '\\', '/':
				b.WriteByte(c)
				i++
			case '\'':
				if !opts.allowJSON5Escapes {
					return "", fmt.Errorf("invalid JSON string escape %q", s)
				}
				b.WriteByte(c)
				i++
			case 'b':
				b.WriteByte('\b')
				i++
			case 'f':
				b.WriteByte('\f')
				i++
			case 'v':
				if !opts.allowJSON5Escapes {
					return "", fmt.Errorf("invalid JSON string escape %q", s)
				}
				b.WriteByte('\v')
				i++
			case '0':
				if !opts.allowJSON5Escapes {
					return "", fmt.Errorf("invalid JSON string escape %q", s)
				}
				b.WriteByte(0)
				i++
			case 'x':
				if !opts.allowJSON5Escapes || i+2 >= len(s)-1 {
					return "", fmt.Errorf("invalid JSON string escape %q", s)
				}
				r, err := unquoteJSONHexEscape(s, i+1)
				if err != nil {
					return "", err
				}
				b.WriteRune(r)
				i += 3
			case 'n':
				b.WriteByte('\n')
				i++
			case 'r':
				b.WriteByte('\r')
				i++
			case 't':
				b.WriteByte('\t')
				i++
			case 'u':
				r, err := unquoteJSONUnicodeEscape(s, i+1)
				if err != nil {
					return "", err
				}
				i += 5

				if 0xd800 <= r && r <= 0xdbff {
					if i+5 < len(s)-1 && s[i] == '\\' && s[i+1] == 'u' {
						r2, err := unquoteJSONUnicodeEscape(s, i+2)
						if err != nil {
							return "", err
						}
						if decoded := utf16.DecodeRune(r, r2); decoded != utf8.RuneError {
							r = decoded
							i += 6
						} else {
							r = utf8.RuneError
						}
					} else {
						r = utf8.RuneError
					}
				} else if 0xdc00 <= r && r <= 0xdfff {
					r = utf8.RuneError
				}
				b.WriteRune(r)
			default:
				return "", fmt.Errorf("invalid JSON string escape %q", s)
			}
			continue
		}

		if c < 0x20 {
			return "", fmt.Errorf("invalid character in JSON string %q", s)
		}

		r, size := utf8.DecodeRuneInString(s[i : len(s)-1])
		b.WriteRune(r)
		i += size
	}

	return b.String(), nil
}

func unquoteJSONUnicodeEscape(s string, i int) (rune, error) {
	if i+4 > len(s) {
		return 0, fmt.Errorf("invalid JSON unicode escape %q", s)
	}

	var r rune
	for _, c := range s[i : i+4] {
		r <<= 4
		switch {
		case '0' <= c && c <= '9':
			r += c - '0'
		case 'a' <= c && c <= 'f':
			r += c - 'a' + 10
		case 'A' <= c && c <= 'F':
			r += c - 'A' + 10
		default:
			return 0, fmt.Errorf("invalid JSON unicode escape %q", s)
		}
	}
	return r, nil
}

func unquoteJSONHexEscape(s string, i int) (rune, error) {
	if i+2 > len(s) {
		return 0, fmt.Errorf("invalid JSON hex escape %q", s)
	}
	var r rune
	for _, c := range s[i : i+2] {
		r <<= 4
		switch {
		case '0' <= c && c <= '9':
			r += c - '0'
		case 'a' <= c && c <= 'f':
			r += c - 'a' + 10
		case 'A' <= c && c <= 'F':
			r += c - 'A' + 10
		default:
			return 0, fmt.Errorf("invalid JSON hex escape %q", s)
		}
	}
	return r, nil
}

func json5LineContinuation(s string, i int) (int, bool) {
	if i >= len(s)-1 {
		return i, false
	}
	switch s[i] {
	case '\n':
		return i + 1, true
	case '\r':
		if i+1 < len(s)-1 && s[i+1] == '\n' {
			return i + 2, true
		}
		return i + 1, true
	}
	r, size := utf8.DecodeRuneInString(s[i : len(s)-1])
	switch r {
	case '\u2028', '\u2029':
		return i + size, true
	default:
		return i, false
	}
}

func validJSON5Number(s string) bool {
	if s == "NaN" || s == "+NaN" || s == "-NaN" ||
		s == "Infinity" || s == "+Infinity" || s == "-Infinity" {
		return true
	}

	start := 0
	if strings.HasPrefix(s, "+") || strings.HasPrefix(s, "-") {
		start = 1
		if start == len(s) {
			return false
		}
	}
	if len(s[start:]) > 2 && s[start] == '0' && (s[start+1] == 'x' || s[start+1] == 'X') {
		if start+2 == len(s) {
			return false
		}
		for _, r := range s[start+2:] {
			if !isHex(r) {
				return false
			}
		}
		return true
	}
	if validNumber(strings.TrimPrefix(s, "+")) {
		return true
	}
	return validJSON5Decimal(s)
}

func validJSON5Decimal(s string) bool {
	i := 0
	if i < len(s) && (s[i] == '+' || s[i] == '-') {
		i++
	}
	if i == len(s) {
		return false
	}

	digitsBefore := 0
	for i < len(s) && '0' <= s[i] && s[i] <= '9' {
		digitsBefore++
		i++
	}

	hasDot := i < len(s) && s[i] == '.'
	if hasDot {
		i++
	}

	digitsAfter := 0
	for i < len(s) && '0' <= s[i] && s[i] <= '9' {
		digitsAfter++
		i++
	}

	if digitsBefore == 0 && digitsAfter == 0 {
		return false
	}
	if digitsBefore > 1 {
		signOffset := 0
		if s[0] == '+' || s[0] == '-' {
			signOffset = 1
		}
		if s[signOffset] == '0' {
			return false
		}
	}

	if i < len(s) && (s[i] == 'e' || s[i] == 'E') {
		i++
		if i < len(s) && (s[i] == '+' || s[i] == '-') {
			i++
		}
		expStart := i
		for i < len(s) && '0' <= s[i] && s[i] <= '9' {
			i++
		}
		if i == expStart {
			return false
		}
	}
	return i == len(s)
}

func parseIntLiteral(s string, bits int, opts decodeOptions) (int64, error) {
	if opts.json5 {
		return strconv.ParseInt(strings.TrimPrefix(s, "+"), 0, bits)
	}
	return strconv.ParseInt(s, 10, bits)
}

func parseUintLiteral(s string, bits int, opts decodeOptions) (uint64, error) {
	if opts.json5 {
		return strconv.ParseUint(strings.TrimPrefix(s, "+"), 0, bits)
	}
	return strconv.ParseUint(s, 10, bits)
}

func parseFloatLiteral(s string, bits int, opts decodeOptions) (float64, error) {
	if !opts.json5 {
		return strconv.ParseFloat(s, bits)
	}
	switch s {
	case "NaN", "+NaN", "-NaN":
		return math.NaN(), nil
	case "Infinity", "+Infinity":
		return math.Inf(1), nil
	case "-Infinity":
		return math.Inf(-1), nil
	}
	start := 0
	if strings.HasPrefix(s, "+") || strings.HasPrefix(s, "-") {
		start = 1
	}
	if len(s[start:]) > 2 && s[start] == '0' && (s[start+1] == 'x' || s[start+1] == 'X') {
		u, err := strconv.ParseUint(s[start+2:], 16, 64)
		if err != nil {
			return 0, err
		}
		f := float64(u)
		if strings.HasPrefix(s, "-") {
			f = -f
		}
		return f, nil
	}
	return strconv.ParseFloat(s, bits)
}
