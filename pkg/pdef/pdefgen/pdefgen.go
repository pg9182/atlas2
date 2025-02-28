// Command pdefgen generates a Go package to process r2 pdata from a pdef.
package main

import (
	"bytes"
	"fmt"
	"go/format"
	"os"
	"path/filepath"
	"runtime/debug"
	"sort"
	"strconv"
	"strings"
	"unicode"

	"github.com/r2northstar/atlas/v2/pkg/pdef"
)

var self string

func init() {
	if bi, ok := debug.ReadBuildInfo(); !ok || !strings.Contains(bi.Path, "/") {
		panic("failed to get own package name")
	} else {
		self = bi.Path
	}
}

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintf(os.Stderr, "usage: go run %s pdef_version_number\n", self)
		os.Exit(2)
	}

	pdefVersion, err := strconv.Atoi(os.Args[1])
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: invalid pdef version: %v\n", err)
		os.Exit(1)
	}

	f, err := openSingleFile("*.pdef")
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: find pdef: %v\n", err)
		os.Exit(1)
	}

	pd, err := pdef.ParsePdef(f)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: parse pdef: %v\n", err)
		os.Exit(1)
	}

	var buf bytes.Buffer

	pln(&buf, `// Code generated by pdefgen; DO NOT EDIT.`)
	pln(&buf, ``)

	pln(&buf, `// Package pdata parses Titanfall 2 player data using the schema defined in %s.`, f.Name())
	pln(&buf, `//`)
	pln(&buf, `// Roundtrip marshal/unmarshal should be byte-identical except for extra data after`)
	pln(&buf, `// the null terminator in strings or non-0/1 boolean values. Invalid enum values`)
	pln(&buf, `// and trailing data after the pdata root struct are preserved.`)
	pln(&buf, `package pdata`)
	pln(&buf, ``)

	pln(&buf, `//go:generate go run %s %#v`, self, pdefVersion)
	pln(&buf, ``)

	pln(&buf, `import (`)
	pln(&buf, `"bytes"`)
	pln(&buf, `_ "embed"`)
	pln(&buf, `"encoding"`)
	pln(&buf, `"encoding/binary"`)
	pln(&buf, `"encoding/json"`)
	pln(&buf, `"errors"`)
	pln(&buf, `"fmt"`)
	pln(&buf, `"io"`)
	pln(&buf, `"math"`)
	pln(&buf, `"reflect"`)
	pln(&buf, `"strconv"`)
	pln(&buf, `"strings"`)
	pln(&buf, `)`)
	pln(&buf, ``)

	pln(&buf, `const Version int32 = %#v`, pdefVersion)
	pln(&buf, ``)

	pln(&buf, `//go:embed %+q`, f.Name())
	pln(&buf, `var rawPdef []byte`)
	pln(&buf, `func RawPdef() io.Reader { return bytes.NewReader(rawPdef) }`)
	pln(&buf, ``)

	pln(&buf, `var (`)
	pln(&buf, `ErrUnsupportedVersion = errors.New(%#v)`, `unsupported pdata version`)
	pln(&buf, `ErrInvalidSize = errors.New(%#v)`, `invalid size`)
	pln(&buf, `ErrInvalidEnumValue = errors.New(%#v) // only used when encoding/decoding text/json (binary encode/decode will still preserve unknown values)`, `invalid enum value`)
	pln(&buf, `)`)

	pln(&buf, `func getString(b []byte) string {`)
	pln(&buf, `for i, x := range b {`)
	pln(&buf, `if x == '\x00' {`)
	pln(&buf, `return string(b[:i])`)
	pln(&buf, `}`)
	pln(&buf, `}`)
	pln(&buf, `return string(b)`)
	pln(&buf, `}`)

	pln(&buf, `func getInt(b []byte) int32 {`)
	pln(&buf, `return int32(binary.LittleEndian.Uint32(b))`)
	pln(&buf, `}`)

	pln(&buf, `func getFloat(b []byte) float32 {`)
	pln(&buf, `return math.Float32frombits(binary.LittleEndian.Uint32(b))`)
	pln(&buf, `}`)

	pln(&buf, `func getBool(b []byte) bool {`)
	pln(&buf, `return b[0] != 0`)
	pln(&buf, `}`)

	pln(&buf, `func getEnum(b []byte) uint8 {`)
	pln(&buf, `return b[0]`)
	pln(&buf, `}`)

	pln(&buf, `func must(err error) {`)
	pln(&buf, `if err != nil {`)
	pln(&buf, `panic(err)`)
	pln(&buf, `}`)
	pln(&buf, `}`)

	pln(&buf, `func putString(b []byte, x string) error {`)
	pln(&buf, `s := []byte(x)`)
	pln(&buf, `if len(s) > len(b) {`)
	pln(&buf, `return fmt.Errorf(%#v, len(s), len(b))`, `string length %d too long for field length %d`)
	pln(&buf, `}`)
	pln(&buf, `for i, c := range s {`)
	pln(&buf, `b[i] = c`)
	pln(&buf, `}`)
	pln(&buf, `for i := len(s); i < len(b); i++ {`)
	pln(&buf, `b[i] = '\x00'`)
	pln(&buf, `}`)
	pln(&buf, `return nil`)
	pln(&buf, `}`)

	pln(&buf, `func putInt(b []byte, x int32) {`)
	pln(&buf, `binary.LittleEndian.PutUint32(b, uint32(x))`)
	pln(&buf, `}`)

	pln(&buf, `func putFloat(b []byte, x float32) {`)
	pln(&buf, `binary.LittleEndian.PutUint32(b, math.Float32bits(x))`)
	pln(&buf, `}`)

	pln(&buf, `func putBool(b []byte, x bool) {`)
	pln(&buf, `if x {`)
	pln(&buf, `b[0] = 1`)
	pln(&buf, `} else {`)
	pln(&buf, `b[0] = 0`)
	pln(&buf, `}`)
	pln(&buf, `}`)

	pln(&buf, `func putEnum(b []byte, x uint8) {`)
	pln(&buf, `b[0] = x`)
	pln(&buf, `}`)

	generateStruct(&buf, "pdata", pd, pd.Root, true)

	each(pd.Struct, func(k string, v []pdef.Field) {
		generateStruct(&buf, k, pd, v, false)
	})

	each(pd.Enum, func(k string, v []string) {
		generateEnum(&buf, k, v)
	})

	// this is needed since the default Go marshaler can't filter and doesn't
	// handle NaN/Inf floats.
	pln(&buf, "%s", `
		func pdataMarshalJSONStruct(obj any, filter func(path ...string) bool, path ...string) ([]byte, error) {
			objVal := reflect.ValueOf(obj)
			objTyp := objVal.Type()

			if objTyp.Kind() != reflect.Struct {
				panic("not a struct")
			}

			var b bytes.Buffer
			b.WriteByte('{')
			needComma := false
			for i := 0; i < objTyp.NumField(); i++ {
				fldTyp := objTyp.Field(i)
				fldVal := objVal.Field(i)
				fld := fldVal.Interface()

				fldTag := fldTyp.Tag.Get("pdef")
				fldTagName, fldTagAttr, _ := strings.Cut(fldTag, ",")
				if fldTagName == "" {
					continue
				}
				if fldTagAttr != "" {
					panic(fmt.Errorf("unknown pdef field tag attrs %q", fldTagAttr))
				}

				fldPath := append(path, fldTagName)
				if filter != nil && !filter(fldPath...) {
					continue
				}

				if needComma {
					b.WriteByte(',')
					needComma = false
				}

				b.WriteString("\"" + fldTagName + "\":")
				needComma = true

				switch fldTyp.Type.Kind() {
				case reflect.Struct:
					buf, err := pdataMarshalJSONStruct(fld, filter, fldPath...)
					if err != nil {
						return nil, err
					}
					b.Write(buf)
				case reflect.Array:
					b.WriteByte('[')
					for j := 0; j < fldTyp.Type.Len(); j++ {
						fldValElemVal := fldVal.Index(j)
						fldValElemTyp := fldValElemVal.Type()
						fldValElem := fldValElemVal.Interface()

						if j != 0 {
							b.WriteByte(',')
						}
						if fldValElemTyp.Kind() == reflect.Struct {
							buf, err := pdataMarshalJSONStruct(fldValElem, filter, fldPath...)
							if err != nil {
								return nil, err
							}
							b.Write(buf)
						} else {
							buf, err := pdataMarshalJSONPrimitive(fldValElem)
							if err != nil {
								return nil, err
							}
							b.Write(buf)
						}
					}
					b.WriteByte(']')
				default:
					buf, err := pdataMarshalJSONPrimitive(fld)
					if err != nil {
						return nil, err
					}
					b.Write(buf)
				}
			}
			b.WriteByte('}')
			return b.Bytes(), nil
		}

		func pdataMarshalJSONPrimitive(v any) ([]byte, error) {
			switch v := v.(type) {
			case float32:
				if math.IsNaN(float64(v)) || math.IsInf(float64(v), 0) {
					return []byte("null"), nil // match the JS behaviour
				}
				return json.Marshal(v)
			case int32, bool, string:
				return json.Marshal(v)
			case json.Marshaler:
				if reflect.TypeOf(v).ConvertibleTo(reflect.TypeOf(uint8(0))) {
					return v.MarshalJSON() // enum
				}
				panic(fmt.Errorf("unhandled type %T", v))
			default:
				panic(fmt.Errorf("unhandled type %T", v))
			}
		}
	`)

	if err := writeGo(strings.TrimSuffix(filepath.Base(f.Name()), filepath.Ext(f.Name()))+".go", buf.Bytes()); err != nil {
		fmt.Fprintf(os.Stderr, "error: write generated source: %v\n", err)
		os.Exit(1)
	}

	tfs, err := filepath.Glob(`*.pdata`)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: find test pdata: %v\n", err)
		os.Exit(1)
	}
	if len(tfs) != 0 {
		var tbuf bytes.Buffer
		fmt.Fprintf(&tbuf, `
			package pdata

			import (
				"bytes"
				"encoding/json"
				"os"
				"testing"
			)

			func TestPdataRoundtrip(t *testing.T) {
				for _, fn := range %#v {
					fn := fn
					t.Run(fn, func(t *testing.T) {
						obuf, err := os.ReadFile(fn)
						if err != nil {
							panic(err)
						}

						var d1 Pdata
						if err := d1.UnmarshalBinary(obuf); err != nil {
							t.Fatalf("failed to unmarshal %%q: %%v", fn, err)
						}
						rbuf, err := d1.MarshalBinary()
						if err != nil {
							t.Fatalf("failed to marshal %%q: %%v", fn, err)
						}
						if !bytes.Equal(obuf, rbuf) {
							t.Errorf("round-trip failed: re-marshaled data does not match")
						}

						var d2 Pdata
						if err := d2.UnmarshalBinary(rbuf); err != nil {
							t.Fatalf("failed to unmarshal marshaled %%q: %%v", fn, err)
						}
						ebuf, err := d2.MarshalBinary()
						if err != nil {
							t.Fatalf("failed to marshal unmarshaled marshaled %%q: %%v", fn, err)
						}
						if !bytes.Equal(rbuf, ebuf) {
							t.Errorf("internal round-trip failed: re-marshaled unmarshaled data encoded by marshal does not match")
						}

						if buf, err := d2.MarshalJSON(); err != nil {
							t.Errorf("failed to marshal as JSON: %%v", err)
						} else if err = json.Unmarshal(buf, new(map[string]interface{})); err != nil {
							t.Errorf("bad json marshal result: %%v", err)
						}
					})
				}
			}
		`, tfs)
		if err := writeGo(strings.TrimSuffix(filepath.Base(f.Name()), filepath.Ext(f.Name()))+"_test.go", tbuf.Bytes()); err != nil {
			fmt.Fprintf(os.Stderr, "error: write generated source: %v\n", err)
			os.Exit(1)
		}
	}
}

