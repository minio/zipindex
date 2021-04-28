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

package zipindex_test

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"

	"github.com/minio/zipindex"
)

func ExampleReadDir() {
	b, err := ioutil.ReadFile("testdata/big.zip")
	if err != nil {
		panic(err)
	}
	// We only need the end of the file to parse the directory.
	// Usually this should be at least 64K on initial try.
	sz := 10 << 10
	var files zipindex.Files
	for {
		files, err = zipindex.ReadDir(b[len(b)-sz:], int64(len(b)), nil)
		if err == nil {
			fmt.Printf("Got %d files\n", len(files))
			break
		}
		var terr zipindex.ErrNeedMoreData
		if errors.As(err, &terr) {
			if terr.FromEnd > 1<<20 {
				panic("we will only provide max 1MB data")
			}
			sz = int(terr.FromEnd)
			fmt.Printf("Retrying with %d bytes at the end of file\n", sz)
		} else {
			// Unable to parse...
			panic(err)
		}
	}

	fmt.Printf("First file: %+v", files[0])
	// Output:
	// Retrying with 57912 bytes at the end of file
	// Got 1000 files
	// First file: {Name:file-0.txt CompressedSize64:1 UncompressedSize64:1 Offset:0 CRC32:4108050209 Method:0 Flags:0 Custom:map[]}
}

// ExampleReadFile demonstrates how to read the index of a file on disk.
func ExampleReadFile() {
	files, err := zipindex.ReadFile("testdata/go-with-datadesc-sig.zip", nil)
	if err != nil {
		panic(err)
	}
	fmt.Printf("Got %d files\n", len(files))
	fmt.Printf("First file: %+v", files[0])
	// Output:
	// Got 2 files
	// First file: {Name:foo.txt CompressedSize64:4 UncompressedSize64:4 Offset:0 CRC32:2117232040 Method:0 Flags:8 Custom:map[]}
}

// ExampleReadFile demonstrates how to read the index of a file on disk.
func ExampleReaderAt() {
	f, err := os.Open("testdata/big.zip")
	if err != nil {
		panic(err)
	}
	fi, err := f.Stat()
	if err != nil {
		panic(err)
	}

	// Read and allow up to 10MB index.
	files, err := zipindex.ReaderAt(f, fi.Size(), 10<<20, nil)
	if err != nil {
		panic(err)
	}
	fmt.Printf("Got %d files\n", len(files))
	fmt.Printf("First file: %+v", files[0])
	// Output:
	// Got 1000 files
	// First file: {Name:file-0.txt CompressedSize64:1 UncompressedSize64:1 Offset:0 CRC32:4108050209 Method:0 Flags:0 Custom:map[]}
}

// ExampleFileFilter demonstrates how to filter incoming files.
func ExampleFileFilter() {
	files, err := zipindex.ReadFile("testdata/unix.zip",
		func(dst *zipindex.File, entry *zipindex.ZipDirEntry) *zipindex.File {
			if dst.Name == "hello" {
				// Filter out on specific properties.
				return nil
			}
			// Add custom data.
			if dst.Custom == nil {
				dst.Custom = make(map[string]string, 3)
			}
			dst.Custom["modified"] = entry.Modified.String()
			dst.Custom["perm"] = fmt.Sprintf("0%o", entry.Mode().Perm())
			if len(entry.Comment) > 0 {
				dst.Custom["comment"] = entry.Comment
			}
			return dst
		})
	if err != nil {
		panic(err)
	}
	fmt.Printf("Got %d files\n", len(files))
	for i, file := range files {
		fmt.Printf("%d: %+v\n", i, file)
	}
	// Output:
	// Got 3 files
	// 0: {Name:dir/bar CompressedSize64:6 UncompressedSize64:6 Offset:71 CRC32:2055117726 Method:0 Flags:0 Custom:map[modified:2011-12-08 10:04:50 +0000 +0000 perm:0666]}
	// 1: {Name:dir/empty/ CompressedSize64:0 UncompressedSize64:0 Offset:142 CRC32:0 Method:0 Flags:0 Custom:map[modified:2011-12-08 10:08:06 +0000 +0000 perm:0777]}
	// 2: {Name:readonly CompressedSize64:12 UncompressedSize64:12 Offset:210 CRC32:3127775578 Method:0 Flags:0 Custom:map[modified:2011-12-08 10:06:08 +0000 +0000 perm:0444]}
}

// ExampleReadFile demonstrates how to read the index of a file on disk.
func ExampleFindSerialized() {
	files, err := zipindex.ReadFile("testdata/go-with-datadesc-sig.zip", nil)
	if err != nil {
		panic(err)
	}
	files.OptimizeSize()
	serialized, err := files.Serialize()
	if err != nil {
		panic(err)
	}

	file, err := zipindex.FindSerialized(serialized, "bar.txt")
	if err != nil {
		panic(err)
	}
	fmt.Printf("bar.txt: %+v", *file)
	// Output:
	// bar.txt: {Name:bar.txt CompressedSize64:4 UncompressedSize64:4 Offset:57 CRC32:0 Method:0 Flags:8 Custom:map[]}
}

func ExampleDeserializeFiles() {
	exitOnErr := func(err error) {
		if err != nil {
			log.Fatalln(err)
		}
	}

	b, err := ioutil.ReadFile("testdata/big.zip")
	exitOnErr(err)
	// We only need the end of the file to parse the directory.
	// Usually this should be at least 64K on initial try.
	sz := 64 << 10
	var files zipindex.Files
	files, err = zipindex.ReadDir(b[len(b)-sz:], int64(len(b)), nil)
	// Omitted: Check if ErrNeedMoreData and retry with more data
	exitOnErr(err)

	// OptimizeSize files will make the size as efficient as possible
	// without loosing data.
	files.OptimizeSize()

	// Serialize files to binary.
	serialized, err := files.Serialize()
	exitOnErr(err)

	// This output may change if compression is improved.
	// Output is rounded up.
	fmt.Printf("Size of serialized data: %dKB\n", (len(serialized)+1023)/1024)

	// StripCRC(true) will strip CRC, even if there is no file descriptor.
	files.StripCRC(true)
	// StripFlags(1<<3) will strip all flags that aren't a file descriptor flag (bit 3).
	files.StripFlags(1 << 3)
	noCRC, err := files.Serialize()
	exitOnErr(err)

	// This output may change if compression is improved.
	// Output is rounded up.
	fmt.Printf("Size of serialized data without CRC: %dKB\n", (len(noCRC)+1023)/1024)

	// Deserialize the content (with CRC).
	files, err = zipindex.DeserializeFiles(serialized)
	exitOnErr(err)

	file := files.Find("file-10.txt")
	fmt.Printf("Reading file: %+v\n", *file)

	// Create a reader with entire zip file...
	rs := bytes.NewReader(b)
	// Seek to the file offset.
	_, err = rs.Seek(file.Offset, io.SeekStart)
	exitOnErr(err)

	// Provide the forwarded reader..
	rc, err := file.Open(rs)
	exitOnErr(err)
	defer rc.Close()

	// Read the zip file content.
	content, err := ioutil.ReadAll(rc)
	exitOnErr(err)

	fmt.Printf("File content is '%s'\n", string(content))

	// Output:
	// Size of serialized data: 6KB
	// Size of serialized data without CRC: 1KB
	// Reading file: {Name:file-10.txt CompressedSize64:2 UncompressedSize64:2 Offset:410 CRC32:2707236321 Method:0 Flags:0 Custom:map[]}
	// File content is '10'
}
