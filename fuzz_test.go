//+build go1.18
//go:build go1.18

/*
 * zipindex, (C)2021 MinIO, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package zipindex

import (
	"archive/zip"
	"bytes"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"io/ioutil"
	"os"
	"strconv"
	"testing"
)

//var n uint32

// Fuzz a roundtrip.
func FuzzRoundtrip(f *testing.F) {
	addBytesFromZip(f, "testdata/fuzz/FuzzRoundtrip.zip")
	f.Fuzz(func(t *testing.T, b []byte) {
		exitOnErr := func(err error) {
			if err != nil {
				t.Fatal(err)
			}
		}

		sz := 1 << 10
		if sz > len(b) {
			sz = len(b)
		}
		var files Files
		var err error
		for {
			files, err = ReadDir(b[len(b)-sz:], int64(len(b)), func(dst *File, entry *ZipDirEntry) *File {
				return dst
			})
			if err == nil {
				break
			}
			var terr ErrNeedMoreData
			if errors.As(err, &terr) {
				if terr.FromEnd > int64(len(b)) {
					return
				}
				sz = int(terr.FromEnd)
			} else {
				// Unable to parse...
				return
			}
		}
		// Serialize files to binary.
		serialized, err := files.Serialize()
		if errors.Is(err, ErrTooManyFiles) {
			return
		}
		exitOnErr(err)

		// Deserialize the content.
		files, err = DeserializeFiles(serialized)
		exitOnErr(err)

		if len(files) == 0 {
			return
		}
		for _, file := range files {
			// Create a reader with entire zip file...
			rs := bytes.NewReader(b)
			// Seek to the file offset.
			_, err = rs.Seek(file.Offset, io.SeekStart)
			if err != nil {
				continue
			}

			// Provide the forwarded reader..
			rc, err := file.Open(rs)
			if err != nil {
				continue
			}
			defer rc.Close()

			// Read the zip file content.
			ioutil.ReadAll(rc)
		}
	})
}

func FuzzDeserializeFiles(f *testing.F) {
	f.Add([]byte{0x1, 0x90})
	addBytesFromZip(f, "testdata/fuzz/FuzzDeserializeFiles.zip")
	f.Fuzz(func(t *testing.T, b []byte) {
		// Just test if we crash...
		defer func() {
			if r := recover(); r != nil {
				t.Log(r)
				t.Fatal(r)
			}
		}()
		DeserializeFiles(b)
		FindSerialized(b, "a.txt")
	})
}

func addBytesFromZip(f *testing.F, filename string) {
	file, err := os.Open(filename)
	if err != nil {
		f.Fatal(err)
	}
	fi, err := file.Stat()
	if err != nil {
		f.Fatal(err)
	}
	zr, err := zip.NewReader(file, fi.Size())
	if err != nil {
		f.Fatal(err)
	}
	for _, file := range zr.File {
		rc, err := file.Open()
		if err != nil {
			f.Fatal(err)
		}

		b, err := io.ReadAll(rc)
		if err != nil {
			f.Fatal(err)
		}
		rc.Close()
		vals, err := unmarshalCorpusFile(b)
		if err != nil {
			f.Fatal(err)
		}
		for _, v := range vals {
			f.Add(v)
		}
	}
}

func TestDeserializeFiles(t *testing.T) {
	b := []byte("\x01\x91\x98\xd93aasdgasgdiausgdiashdas.as\x0014\a\x01\x00\xce\x00\xdfiasdasd-1237814dgasidgb\xdf\xdf\xdf\b\x80")
	DeserializeFiles(b)
	FindSerialized(b, "a.txt")
}

// unmarshalCorpusFile decodes corpus bytes into their respective values.
func unmarshalCorpusFile(b []byte) ([][]byte, error) {
	if len(b) == 0 {
		return nil, fmt.Errorf("cannot unmarshal empty string")
	}
	lines := bytes.Split(b, []byte("\n"))
	if len(lines) < 2 {
		return nil, fmt.Errorf("must include version and at least one value")
	}
	var vals = make([][]byte, 0, len(lines)-1)
	for _, line := range lines[1:] {
		line = bytes.TrimSpace(line)
		if len(line) == 0 {
			continue
		}
		v, err := parseCorpusValue(line)
		if err != nil {
			return nil, fmt.Errorf("malformed line %q: %v", line, err)
		}
		vals = append(vals, v)
	}
	return vals, nil
}

// parseCorpusValue
func parseCorpusValue(line []byte) ([]byte, error) {
	fs := token.NewFileSet()
	expr, err := parser.ParseExprFrom(fs, "(test)", line, 0)
	if err != nil {
		return nil, err
	}
	call, ok := expr.(*ast.CallExpr)
	if !ok {
		return nil, fmt.Errorf("expected call expression")
	}
	if len(call.Args) != 1 {
		return nil, fmt.Errorf("expected call expression with 1 argument; got %d", len(call.Args))
	}
	arg := call.Args[0]

	if arrayType, ok := call.Fun.(*ast.ArrayType); ok {
		if arrayType.Len != nil {
			return nil, fmt.Errorf("expected []byte or primitive type")
		}
		elt, ok := arrayType.Elt.(*ast.Ident)
		if !ok || elt.Name != "byte" {
			return nil, fmt.Errorf("expected []byte")
		}
		lit, ok := arg.(*ast.BasicLit)
		if !ok || lit.Kind != token.STRING {
			return nil, fmt.Errorf("string literal required for type []byte")
		}
		s, err := strconv.Unquote(lit.Value)
		if err != nil {
			return nil, err
		}
		return []byte(s), nil
	}
	return nil, fmt.Errorf("expected []byte")
}