func generateStruct(buf *bytes.Buffer, name string, pd *pdef.Pdef, fields []pdef.Field, root bool) {
	if root && (len(fields) == 0 || fields[0].Type.Int == nil) {
		panic("pdef root must have a version field of type int")
	}
	var size int
	for _, v := range fields {
		size += pd.TypeSize(v.Type)
	}
	{
		pln(buf, `type %s struct {`, mangle(name, true))
		for _, v := range fields {
			pln(buf, `%s %s`+" `pdef:%q`", mangle(v.Name, true), pdefGoType(v.Type), v.Name)
		}
		if root {
			pln(buf, `%s %s`, mangle("extraData", true), "[]byte")
		}
		pln(buf, `}`)

		pln(buf, `var _ encoding.BinaryUnmarshaler = (*%s)(nil)`, mangle(name, true))
		pln(buf, `var _ encoding.BinaryMarshaler = %s{}`, mangle(name, true))
		pln(buf, `var _ json.Unmarshaler = (*%s)(nil)`, mangle(name, true))
		pln(buf, `var _ json.Marshaler = %s{}`, mangle(name, true))
	}
	{
		pln(buf, `func (v *%s) UnmarshalBinary(b []byte) error {`, mangle(name, true))
		if root {
			vs := pd.TypeSize(fields[0].Type)
			pln(buf, `if len(b) < %d {`, vs)
			pln(buf, `return fmt.Errorf(%#v, %#v, Version, ErrInvalidSize)`, `decode %q (v%d): %w: expected pdef version`, name)
			pln(buf, `}`)
			pln(buf, `if x := getInt(b[0:%d]); x != Version {`, vs)
			pln(buf, `return fmt.Errorf(%#v, %#v, Version, ErrUnsupportedVersion, x)`, `decode %q (v%d): %w: got %d`, name)
			pln(buf, `}`)
			pln(buf, `if len(b) < %d {`, size)
		} else {
			pln(buf, `if len(b) != %d {`, size)
		}
		pln(buf, `return fmt.Errorf(%#v, %#v, Version, ErrInvalidSize, %#v, len(b))`, `decode %q (v%d): %w: expected %d bytes, got %d`, name, size)
		pln(buf, `}`)
		var offset int
		for _, v := range fields {
			sz := pd.TypeSize(v.Type)
			switch {
			case v.Type.Int != nil:
				pln(buf, `v.%s = getInt(b[%d:%d])`, mangle(v.Name, true), offset, offset+sz)
			case v.Type.Bool != nil:
				pln(buf, `v.%s = getBool(b[%d:%d])`, mangle(v.Name, true), offset, offset+sz)
			case v.Type.Float != nil:
				pln(buf, `v.%s = getFloat(b[%d:%d])`, mangle(v.Name, true), offset, offset+sz)
			case v.Type.String != nil:
				pln(buf, `v.%s = getString(b[%d:%d])`, mangle(v.Name, true), offset, offset+sz)
			case v.Type.Array != nil:
				fallthrough
			case v.Type.MappedArray != nil:
				var arrCount int
				var arrType pdef.TypeInfo
				switch {
				case v.Type.Array != nil:
					arrCount = v.Type.Array.Length
					arrType = v.Type.Array.Type
				case v.Type.MappedArray != nil:
					if x, ok := pd.Enum[v.Type.MappedArray.Enum]; ok {
						arrCount = len(x)
						arrType = v.Type.MappedArray.Type
					} else {
						panic("undefined enum")
					}
				default:
					panic("bug: missing case")
				}
				szi := pd.TypeSize(arrType)
				for i := 0; i < arrCount; i++ {
					switch {
					case arrType.Int != nil:
						pln(buf, `v.%s[%d] = getInt(b[%d:%d])`, mangle(v.Name, true), i, offset+szi*i, offset+szi*i+szi)
					case arrType.Bool != nil:
						pln(buf, `v.%s[%d] = getBool(b[%d:%d])`, mangle(v.Name, true), i, offset+szi*i, offset+szi*i+szi)
					case arrType.Float != nil:
						pln(buf, `v.%s[%d] = getFloat(b[%d:%d])`, mangle(v.Name, true), i, offset+szi*i, offset+szi*i+szi)
					case arrType.String != nil:
						pln(buf, `v.%s[%d] = getString(b[%d:%d])`, mangle(v.Name, true), i, offset+szi*i, offset+szi*i+szi)
					case arrType.Array != nil:
						panic("impossible: can't have an array inside an array")
					case arrType.MappedArray != nil:
						panic("impossible: can't have a mapped array inside an array")
					case arrType.Enum != nil:
						pln(buf, `v.%s[%d] = %s(getEnum(b[%d:%d]))`, mangle(v.Name, true), i, mangle(arrType.Enum.Name, true), offset+szi*i, offset+szi*i+szi)
					case arrType.Struct != nil:
						// shouldn't error since we already checked the size
						pln(buf, `must(v.%s[%d].UnmarshalBinary(b[%d:%d]))`, mangle(v.Name, true), i, offset+szi*i, offset+szi*i+szi)
					default:
						panic("bug: unimplemented type in array")
					}
				}
			case v.Type.Enum != nil:
				pln(buf, `v.%s = %s(getEnum(b[%d:%d]))`, mangle(v.Name, true), mangle(v.Type.Enum.Name, true), offset, offset+sz)
			case v.Type.Struct != nil:
				// shouldn't error since we already checked the size
				pln(buf, `must(v.%s.UnmarshalBinary(b[%d:%d]))`, mangle(v.Name, true), offset, offset+sz)
			default:
				panic("bug: unimplemented pdef type")
			}
			offset += sz
		}
		if root {
			pln(buf, `v.%s = b[%d:]`, mangle("extraData", true), size)
		}
		pln(buf, `return nil`)
		pln(buf, `}`)
	}
	{
		pln(buf, `func (v %s) MarshalBinary() ([]byte, error) {`, mangle(name, true))
		if root {
			pln(buf, `if x := v.%s; x != Version {`, mangle(fields[0].Name, true))
			pln(buf, `return nil, fmt.Errorf(%#v, %#v, Version, ErrUnsupportedVersion, x)`, `encode %q (v%d): %w: got %d`, name)
			pln(buf, `}`)
		}
		pln(buf, `b := make([]byte, %d)`, size)
		var offset int
		for _, v := range fields {
			sz := pd.TypeSize(v.Type)
			switch {
			case v.Type.Int != nil:
				pln(buf, `putInt(b[%d:%d], v.%s)`, offset, offset+sz, mangle(v.Name, true))
			case v.Type.Bool != nil:
				pln(buf, `putBool(b[%d:%d], v.%s)`, offset, offset+sz, mangle(v.Name, true))
			case v.Type.Float != nil:
				pln(buf, `putFloat(b[%d:%d], v.%s)`, offset, offset+sz, mangle(v.Name, true))
			case v.Type.String != nil:
				pln(buf, `if err := putString(b[%d:%d], v.%s); err != nil {`, offset, offset+sz, mangle(v.Name, true))
				pln(buf, `return nil, fmt.Errorf(%#v, %#v, Version, ErrInvalidSize, %#v, err)`, `encode %q (v%d): %w: field %q: %v`, name, v.Name)
				pln(buf, `}`)
			case v.Type.Array != nil:
				fallthrough
			case v.Type.MappedArray != nil:
				var arrCount int
				var arrType pdef.TypeInfo
				switch {
				case v.Type.Array != nil:
					arrCount = v.Type.Array.Length
					arrType = v.Type.Array.Type
				case v.Type.MappedArray != nil:
					if x, ok := pd.Enum[v.Type.MappedArray.Enum]; ok {
						arrCount = len(x)
						arrType = v.Type.MappedArray.Type
					} else {
						panic("undefined enum")
					}
				default:
					panic("bug: missing case")
				}
				szi := pd.TypeSize(arrType)
				for i := 0; i < arrCount; i++ {
					switch {
					case arrType.Int != nil:
						pln(buf, `putInt(b[%d:%d], v.%s[%d])`, offset+szi*i, offset+szi*i+szi, mangle(v.Name, true), i)
					case arrType.Bool != nil:
						pln(buf, `putBool(b[%d:%d], v.%s[%d])`, offset+szi*i, offset+szi*i+szi, mangle(v.Name, true), i)
					case arrType.Float != nil:
						pln(buf, `putFloat(b[%d:%d], v.%s[%d])`, offset+szi*i, offset+szi*i+szi, mangle(v.Name, true), i)
					case arrType.String != nil:
						pln(buf, `if err := putString(b[%d:%d], v.%s[%d]); err != nil {`, offset+szi*i, offset+szi*i+szi, mangle(v.Name, true), i)
						pln(buf, `return nil, fmt.Errorf(%#v, %#v, Version, ErrInvalidSize, %#v, %#v, err)`, `encode %q (v%d): %w: field %q idx %d: %v`, name, v.Name, i)
						pln(buf, `}`)
					case arrType.Array != nil:
						panic("impossible: can't have an array inside an array")
					case arrType.MappedArray != nil:
						panic("impossible: can't have a mapped array inside an array")
					case arrType.Enum != nil:
						pln(buf, `putEnum(b[%d:%d], uint8(v.%s[%d]))`, offset+szi*i, offset+szi*i+szi, mangle(v.Name, true), i)
					case arrType.Struct != nil:
						pln(buf, `if x, err := v.%s[%d].MarshalBinary(); err != nil {`, mangle(v.Name, true), i)
						pln(buf, `return nil, fmt.Errorf(%#v, %#v, Version, %#v, %#v, err)`, `encode %q (v%d): field %q idx %d: %w`, name, v.Name, i)
						pln(buf, `} else if len(x) != %d {`, szi)
						pln(buf, `panic("bug: invalid marshal struct size")`)
						pln(buf, `} else {`)
						pln(buf, `copy(b[%d:%d], x)`, offset+szi*i, offset+szi*i+szi)
						pln(buf, `}`)
					default:
						panic("bug: unimplemented type in array")
					}
				}
			case v.Type.Enum != nil:
				pln(buf, `putEnum(b[%d:%d], uint8(v.%s))`, offset, offset+sz, mangle(v.Name, true))
			case v.Type.Struct != nil:
				pln(buf, `if x, err := v.%s.MarshalBinary(); err != nil {`, mangle(v.Name, true))
				pln(buf, `return nil, fmt.Errorf(%#v, %#v, Version, %#v, err)`, `encode %q (v%d): field %q: %w`, name, v.Name)
				pln(buf, `} else if len(x) != %d {`, sz)
				pln(buf, `panic("bug: invalid marshal struct size")`)
				pln(buf, `} else {`)
				pln(buf, `copy(b[%d:%d], x)`, offset, offset+sz)
				pln(buf, `}`)
			default:
				panic("bug: unimplemented pdef type")
			}
			offset += sz
		}
		if root {
			pln(buf, `b = append(b, v.%s...)`, mangle("extraData", true))
		}
		pln(buf, `return b, nil`)
		pln(buf, `}`)
	}
	{
		pln(buf, `func (v *%s) UnmarshalJSON(b []byte) error {`, mangle(name, true))
		// TODO: implement this if we actually have a use for it
		pln(buf, `return fmt.Errorf("not implemented")`)
		pln(buf, `}`)
	}
	{
		pln(buf, `func (v %s) MarshalJSON() ([]byte, error) {`, mangle(name, true))
		pln(buf, `return v.MarshalJSONFilter(nil)`)
		pln(buf, `}`)
	}
	{
		pln(buf, `func (v %s) MarshalJSONFilter(filter func(path ...string) bool) ([]byte, error) {`, mangle(name, true))
		if root {
			pln(buf, `if x := v.%s; x != Version {`, mangle(fields[0].Name, true))
			pln(buf, `return nil, fmt.Errorf(%#v, %#v, Version, ErrUnsupportedVersion, x)`, `encode %q (v%d): %w: got %d`, name)
			pln(buf, `}`)
		}
		pln(buf, `return pdataMarshalJSONStruct(v, filter)`)
		pln(buf, `}`)
	}
}

