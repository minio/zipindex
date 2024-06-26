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
	"encoding/binary"
	"errors"
	"fmt"
	"hash/crc32"
	"io"
	"path/filepath"
	"sort"
	"strings"

	"github.com/klauspost/compress/zstd"
	"github.com/tinylib/msgp/msgp"
)

//go:generate msgp -file $GOFILE -unexported

// File is a sparse representation of a File inside a zip file.
//
//msgp:tuple File
type File struct {
	Name               string // Name of the file as stored in the zip.
	CompressedSize64   uint64 // Size of compressed data, excluding ZIP headers.
	UncompressedSize64 uint64 // Size of the Uncompressed data.
	Offset             int64  // Offset where file data header starts.
	CRC32              uint32 // CRC of the uncompressed data.
	Method             uint16 // Storage method.
	Flags              uint16 // General purpose bit flag

	// Custom data.
	Custom map[string]string
}

// Open returns a ReadCloser that provides access to the File's contents.
// The Reader 'r' must be forwarded to f.Offset before being provided.
func (f *File) Open(r io.Reader) (io.ReadCloser, error) {
	if err := f.skipToBody(r); err != nil {
		return nil, err
	}
	size := int64(f.CompressedSize64)
	dcomp := decompressor(f.Method)
	if dcomp == nil {
		return nil, ErrAlgorithm
	}
	compReader := io.LimitReader(r, size)
	var rc = dcomp(compReader)
	rc = &checksumReader{
		compReader: compReader,
		rc:         rc,
		hash:       crc32.NewIEEE(),
		f:          f,
		raw:        r,
	}
	return rc, nil
}

// OpenRaw returns a Reader that returns the *compressed* output of the file.
func (f *File) OpenRaw(r io.Reader) (io.Reader, error) {
	if err := f.skipToBody(r); err != nil {
		return nil, err
	}
	return io.LimitReader(r, int64(f.CompressedSize64)), nil
}

// Files is a collection of files.
//
//msgp:ignore Files
type Files []File

// unexported type to hide generated serialization functions.
type files []File

// Any changes to the file format MUST cause an increment here and MUST
// be backwards compatible.
const currentVerPlain = 1
const currentVerCompressed = 2
const currentVerCompressedStructs = 3

var zstdEnc, _ = zstd.NewWriter(nil, zstd.WithWindowSize(128<<10), zstd.WithEncoderConcurrency(2), zstd.WithEncoderLevel(zstd.SpeedBetterCompression))
var zstdDec, _ = zstd.NewReader(nil, zstd.WithDecoderLowmem(true), zstd.WithDecoderConcurrency(2), zstd.WithDecoderMaxMemory(MaxIndexSize), zstd.WithDecoderMaxWindow(8<<20))

//msgp:tuple filesAsStructs
type filesAsStructs struct {
	Names   [][]byte
	CSizes  []int64
	USizes  []int64
	Offsets []int64
	Methods []uint16
	Flags   []uint16
	Crcs    []byte
	Custom  [][]byte
}

