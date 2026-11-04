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
	"encoding/hex"
	"errors"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/klauspost/compress/zstd"
)

func TestReadDir(t *testing.T) {
	testSet := []string{
		"big.zip",
		"crc32-not-streamed.zip",
		"dd.zip",
		"go-no-datadesc-sig.zip",
		"gophercolor16x16.png",
		"go-with-datadesc-sig.zip",
		"readme.notzip",
		"readme.zip",
		"symlink.zip",
		"test.zip",
		"test-trailing-junk.zip",
		"time-22738.zip",
		"time-7zip.zip",
		"time-go.zip",
		"time-infozip.zip",
		"time-osx.zip",
		"time-win7.zip",
		"time-winrar.zip",
		"time-winzip.zip",
		"unix.zip",
		"utf8-7zip.zip",
		"utf8-infozip.zip",
		"utf8-osx.zip",
		"utf8-winrar.zip",
		"utf8-winzip.zip",
		"winxp.zip",
		"zip64.zip",
		"zip64-2.zip",
		"smallish.zip",
		"zstd-compressed.zip",
		"test-badbase.zip",
		"comment-truncated.zip",
		"fuzz/FuzzDeserializeFiles.zip",
		"fuzz/FuzzRoundtrip.zip",
	}
	for _, test := range testSet {
		t.Run(test, func(t *testing.T) {
			input, err := os.ReadFile(filepath.Join("testdata", test))
			if err != nil {
				t.Fatal(err)
			}
			zr, err := zip.NewReader(bytes.NewReader(input), int64(len(input)))
			if err != nil {
				// We got an error, we should also get one from ourself
				_, err := ReadDir(input, int64(len(input)), DefaultFileFilter)
				if err == nil {
					t.Errorf("want error, like %v, got none", err)
				}
				return
			}
			zr.RegisterDecompressor(zstd.ZipMethodWinZip, zstd.ZipDecompressor(zstd.WithDecoderLowmem(true)))
			sz := 8 << 10
			if sz > len(input) {
				// Truncate a bit from the start...
				sz = len(input) - 10
			}
			var files Files
			for {
				files, err = ReadDir(input[len(input)-sz:], int64(len(input)),
					func(dst *File, entry *ZipDirEntry) *File {
						dst.Custom = map[string]string{
							"modified": entry.Modified.UTC().String(),
						}
						return DefaultFileFilter(dst, entry)
					})
				if err == nil {
					break
				}
				var more ErrNeedMoreData
				if !errors.As(err, &more) {
					t.Errorf("unexpected error: %v", err)
					return
				}
				t.Logf("wanted more: %d bytes from end...", more.FromEnd)
				sz = int(more.FromEnd)
			}
			files.OptimizeSize()
			files.RemoveInsecurePaths()
			ser, err := files.Serialize()
			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}
			t.Log("Serialized size:", len(ser), "Files:", len(files))
			files, err = DeserializeFiles(ser)
			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}
			var nFiles int
			for _, file := range zr.File {
				if !file.Mode().IsRegular() {
					if files.Find(file.Name) != nil {
						t.Errorf("found non-regular file %v", file.Name)
					}
					continue
				}
				nFiles++
				gotFile := files.Find(file.Name)
				if gotFile == nil {
					t.Errorf(" could not find regular file %v", file.Name)
					continue
				}
				if f, err := FindSerialized(ser, file.Name); err != nil || f == nil {
					t.Errorf("FindSerialized: could not find regular file %v, err: %v, file: %v", file.Name, err, f)
					continue
				} else {
					if !reflect.DeepEqual(*f, *gotFile) {
						t.Errorf("FindSerialized returned %+v\nfiles.Find returned: %+v", *f, *gotFile)
					}
				}

				wantRC, wantErr := file.Open()
				rc, err := gotFile.Open(bytes.NewReader(input[gotFile.Offset:]))
				if err != nil {
					if wantErr != nil {
						continue
					}
					t.Error("got error:", err)
					return
				}
				if wantErr != nil {
					t.Errorf("want error, like %v, got none", wantErr)
				}
				defer func() {
					wantErr := wantRC.Close()
					gotErr := rc.Close()
					if wantErr != gotErr {
						t.Errorf("err mismatch: %v != %v", wantErr, gotErr)
					}
				}()
				wantData, wantErr := io.ReadAll(wantRC)
				gotData, err := io.ReadAll(rc)
				if err != nil {
					if err == wantErr {
						continue
					}
					t.Error("got error:", err)
					return
				}
				if !bytes.Equal(wantData, gotData) {
					t.Error("data mismatch")
				}
			}
			if nFiles > 0 {
				t.Logf("%.02f bytes/file", float64(len(ser))/float64(nFiles))
			}
		})
	}
}