func generateEnum(buf *bytes.Buffer, name string, values []string) {
	pln(buf, `type %s uint8`, mangle(name, true))
	{
		pln(buf, `const (`)
		for i, v := range values {
			pln(buf, `%s %s = %d`, mangleEnumValue(name, v), mangle(name, true), i)
		}
		pln(buf, `%s %s = %d`, mangle(name, true)+"Count", mangle(name, true), len(values))
		pln(buf, `)`)

		pln(buf, `var _ fmt.Stringer = %s(0)`, mangle(name, true))
		pln(buf, `var _ fmt.GoStringer = %s(0)`, mangle(name, true))
		pln(buf, `var _ encoding.TextMarshaler = %s(0)`, mangle(name, true))
		pln(buf, `var _ encoding.TextUnmarshaler = (*%s)(nil)`, mangle(name, true))
		pln(buf, `var _ json.Marshaler = %s(0)`, mangle(name, true))
		pln(buf, `var _ json.Unmarshaler = (*%s)(nil)`, mangle(name, true))
	}
	{
		pln(buf, `func (v %s) String() string {`, mangle(name, true))
		pln(buf, `if b, err := v.MarshalText(); err == nil {`)
		pln(buf, `return string(b)`)
		pln(buf, `}`)
		pln(buf, `return strconv.Itoa(int(v))`)
		pln(buf, `}`)
	}
	{
		pln(buf, `func (v %s) GoString() string {`, mangle(name, true))
		pln(buf, `switch v {`)
		for _, v := range values {
			pln(buf, `case %s:`, mangleEnumValue(name, v))
			pln(buf, `return %#v`, mangleEnumValue(name, v))
		}
		pln(buf, `default:`)
		pln(buf, `return fmt.Sprintf(%#v, %#v, int(v))`, `%s(%d)`, mangle(name, true))
		pln(buf, `}`)
		pln(buf, `}`)
	}
	{
		pln(buf, `func (v %s) MarshalText() ([]byte, error) {`, mangle(name, true))
		pln(buf, `switch v {`)
		for _, v := range values {
			pln(buf, `case %s:`, mangleEnumValue(name, v))
			pln(buf, `return []byte(%#v), nil`, v)
		}
		pln(buf, `default:`)
		pln(buf, `return nil, fmt.Errorf(%#v, ErrInvalidEnumValue, int(v), %#v)`, "%w: invalid value %d for enum %q", mangle(name, true))
		pln(buf, `}`)
		pln(buf, `}`)
	}
	{
		pln(buf, `func (v *%s) UnmarshalText(b []byte) error {`, mangle(name, true))
		pln(buf, `switch string(b) {`)
		for _, v := range values {
			pln(buf, `case %q:`, v)
			pln(buf, `*v = %s`, mangleEnumValue(name, v))
		}
		pln(buf, `default:`)
		pln(buf, `return fmt.Errorf(%#v, ErrInvalidEnumValue, string(b), %#v)`, "%w: invalid value %q for enum %q", mangle(name, true))
		pln(buf, `}`)
		pln(buf, `return nil`)
		pln(buf, `}`)
	}
	{
		pln(buf, `func (v %s) MarshalJSON() ([]byte, error) {`, mangle(name, true))
		pln(buf, `switch v {`)
		for _, v := range values {
			pln(buf, `case %s:`, mangleEnumValue(name, v))
			pln(buf, `return []byte(%#v), nil`, `"`+v+`"`)
		}
		pln(buf, `default:`)
		pln(buf, `return []byte(strconv.Itoa(int(v))), nil`)
		pln(buf, `}`)
		pln(buf, `}`)
	}
	{
		pln(buf, `func (v *%s) UnmarshalJSON(b []byte) error {`, mangle(name, true))
		pln(buf, `switch string(b) {`)
		for _, v := range values {
			pln(buf, `case %#v:`, `"`+v+`"`)
			pln(buf, `*v = %s`, mangleEnumValue(name, v))
		}
		pln(buf, `default:`)
		pln(buf, `return json.Unmarshal(b, (*uint8)(v))`)
		pln(buf, `}`)
		pln(buf, `return nil`)
		pln(buf, `}`)
	}
}