// Serialize the files.
func (f Files) Serialize() ([]byte, error) {
	if len(f) > MaxFiles {
		return nil, ErrTooManyFiles
	}

	if len(f) < 10 {
		for _, file := range f {
			if len(file.Custom) > MaxCustomEntries {
				return nil, ErrTooManyCustomEntries
			}
		}
		payload, err := files(f).MarshalMsg(nil)
		if err != nil {
			return nil, err
		}
		res := make([]byte, 0, len(payload))
		if len(payload) < 200 {
			res = append(res, currentVerPlain)
			return append(res, payload...), nil
		}
		res = append(res, currentVerCompressed)
		return zstdEnc.EncodeAll(payload, res), nil
	}

	// Encode many files as struct of arrays...
	x := filesAsStructs{
		Names:   make([][]byte, len(f)),
		CSizes:  make([]int64, len(f)),
		USizes:  make([]int64, len(f)),
		Offsets: make([]int64, len(f)),
		Methods: make([]uint16, len(f)),
		Flags:   make([]uint16, len(f)),
		Crcs:    make([]byte, len(f)*4),
		Custom:  make([][]byte, len(f)),
	}
	for i, file := range f {
		x.CSizes[i] = int64(file.CompressedSize64)
		if i > 0 {
			// Try to predict offset from previous file..
			file.Offset -= f[i-1].Offset + int64(f[i-1].CompressedSize64) + fileHeaderLen + int64(len(f[i-1].Name)+dataDescriptorLen)
			// Only encode when method changes.
			file.Method ^= f[i-1].Method
			file.Flags ^= f[i-1].Flags
			// Use previous size as base.
			x.CSizes[i] = int64(file.CompressedSize64) - int64(f[i-1].CompressedSize64)
		}
		x.Names[i] = []byte(file.Name)
		// Uncompressed size is the size from the compressed.
		x.USizes[i] = int64(file.UncompressedSize64) - int64(f[i].CompressedSize64)
		x.Offsets[i] = file.Offset
		x.Methods[i] = file.Method
		x.Flags[i] = file.Flags
		binary.LittleEndian.PutUint32(x.Crcs[i*4:], file.CRC32)
		if len(file.Custom) > 0 {
			if len(file.Custom) > MaxCustomEntries {
				return nil, ErrTooManyCustomEntries
			}
			x.Custom[i] = msgp.AppendMapStrStr(nil, file.Custom)
		}
	}
	payload, err := x.MarshalMsg(nil)
	if err != nil {
		return nil, err
	}
	res := make([]byte, 0, len(payload))
	res = append(res, currentVerCompressedStructs)
	return zstdEnc.EncodeAll(payload, res), nil
}

