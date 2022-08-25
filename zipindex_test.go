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
	"io/ioutil"
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
		"fuzz/FuzzDeserializeFiles.zip",
		"fuzz/FuzzRoundtrip.zip",
	}
	for _, test := range testSet {
		t.Run(test, func(t *testing.T) {
			input, err := ioutil.ReadFile(filepath.Join("testdata", test))
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
				wantData, wantErr := ioutil.ReadAll(wantRC)
				gotData, err := ioutil.ReadAll(rc)
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