func pdefGoType(t pdef.TypeInfo) string {
	switch {
	case t.Int != nil:
		return "int32"
	case t.Float != nil:
		return "float32"
	case t.Bool != nil:
		return "bool"
	case t.String != nil:
		return "string"
	case t.Struct != nil:
		return mangle(t.Struct.Name, true)
	case t.Enum != nil:
		return mangle(t.Enum.Name, true)
	case t.Array != nil:
		return fmt.Sprintf("[%d]%s", t.Array.Length, pdefGoType(t.Array.Type))
	case t.MappedArray != nil:
		return fmt.Sprintf("[%sCount]%s", mangle(t.MappedArray.Enum, true), pdefGoType(t.MappedArray.Type))
	default:
		panic("unimplemented type")
	}
}

func mangleEnumValue(name, value string) string {
	return mangle(name, true) + "_" + mangle(value, false)
}

func mangle(ident string, upper bool) string {
	x := []rune(ident)
	if len(x) > 2 && x[0] == 's' && unicode.IsUpper(x[1]) {
		x = x[1:] // remove the s/e prefix for struct/enum names
	}
	if upper {
		x[0] = unicode.ToUpper(x[0])
	}
	return string(x)
}

func pln(b *bytes.Buffer, format string, a ...interface{}) {
	fmt.Fprintf(b, format+"\n", a...)
}

func openSingleFile(pattern string) (*os.File, error) {
	ms, err := filepath.Glob(pattern)
	if err != nil {
		return nil, err
	}
	if len(ms) != 1 {
		return nil, fmt.Errorf("expected 1 file matching %q, found %d: %q", pattern, len(ms), ms)
	}
	return os.Open(ms[0])
}

func writeGo(name string, data []byte) error {
	var err error
	if err = os.WriteFile(name, data, 0666); err != nil {
		return err
	}
	if data, err = format.Source(data); err != nil {
		return err
	}
	if err = os.WriteFile(name, data, 0666); err != nil {
		return err
	}
	return nil
}

func each[T any](m map[string]T, fn func(k string, v T)) {
	k := make([]string, 0, len(m))
	for x := range m {
		k = append(k, x)
	}
	sort.Strings(k)
	for _, x := range k {
		fn(x, m[x])
	}
}