// RemoveInsecurePaths will remove any file with path deemed insecure.
// This is files that fail either !filepath.IsLocal(file.Name) or contain a backslash.
func (f *Files) RemoveInsecurePaths() {
	files := *f
	for i, file := range files {
		if file.Name == "" {
			// Zip permits an empty file name field.
			continue
		}
		// The zip specification states that names must use forward slashes,
		// so consider any backslashes in the name insecure.
		if !filepath.IsLocal(file.Name) || strings.Contains(file.Name, `\`) {
			files = append(files[:i], files[:i+1]...)
		}
	}
	*f = files
}

// Sort files by offset in zip file.
// Typically, directories are already sorted by offset.
// This will usually provide the smallest possible serialized size.
func (f Files) Sort() {
	less := func(i, j int) bool {
		a, b := f[i], f[j]
		return a.Offset < b.Offset
	}
	if !sort.SliceIsSorted(f, less) {
		sort.Slice(f, less)
	}
}

// Find the file with the provided name.
// Search is linear.
func (f Files) Find(name string) *File {
	for _, file := range f {
		if file.Name == name {
			return &file
		}
	}
	return nil
}

// OptimizeSize will sort entries and strip CRC data when the file has a file descriptor.
func (f Files) OptimizeSize() {
	f.Sort()
	f.StripCRC(false)
}

// StripCRC will zero out the CRC for all files if there is a data descriptor
// (which will contain a CRC) or optionally for all.
func (f Files) StripCRC(all bool) {
	for i, file := range f {
		if all || file.hasDataDescriptor() {
			f[i].CRC32 = 0
		}
	}
}

// StripFlags will zero out the Flags, except the ones provided in mask.
func (f Files) StripFlags(mask uint16) {
	for i, file := range f {
		f[i].Flags = file.Flags & mask
	}
}

// unpackPayload unpacks and optionally decompresses the payload.
func unpackPayload(b []byte) ([]byte, bool, error) {
	if len(b) < 1 {
		return nil, false, io.ErrUnexpectedEOF
	}
	if len(b) > MaxIndexSize {
		return nil, false, ErrMaxSizeExceeded
	}
	var out []byte
	switch b[0] {
	case currentVerPlain:
		out = b[1:]
	case currentVerCompressed, currentVerCompressedStructs:
		decoded, err := zstdDec.DecodeAll(b[1:], nil)
		if err != nil {
			switch err {
			case zstd.ErrDecoderSizeExceeded, zstd.ErrWindowSizeExceeded:
				err = ErrMaxSizeExceeded
			}
			return nil, false, err
		}
		out = decoded
	default:
		return nil, false, errors.New("unknown version")
	}
	return out, b[0] == currentVerCompressedStructs, nil
}

// DeserializeFiles will de-serialize the files.
func DeserializeFiles(b []byte) (Files, error) {
	b, structs, err := unpackPayload(b)
	if err != nil {
		return nil, err
	}
	if !structs {
		var dst files
		// Check number of files.
		nFiles, _, err := msgp.ReadArrayHeaderBytes(b)
		if nFiles > 100 {
			return nil, ErrTooManyFiles
		}
		if err != nil {
			return nil, err
		}
		_, err = dst.UnmarshalMsg(b)
		return Files(dst), err
	}

	var dst filesAsStructs
	if _, err = dst.UnmarshalMsg(b); err != nil {
		return nil, err
	}

	files := make(Files, len(dst.Names))
	var cur File
	for i := range files {
		cur = File{
			Name:             string(dst.Names[i]),
			CompressedSize64: uint64(dst.CSizes[i] + int64(cur.CompressedSize64)),
			CRC32:            binary.LittleEndian.Uint32(dst.Crcs[i*4:]),
			Method:           dst.Methods[i] ^ cur.Method,
			Flags:            dst.Flags[i] ^ cur.Flags,
		}
		cur.UncompressedSize64 = uint64(dst.USizes[i] + int64(cur.CompressedSize64))
		if i == 0 {
			cur.Offset = dst.Offsets[i]
		} else {
			cur.Offset = dst.Offsets[i] + files[i-1].Offset + int64(files[i-1].CompressedSize64) + fileHeaderLen + int64(len(files[i-1].Name)) + dataDescriptorLen
		}
		if len(dst.Custom[i]) > 0 {
			if cur.Custom, err = readCustomData(dst.Custom[i]); err != nil {
				return nil, err
			}
		}
		files[i] = cur

	}
	return files, err
}

func readCustomData(bts []byte) (dst map[string]string, err error) {
	var zb0002 uint32
	zb0002, bts, err = msgp.ReadMapHeaderBytes(bts)
	if err != nil {
		err = msgp.WrapError(err, "Custom")
		return
	}
	if zb0002 > MaxCustomEntries {
		err = msgp.WrapError(ErrTooManyCustomEntries, "Custom", zb0002)
		return
	}
	dst = make(map[string]string, zb0002)
	for zb0002 > 0 {
		var za0001 string
		var za0002 string
		zb0002--
		za0001, bts, err = msgp.ReadStringBytes(bts)
		if err != nil {
			err = msgp.WrapError(err, "Custom")
			return
		}
		za0002, bts, err = msgp.ReadStringBytes(bts)
		if err != nil {
			err = msgp.WrapError(err, "Custom", za0001)
			return
		}
		dst[za0001] = za0002
	}
	return dst, nil
}

// FindSerialized will locate a file by name and return it.
// This will be less resource intensive than decoding all files,
// if only one it requested.
// Expected speed scales O(n) for n files.
// Returns nil, io.EOF if not found.
func FindSerialized(b []byte, name string) (*File, error) {
	buf, structs, err := unpackPayload(b)
	if err != nil {
		return nil, err
	}
	if !structs {
		n, buf, err := msgp.ReadArrayHeaderBytes(buf)
		if err != nil {
			return nil, err
		}
		if n > 100 {
			return nil, ErrTooManyFiles
		}
		var f File
		for i := 0; i < int(n); i++ {
			buf, err = f.UnmarshalMsg(buf)
			if err != nil {
				return nil, err
			}
			if f.Name == name {
				return &f, nil
			}
		}
		return nil, io.EOF
	}

	// Files are packed as an array of arrays...
	idx := -1
	var zb0001 uint32
	bts := buf
	zb0001, bts, err = msgp.ReadArrayHeaderBytes(bts)
	if err != nil {
		err = msgp.WrapError(err, "Files")
		return nil, err
	}
	const arraySize = 8
	if zb0001 != arraySize {
		err = msgp.ArrayError{Wanted: arraySize, Got: zb0001}
		return nil, err
	}
	var nFiles uint32
	nFiles, bts, err = msgp.ReadArrayHeaderBytes(bts)
	if err != nil {
		err = msgp.WrapError(err, "Names")
		return nil, err
	}
	if nFiles > MaxFiles {
		return nil, ErrTooManyFiles
	}

	// We accumulate values needed for cur as we parse...
	var cur File
	for i := 0; i < int(nFiles); i++ {
		var got []byte
		got, bts, err = msgp.ReadBytesZC(bts)
		if err != nil {
			err = msgp.WrapError(err, "Names-Field")
			return nil, err
		}
		if idx >= 0 {
			continue
		}
		if string(got) == name {
			idx = i
			continue
		}
		// Names add to offset...
		cur.Offset += int64(len(got))
	}
	if idx < 0 {
		return nil, io.EOF
	}

	cur.Name = name
	for field := 0; field < arraySize-1; field++ {
		// CRC is just a single array.
		if field == 5 {
			var Crcs []byte
			Crcs, bts, err = msgp.ReadBytesZC(bts)
			if err != nil {
				err = msgp.WrapError(err, "Crcs")
				return nil, err
			}
			if len(Crcs) != int(nFiles*4) {
				return nil, fmt.Errorf("CRC field too short, want %d, got %d", int(nFiles*4), len(Crcs))
			}
			cur.CRC32 = binary.LittleEndian.Uint32(Crcs[idx*4:])
			continue
		}

		var elems uint32
		elems, bts, err = msgp.ReadArrayHeaderBytes(bts)
		if err != nil {
			err = msgp.WrapError(err, fmt.Sprintf("field-%d", field))
			return nil, err
		}
		if elems != nFiles {
			return nil, fmt.Errorf("unexpected array size, got %d, want %d", elems, nFiles)
		}

		for i := 0; i < int(nFiles); i++ {
			var val64 int64
			switch field {
			case 0: // CSizes []int64
				val64, bts, err = msgp.ReadInt64Bytes(bts)
				if err != nil {
					err = msgp.WrapError(err, "CSizes")
					return nil, err
				}
				if i > idx {
					continue
				}
				cur.CompressedSize64 = uint64(int64(cur.CompressedSize64) + val64)
				if i < idx {
					// Compressed size adds to offset for all before idx.
					cur.Offset += int64(cur.CompressedSize64)
				}
			case 1: // USizes []int64
				val64, bts, err = msgp.ReadInt64Bytes(bts)
				if err != nil {
					err = msgp.WrapError(err, "USizes")
					return nil, err
				}
				if i > idx {
					continue
				}
				cur.UncompressedSize64 = uint64(int64(cur.CompressedSize64) + val64)
			case 2: // Offsets []int64
				val64, bts, err = msgp.ReadInt64Bytes(bts)
				if err != nil {
					err = msgp.WrapError(err, "Offsets")
					return nil, err
				}
				if i > idx {
					continue
				}
				// Offset adds to offset
				cur.Offset += val64
				if i > 0 {
					// Add constants...
					cur.Offset += fileHeaderLen + dataDescriptorLen
				}
			case 3: // Methods []uint16
				var val16 uint16
				val16, bts, err = msgp.ReadUint16Bytes(bts)
				if err != nil {
					err = msgp.WrapError(err, "Methods")
					return nil, err
				}
				if i > idx {
					continue
				}
				cur.Method ^= val16
			case 4: // Flags []uint16
				var val16 uint16
				val16, bts, err = msgp.ReadUint16Bytes(bts)
				if err != nil {
					err = msgp.WrapError(err, "Flags")
					return nil, err
				}
				if i > idx {
					continue
				}
				cur.Flags ^= val16
			case 6: // Custom
				var custom []byte
				custom, bts, err = msgp.ReadBytesZC(bts)
				if err != nil {
					err = msgp.WrapError(err, fmt.Sprintf("field-%d", field))
					return nil, err
				}
				if i != idx || len(custom) == 0 {
					continue
				}
				cur.Custom, err = readCustomData(custom)
				if err != nil {
					err = msgp.WrapError(err, "Custom Data")
					return nil, err
				}
			default:
				panic("internal error, unexpected field")
			}
		}
	}
	return &cur, nil
}

//msgp:unmarshal ignore File

// UnmarshalMsg implements msgp.Unmarshaler
func (f *File) UnmarshalMsg(bts []byte) (o []byte, err error) {
	var zb0001 uint32
	zb0001, bts, err = msgp.ReadArrayHeaderBytes(bts)
	if err != nil {
		err = msgp.WrapError(err)
		return
	}
	if zb0001 != 8 {
		err = msgp.ArrayError{Wanted: 8, Got: zb0001}
		return
	}
	f.Name, bts, err = msgp.ReadStringBytes(bts)
	if err != nil {
		err = msgp.WrapError(err, "Name")
		return
	}
	f.CompressedSize64, bts, err = msgp.ReadUint64Bytes(bts)
	if err != nil {
		err = msgp.WrapError(err, "CompressedSize64")
		return
	}
	f.UncompressedSize64, bts, err = msgp.ReadUint64Bytes(bts)
	if err != nil {
		err = msgp.WrapError(err, "UncompressedSize64")
		return
	}
	f.Offset, bts, err = msgp.ReadInt64Bytes(bts)
	if err != nil {
		err = msgp.WrapError(err, "Offset")
		return
	}
	f.CRC32, bts, err = msgp.ReadUint32Bytes(bts)
	if err != nil {
		err = msgp.WrapError(err, "CRC32")
		return
	}
	f.Method, bts, err = msgp.ReadUint16Bytes(bts)
	if err != nil {
		err = msgp.WrapError(err, "Method")
		return
	}
	f.Flags, bts, err = msgp.ReadUint16Bytes(bts)
	if err != nil {
		err = msgp.WrapError(err, "Flags")
		return
	}
	var zb0002 uint32
	zb0002, bts, err = msgp.ReadMapHeaderBytes(bts)
	if err != nil {
		err = msgp.WrapError(err, "Custom")
		return
	}
	if zb0002 > MaxCustomEntries {
		err = msgp.WrapError(ErrTooManyCustomEntries, "Custom", zb0002)
		return
	}
	if f.Custom == nil {
		f.Custom = make(map[string]string, zb0002)
	} else if len(f.Custom) > 0 {
		for key := range f.Custom {
			delete(f.Custom, key)
		}
	}
	for zb0002 > 0 {
		var za0001 string
		var za0002 string
		zb0002--
		za0001, bts, err = msgp.ReadStringBytes(bts)
		if err != nil {
			err = msgp.WrapError(err, "Custom")
			return
		}
		za0002, bts, err = msgp.ReadStringBytes(bts)
		if err != nil {
			err = msgp.WrapError(err, "Custom", za0001)
			return
		}
		f.Custom[za0001] = za0002
	}
	o = bts
	return
}

//msgp:unmarshal ignore filesAsStructs

// UnmarshalMsg implements msgp.Unmarshaler
func (z *filesAsStructs) UnmarshalMsg(bts []byte) (o []byte, err error) {
	var zb0001 uint32
	zb0001, bts, err = msgp.ReadArrayHeaderBytes(bts)
	if err != nil {
		err = msgp.WrapError(err)
		return
	}
	if zb0001 != 8 {
		err = msgp.ArrayError{Wanted: 8, Got: zb0001}
		return
	}
	var zb0002 uint32
	zb0002, bts, err = msgp.ReadArrayHeaderBytes(bts)
	if err != nil {
		err = msgp.WrapError(err, "Names")
		return
	}
	if zb0002 > MaxFiles {
		err = msgp.WrapError(err, "ErrTooManyFiles", zb0002)
		return
	}
	if cap(z.Names) >= int(zb0002) {
		z.Names = (z.Names)[:zb0002]
	} else {
		z.Names = make([][]byte, zb0002)
	}
	for za0001 := range z.Names {
		z.Names[za0001], bts, err = msgp.ReadBytesBytes(bts, z.Names[za0001])
		if err != nil {
			err = msgp.WrapError(err, "Names", za0001)
			return
		}
	}
	var zb0003 uint32
	zb0003, bts, err = msgp.ReadArrayHeaderBytes(bts)
	if err != nil {
		err = msgp.WrapError(err, "CSizes")
		return
	}
	if zb0003 != zb0002 {
		err = msgp.WrapError(errors.New("field number mismatch"), "CSizes", zb0003)
		return
	}
	if cap(z.CSizes) >= int(zb0003) {
		z.CSizes = (z.CSizes)[:zb0003]
	} else {
		z.CSizes = make([]int64, zb0003)
	}
	for za0002 := range z.CSizes {
		z.CSizes[za0002], bts, err = msgp.ReadInt64Bytes(bts)
		if err != nil {
			err = msgp.WrapError(err, "CSizes", za0002)
			return
		}
	}
	var zb0004 uint32
	zb0004, bts, err = msgp.ReadArrayHeaderBytes(bts)
	if err != nil {
		err = msgp.WrapError(err, "USizes")
		return
	}
	if zb0004 != zb0002 {
		err = msgp.WrapError(errors.New("field number mismatch"), "USizes", zb0004)
		return
	}

	if cap(z.USizes) >= int(zb0004) {
		z.USizes = (z.USizes)[:zb0004]
	} else {
		z.USizes = make([]int64, zb0004)
	}
	for za0003 := range z.USizes {
		z.USizes[za0003], bts, err = msgp.ReadInt64Bytes(bts)
		if err != nil {
			err = msgp.WrapError(err, "USizes", za0003)
			return
		}
	}
	var zb0005 uint32
	zb0005, bts, err = msgp.ReadArrayHeaderBytes(bts)
	if err != nil {
		err = msgp.WrapError(err, "Offsets")
		return
	}
	if zb0005 != zb0002 {
		err = msgp.WrapError(errors.New("field number mismatch"), "Offsets", zb0005)
		return
	}

	if cap(z.Offsets) >= int(zb0005) {
		z.Offsets = (z.Offsets)[:zb0005]
	} else {
		z.Offsets = make([]int64, zb0005)
	}
	for za0004 := range z.Offsets {
		z.Offsets[za0004], bts, err = msgp.ReadInt64Bytes(bts)
		if err != nil {
			err = msgp.WrapError(err, "Offsets", za0004)
			return
		}
	}
	var zb0006 uint32
	zb0006, bts, err = msgp.ReadArrayHeaderBytes(bts)
	if err != nil {
		err = msgp.WrapError(err, "Methods")
		return
	}
	if zb0006 != zb0002 {
		err = msgp.WrapError(errors.New("field number mismatch"), "Methods", zb0006)
		return
	}
	if cap(z.Methods) >= int(zb0006) {
		z.Methods = (z.Methods)[:zb0006]
	} else {
		z.Methods = make([]uint16, zb0006)
	}
	for za0005 := range z.Methods {
		z.Methods[za0005], bts, err = msgp.ReadUint16Bytes(bts)
		if err != nil {
			err = msgp.WrapError(err, "Methods", za0005)
			return
		}
	}
	var zb0007 uint32
	zb0007, bts, err = msgp.ReadArrayHeaderBytes(bts)
	if err != nil {
		err = msgp.WrapError(err, "Flags")
		return
	}
	if zb0007 != zb0002 {
		err = msgp.WrapError(errors.New("field number mismatch"), "Flags", zb0007)
		return
	}

	if cap(z.Flags) >= int(zb0007) {
		z.Flags = (z.Flags)[:zb0007]
	} else {
		z.Flags = make([]uint16, zb0007)
	}
	for za0006 := range z.Flags {
		z.Flags[za0006], bts, err = msgp.ReadUint16Bytes(bts)
		if err != nil {
			err = msgp.WrapError(err, "Flags", za0006)
			return
		}
	}
	z.Crcs, bts, err = msgp.ReadBytesBytes(bts, z.Crcs)
	if err != nil {
		err = msgp.WrapError(err, "Crcs")
		return
	}
	var zb0008 uint32
	zb0008, bts, err = msgp.ReadArrayHeaderBytes(bts)
	if err != nil {
		err = msgp.WrapError(err, "Custom")
		return
	}
	if zb0008 != zb0002 {
		err = msgp.WrapError(errors.New("field number mismatch"), "Custom")
		return
	}

	if cap(z.Custom) >= int(zb0008) {
		z.Custom = (z.Custom)[:zb0008]
	} else {
		z.Custom = make([][]byte, zb0008)
	}
	for za0007 := range z.Custom {
		z.Custom[za0007], bts, err = msgp.ReadBytesBytes(bts, z.Custom[za0007])
		if err != nil {
			err = msgp.WrapError(err, "Custom", za0007)
			return
		}
	}
	o = bts
	return
}
