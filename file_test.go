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
	"encoding/hex"
	"fmt"
	"math/rand"
	"reflect"
	"strings"
	"testing"
	"time"
)

func BenchmarkFindSerialized(b *testing.B) {
	sizes := []int{1e2, 1e3, 1e4, 1e5, 1e6}
	for _, n := range sizes {
		b.Run(fmt.Sprint(n), func(b *testing.B) {
			files := make(Files, n)
			rng := rand.New(rand.NewSource(int64(n)))
			off := int64(0)
			var tmp [8]byte
			for i := range files {
				rng.Read(tmp[:])
				f := File{
					Name:               "files/" + hex.EncodeToString(tmp[:]) + ".txt",
					CRC32:              rng.Uint32(),
					Method:             Deflate,
					Flags:              2,
					Offset:             off,
					UncompressedSize64: uint64(rng.Intn(64 << 10)),
				}
				f.CompressedSize64 = f.UncompressedSize64 / 2
				off += int64(f.UncompressedSize64) + int64(len(f.Name)+20+rng.Intn(40))
				files[i] = f
			}
			ser, err := files.Serialize()
			if err != nil {
				b.Fatal(err)
			}
			b.ResetTimer()
			b.ReportAllocs()
			t := time.Now()
			for i := 0; i < b.N; i++ {
				get := rng.Intn(n)
				f, err := FindSerialized(ser, files[get].Name)
				if err != nil {
					b.Fatal(err)
				}
				if !reflect.DeepEqual(*f, files[get]) {
					b.Fatalf("%+v != %+v", *f, files[get])
				}
			}
			b.ReportMetric(float64(b.N)/float64(time.Since(t)/time.Second), "files/s")
			b.ReportMetric(float64(len(ser))/float64(n), "b/file")
		})
	}
}

func TestSerializeDoesNotModifyInput(t *testing.T) {
	const n = 60000
	f := make(Files, n)
	for i := range f {
		// Neither name nor offset order, so any in-place sort changes the slice.
		j := (i * 7919) % n
		f[i] = File{Name: fmt.Sprintf("f%06d", j), Offset: int64(n-j) * 100, CompressedSize64: 10}
	}
	want := append(Files(nil), f...)
	if _, err := f.Serialize(); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(f, want) {
		t.Fatal("Serialize modified input")
	}
}

func TestSerializeMaxSize(t *testing.T) {
	if testing.Short() {
		t.Skip("allocates several hundred MiB")
	}
	// Compresses well, so only the uncompressed size exceeds the limit.
	big := map[string]string{"k": strings.Repeat("a", MaxIndexSize)}
	type3 := make(Files, 10)
	type3[0].Custom = big

	// Random names don't compress. Each chunk is below the limit, the total is not.
	rng := rand.New(rand.NewSource(0))
	type4 := make(Files, 50000)
	for i := range type4 {
		name := make([]byte, 2800)
		rng.Read(name)
		type4[i].Name = string(name)
	}

	for name, f := range map[string]Files{
		"type2": {{Custom: big}},
		"type3": type3,
		"type4": type4,
	} {
		if _, err := f.Serialize(); err != ErrMaxSizeExceeded {
			t.Errorf("%s: got error %v, want %v", name, err, ErrMaxSizeExceeded)
		}
	}
}
