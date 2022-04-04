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

// Contains code that is
//
// Copyright 2010 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package zipindex

import (
	"errors"
	"io"
	"io/ioutil"
	"sync"

	"github.com/klauspost/compress/flate"
	"github.com/klauspost/compress/zstd"
)

// A Decompressor returns a new decompressing reader, reading from r.
// The ReadCloser's Close method must be used to release associated resources.
// The Decompressor itself must be safe to invoke from multiple goroutines
// simultaneously, but each returned reader will be used only by
// one goroutine at a time.
type Decompressor func(r io.Reader) io.ReadCloser

var flateReaderPool sync.Pool

func newFlateReader(r io.Reader) io.ReadCloser {
	fr, ok := flateReaderPool.Get().(io.ReadCloser)
	if ok {
		fr.(flate.Resetter).Reset(r, nil)
	} else {
		fr = flate.NewReader(r)
	}
	return &pooledFlateReader{fr: fr}
}

type pooledFlateReader struct {
	mu sync.Mutex // guards Close and Read
	fr io.ReadCloser
}

func (r *pooledFlateReader) Read(p []byte) (n int, err error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.fr == nil {
		return 0, errors.New("Read after Close")
	}
	return r.fr.Read(p)
}

func (r *pooledFlateReader) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	var err error
	if r.fr != nil {
		err = r.fr.Close()
		flateReaderPool.Put(r.fr)
		r.fr = nil
	}
	return err
}

var zstdReaderPool sync.Pool

// newZstdReader creates a pooled zip decompressor.
func newZstdReader(r io.Reader) io.ReadCloser {
	dec, ok := zstdReaderPool.Get().(*zstd.Decoder)
	if ok {
		dec.Reset(r)
	} else {
		d, err := zstd.NewReader(r, zstd.WithDecoderConcurrency(1), zstd.WithDecoderLowmem(true), zstd.WithDecoderMaxWindow(128<<20))
		if err != nil {
			panic(err)
		}
		dec = d
	}
	return &pooledZipReader{dec: dec}
}

type pooledZipReader struct {
	mu  sync.Mutex // guards Close and Read
	dec *zstd.Decoder
}

func (r *pooledZipReader) Read(p []byte) (n int, err error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.dec == nil {
		return 0, errors.New("read after close or EOF")
	}
	dec, err := r.dec.Read(p)
	if err == io.EOF {
		r.dec.Reset(nil)
		zstdReaderPool.Put(r.dec)
		r.dec = nil
	}
	return dec, err
}

func (r *pooledZipReader) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	var err error
	if r.dec != nil {
		err = r.dec.Reset(nil)
		zstdReaderPool.Put(r.dec)
		r.dec = nil
	}
	return err
}

var (
	decompressors sync.Map // map[uint16]Decompressor
)

func init() {
	decompressors.Store(Store, Decompressor(ioutil.NopCloser))
	decompressors.Store(Deflate, Decompressor(newFlateReader))
	// TODO: Use zstd one when https://github.com/klauspost/compress/pull/539 is released.
	decompressors.Store(uint16(zstd.ZipMethodWinZip), Decompressor(newZstdReader))
}

// RegisterDecompressor allows custom decompressors for a specified method ID.
// The common methods Store (0) and Deflate (8) and Zstandard (93) are built in.
func RegisterDecompressor(method uint16, dcomp Decompressor) {
	decompressors.Store(method, dcomp)
}

func decompressor(method uint16) Decompressor {
	di, ok := decompressors.Load(method)
	if !ok {
		return nil
	}
	return di.(Decompressor)
}