// TestReadDirInfoZIPMaxUint32Fields checks archives written by Info-ZIP 3.0
// where a 32-bit size or header offset is exactly 0xFFFFFFFF. Info-ZIP stores
// such values directly, without a Zip64 extra field.
// The tails are from the standard library's testdata/zip64 goldens.
func TestReadDirInfoZIPMaxUint32Fields(t *testing.T) {
	type wantFile struct {
		name   string
		size   uint64
		offset int64
	}
	tests := []struct {
		name  string
		size  int64  // archive size
		tail  string // hex of the archive's last bytes
		files []wantFile
	}{
		{
			// Stored entry of 4GiB-1 bytes: both sizes are 0xFFFFFFFF.
			// The tail starts at the central directory.
			name: "infozip-store-4g-minus-1",
			size: 4294967491,
			tail: "504b01021e030a00000000005d63b25c00000000ffffffffffffffff0b000000" +
				"0000000000000000a4810000000062696734672d312e62696e504b06062c0000" +
				"00000000001e032d000000000000000000010000000000000001000000000000" +
				"0039000000000000002800000001000000504b06070000000061000000010000" +
				"0001000000504b0506000000000100010039000000ffffffff0000",
			files: []wantFile{{name: "big4g-1.bin", size: 1<<32 - 1}},
		},
		{
			// The second local file header starts at offset 0xFFFFFFFF,
			// which is where the tail starts.
			name: "infozip-offset-eq-4g",
			size: 4294967580,
			tail: "504b03040a00000000005d63b25cfb286fe42a0000002a00000009000000736d" +
				"616c6c2e62696e00000000000000000000000000000000000000000000000000" +
				"0000000000000000000000000000000000504b01021e030a00000000005d63b2" +
				"5c63944eaddcffffffdcffffff050000000000000000000000a4810000000066" +
				"69727374504b01021e030a00000000005d63b25cfb286fe42a0000002a000000" +
				"090000000000000000000000a481ffffffff736d616c6c2e62696e504b06062c" +
				"000000000000001e032d00000000000000000002000000000000000200000000" +
				"0000006a000000000000005000000001000000504b060700000000ba00000001" +
				"00000001000000504b050600000000020002006a000000ffffffff0000",
			files: []wantFile{
				{name: "first", size: 1<<32 - 36},
				{name: "small.bin", size: 42, offset: 1<<32 - 1},
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			tail, err := hex.DecodeString(test.tail)
			if err != nil {
				t.Fatal(err)
			}
			files, err := ReadDir(tail, test.size, nil)
			if err != nil {
				t.Fatalf("ReadDir: %v", err)
			}
			if len(files) != len(test.files) {
				t.Fatalf("got %d files, want %d", len(files), len(test.files))
			}
			for i, want := range test.files {
				f := files[i]
				if f.Name != want.name || f.CompressedSize64 != want.size || f.UncompressedSize64 != want.size || f.Offset != want.offset {
					t.Errorf("file %d: got %q sizes %d/%d offset %d, want %q size %d offset %d",
						i, f.Name, f.CompressedSize64, f.UncompressedSize64, f.Offset, want.name, want.size, want.offset)
				}
			}

			// Read the last entry if its local header is in the tail.
			f := files[len(files)-1]
			if start := test.size - int64(len(tail)); f.Offset >= start {
				rc, err := f.Open(bytes.NewReader(tail[f.Offset-start:]))
				if err != nil {
					t.Fatalf("%s: Open: %v", f.Name, err)
				}
				got, err := io.ReadAll(rc)
				if err != nil {
					t.Fatalf("%s: read: %v", f.Name, err)
				}
				if !bytes.Equal(got, make([]byte, f.UncompressedSize64)) {
					t.Errorf("%s: content = %x, want %d zero bytes", f.Name, got, f.UncompressedSize64)
				}
			}
		})
	}
}
