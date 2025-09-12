/*
 * zipindex, (C)2025 MinIO, Inc.
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
	"fmt"
	"iter"
	"sort"
	"strings"
	"sync"

	"github.com/tinylib/msgp/msgp"
)

// LayeredIndex represents multiple indexes layered on top of each other.
// Files from newer layers override files from older layers with the same path.
type LayeredIndex[T comparable] struct {
	layers []layer[T]
}

// layer represents a single index layer with metadata.
type layer[T comparable] struct {
	index    Files
	ref      T
	isDelete bool // If true, files in this layer are deleted from the result
}

// NewLayeredIndex creates a new empty layered index.
func NewLayeredIndex[T comparable]() *LayeredIndex[T] {
	return &LayeredIndex[T]{
		layers: make([]layer[T], 0),
	}
}

// AddLayer adds a new index layer with the given reference.
// Files in this layer will override files with the same path in previous layers.
// Returns an error if a layer with the same reference already exists.
// Files are sorted by name for efficient lookups.
func (l *LayeredIndex[T]) AddLayer(index Files, ref T) error {
	// Check for duplicate reference
	for _, layer := range l.layers {
		if layer.ref == ref {
			return fmt.Errorf("layer with reference %v already exists", ref)
		}
	}
	// Sort files by name for efficient binary search
	index.SortByName()
	l.layers = append(l.layers, layer[T]{
		index:    index,
		ref:      ref,
		isDelete: false,
	})
	return nil
}

// AddDeleteLayer adds a deletion layer with the given reference.
// Files in this layer will be removed from the final result.
// Returns an error if a layer with the same reference already exists.
// Files are sorted by name for efficient lookups.
func (l *LayeredIndex[T]) AddDeleteLayer(index Files, ref T) error {
	// Check for duplicate reference
	for _, layer := range l.layers {
		if layer.ref == ref {
			return fmt.Errorf("layer with reference %v already exists", ref)
		}
	}
	// Sort files by name for efficient binary search
	index.SortByName()
	l.layers = append(l.layers, layer[T]{
		index:    index,
		ref:      ref,
		isDelete: true,
	})
	return nil
}

// LayerCount returns the number of layers in the index.
func (l *LayeredIndex[T]) LayerCount() int {
	return len(l.layers)
}

// GetLayerRef returns the reference for the layer at the given index.
// Returns the zero value of T and false if the index is out of bounds.
func (l *LayeredIndex[T]) GetLayerRef(index int) (T, bool) {
	var zero T
	if index < 0 || index >= len(l.layers) {
		return zero, false
	}
	return l.layers[index].ref, true
}

// RemoveLayer removes the layer at the given index.
// Returns an error if the index is out of bounds.
func (l *LayeredIndex[T]) RemoveLayer(index int) error {
	if index < 0 || index >= len(l.layers) {
		return fmt.Errorf("layer index %d out of bounds [0, %d)", index, len(l.layers))
	}
	l.layers = append(l.layers[:index], l.layers[index+1:]...)
	return nil
}

// RemoveLayerByRef removes all layers with the given reference.
// Returns the number of layers removed.
func (l *LayeredIndex[T]) RemoveLayerByRef(ref T) int {
	removed := 0
	newLayers := make([]layer[T], 0, len(l.layers))
	for _, layer := range l.layers {
		if layer.ref != ref {
			newLayers = append(newLayers, layer)
		} else {
			removed++
		}
	}
	l.layers = newLayers
	return removed
}

// FileWithRef represents a file with its layer reference.
type FileWithRef[T comparable] struct {
	File
	LayerRef T
}

// FilesIter returns an iterator over all files in the layered index.
// Each iteration yields the layer reference and the file.
// Files are returned in name order after applying all layer operations.
func (l *LayeredIndex[T]) FilesIter() iter.Seq2[T, File] {
	return func(yield func(T, File) bool) {
		files := l.Files()
		for _, f := range files {
			if !yield(f.LayerRef, f.File) {
				return
			}
		}
	}
}

// Files returns all files in the layered index after applying layer operations.
// Files from newer layers override files from older layers with the same path.
// Delete layers remove files that exist in previous layers.
func (l *LayeredIndex[T]) Files() []FileWithRef[T] {
	fileMap := make(map[string]FileWithRef[T])

	// Process layers in order
	for _, layer := range l.layers {
		if layer.isDelete {
			// Remove files from the map
			for _, file := range layer.index {
				delete(fileMap, file.Name)
			}

			// After removing all files in this delete layer, check for empty directories
			// We need to check ALL directories to see if they're now empty
			var dirsToCheck []string
			for name := range fileMap {
				if strings.HasSuffix(name, "/") {
					dirsToCheck = append(dirsToCheck, name)
				}
			}

			// Sort dirs by length (deepest first) to check from bottom up
			sort.Slice(dirsToCheck, func(i, j int) bool {
				return len(dirsToCheck[i]) > len(dirsToCheck[j])
			})

			// Check each directory to see if it's empty
			for _, dirPath := range dirsToCheck {
				hasChildren := false
				dirPrefix := dirPath

				// Check if any files or subdirectories exist in this directory
				for name := range fileMap {
					if name != dirPath && strings.HasPrefix(name, dirPrefix) {
						// Check if this is a direct child or deeper descendant
						remainder := name[len(dirPrefix):]
						// If there's content after the prefix, it's a child
						if len(remainder) > 0 {
							hasChildren = true
							break
						}
					}
				}

				if !hasChildren {
					delete(fileMap, dirPath)
				}
			}
		} else {
			// Add or override files
			for _, file := range layer.index {
				fileMap[file.Name] = FileWithRef[T]{
					File:     file,
					LayerRef: layer.ref,
				}
			}
		}
	}

	// Convert map to slice
	result := make([]FileWithRef[T], 0, len(fileMap))
	for _, file := range fileMap {
		result = append(result, file)
	}

	// Sort by name for consistent output
	sort.Slice(result, func(i, j int) bool {
		return result[i].Name < result[j].Name
	})

	return result
}

// binarySearchFile performs a binary search for a file by name in a sorted Files slice.
func binarySearchFile(files Files, name string) *File {
	left, right := 0, len(files)-1
	for left <= right {
		mid := (left + right) / 2
		if files[mid].Name == name {
			return &files[mid]
		}
		if files[mid].Name < name {
			left = mid + 1
		} else {
			right = mid - 1
		}
	}
	return nil
}

// Find searches for a file by name across all layers using binary search.
// Returns the file and its layer reference if found.
// Delete layers remove the file if it exists in previous layers.
// Empty directories are automatically considered deleted.
func (l *LayeredIndex[T]) Find(name string) (*FileWithRef[T], bool) {
	var found *FileWithRef[T]

	// Process layers in order
	for _, layer := range l.layers {
		if layer.isDelete {
			// Binary search in sorted delete layer
			if file := binarySearchFile(layer.index, name); file != nil {
				// File was deleted
				found = nil
			}
		} else {
			// Binary search in sorted regular layer
			if file := binarySearchFile(layer.index, name); file != nil {
				found = &FileWithRef[T]{
					File:     *file,
					LayerRef: layer.ref,
				}
			}
		}
	}

	// For directories, we need to check if they should be auto-removed due to being empty
	// This requires checking the full state, so we use Files() for consistency
	if found != nil && strings.HasSuffix(name, "/") {
		// Use Files() to get the accurate state with directory cleanup applied
		files := l.Files()
		for i := range files {
			if files[i].Name == name {
				return &files[i], true
			}
		}
		// Directory was removed as empty
		return nil, false
	}

	return found, found != nil
}

// FindInLayer searches for a file by name in a specific layer using binary search.
// Returns the file if found in the specified layer.
func (l *LayeredIndex[T]) FindInLayer(name string, ref T) (*File, bool) {
	for _, layer := range l.layers {
		if layer.ref == ref {
			// Use binary search in the sorted layer
			if file := binarySearchFile(layer.index, name); file != nil {
				return file, true
			}
			return nil, false
		}
	}
	return nil, false
}

// ToSingleIndex merges all layers into a single Files collection.
// Files from newer layers override files from older layers with the same path.
// Files in delete layers are removed from the result.
func (l *LayeredIndex[T]) ToSingleIndex() Files {
	filesWithRef := l.Files()
	result := make(Files, len(filesWithRef))
	for i, f := range filesWithRef {
		result[i] = f.File
	}
	return result
}

// Clear removes all layers from the index.
func (l *LayeredIndex[T]) Clear() {
	l.layers = l.layers[:0]
}

// IsEmpty returns true if the index has no files after applying all layer operations.
// This accounts for files that have been deleted by delete layers.
func (l *LayeredIndex[T]) IsEmpty() bool {
	return l.FileCount() == 0
}

// FileCount returns the total number of unique files after applying all layer operations.
func (l *LayeredIndex[T]) FileCount() int {
	return len(l.Files())
}

// HasFile returns true if the file exists in the layered index after applying all operations.
func (l *LayeredIndex[T]) HasFile(name string) bool {
	_, found := l.Find(name)
	return found
}

// RefSerializer provides functions to convert layer references to/from byte slices.
type RefSerializer[T comparable] struct {
	// Marshal converts a reference to bytes
	Marshal func(T) ([]byte, error)
	// Unmarshal converts bytes to a reference
	Unmarshal func([]byte) (T, error)
}

// SerializeLayered serializes the layered index with all layers preserved.
// Uses concurrent serialization for better performance with large indexes.
func (l *LayeredIndex[T]) SerializeLayered(refSerializer RefSerializer[T]) ([]byte, error) {
	if refSerializer.Marshal == nil {
		return nil, fmt.Errorf("marshal function is required")
	}

	// Write header manually using msgp
	// Format: [version:uint8, layers:uint32]
	result := make([]byte, 0, 1024)

	// Write version (uint8)
	result = msgp.AppendUint8(result, 1)

	// Write number of layers (uint32)
	result = msgp.AppendUint32(result, uint32(len(l.layers)))

	// Serialize layers concurrently
	type layerResult struct {
		index int
		data  []byte
		err   error
	}

	results := make(chan layerResult, len(l.layers))
	var wg sync.WaitGroup

	for i, lay := range l.layers {
		wg.Add(1)
		go func(idx int, layer layer[T]) {
			defer wg.Done()

			// Serialize reference
			refData, err := refSerializer.Marshal(layer.ref)
			if err != nil {
				results <- layerResult{index: idx, err: fmt.Errorf("failed to marshal ref for layer %d: %w", idx, err)}
				return
			}

			// Serialize files
			filesData, err := layer.index.Serialize()
			if err != nil {
				results <- layerResult{index: idx, err: fmt.Errorf("failed to serialize files for layer %d: %w", idx, err)}
				return
			}

			// Manual msgpack serialization of layer
			// Format: [refData:bin, isDelete:bool, filesData:bin]
			layerBuf := make([]byte, 0, len(refData)+len(filesData)+64)

			// Write ref data as binary
			layerBuf = msgp.AppendBytes(layerBuf, refData)

			// Write isDelete flag
			layerBuf = msgp.AppendBool(layerBuf, layer.isDelete)

			// Write files data as binary
			layerBuf = msgp.AppendBytes(layerBuf, filesData)

			results <- layerResult{index: idx, data: layerBuf}
		}(i, lay)
	}

	// Wait for all goroutines to complete
	wg.Wait()
	close(results)

	// Collect results in order
	layerData := make([][]byte, len(l.layers))
	for res := range results {
		if res.err != nil {
			return nil, res.err
		}
		layerData[res.index] = res.data
	}

	// Append all layer data in order
	for _, data := range layerData {
		result = append(result, data...)
	}

	return result, nil
}

// DeserializeLayered reconstructs a layered index from serialized data.
// Uses concurrent deserialization for better performance with large indexes.
func DeserializeLayered[T comparable](data []byte, refSerializer RefSerializer[T]) (*LayeredIndex[T], error) {
	if refSerializer.Unmarshal == nil {
		return nil, fmt.Errorf("unmarshal function is required")
	}

	// Read header manually using msgp
	// Format: [version:uint8, layers:uint32]
	var err error
	remaining := data

	// Read version
	var version uint8
	version, remaining, err = msgp.ReadUint8Bytes(remaining)
	if err != nil {
		return nil, fmt.Errorf("failed to read version: %w", err)
	}

	if version != 1 {
		return nil, fmt.Errorf("unsupported version: %d", version)
	}

	// Read number of layers
	var numLayers uint32
	numLayers, remaining, err = msgp.ReadUint32Bytes(remaining)
	if err != nil {
		return nil, fmt.Errorf("failed to read layer count: %w", err)
	}

	// Parse layer data
	layerData := make([][]byte, 0, numLayers)
	for i := uint32(0); i < numLayers; i++ {
		// Peek ahead to find the size of this layer
		tempRemaining := remaining

		// Read ref data (skip)
		var refData []byte
		refData, tempRemaining, err = msgp.ReadBytesZC(tempRemaining)
		if err != nil {
			return nil, fmt.Errorf("failed to read ref data for layer %d: %w", i, err)
		}
		_ = refData

		// Read isDelete flag (skip)
		var isDelete bool
		isDelete, tempRemaining, err = msgp.ReadBoolBytes(tempRemaining)
		if err != nil {
			return nil, fmt.Errorf("failed to read isDelete for layer %d: %w", i, err)
		}
		_ = isDelete

		// Read files data (skip)
		var filesData []byte
		filesData, tempRemaining, err = msgp.ReadBytesZC(tempRemaining)
		if err != nil {
			return nil, fmt.Errorf("failed to read files data for layer %d: %w", i, err)
		}
		_ = filesData

		// Calculate layer size and save it
		layerSize := len(remaining) - len(tempRemaining)
		layerBytes := remaining[:layerSize]
		layerData = append(layerData, layerBytes)
		remaining = tempRemaining
	}

	// Deserialize layers concurrently
	type layerResult struct {
		index int
		layer layer[T]
		err   error
	}

	results := make(chan layerResult, len(layerData))
	var wg sync.WaitGroup

	for i, layerBytes := range layerData {
		wg.Add(1)
		go func(idx int, data []byte) {
			defer wg.Done()

			// Manual msgpack deserialization of layer
			// Format: [refData:bin, isDelete:bool, filesData:bin]

			// Read ref data
			refData, data, err := msgp.ReadBytesZC(data)
			if err != nil {
				results <- layerResult{index: idx, err: fmt.Errorf("failed to read ref data for layer %d: %w", idx, err)}
				return
			}

			// Read isDelete flag
			isDelete, data, err := msgp.ReadBoolBytes(data)
			if err != nil {
				results <- layerResult{index: idx, err: fmt.Errorf("failed to read isDelete for layer %d: %w", idx, err)}
				return
			}

			// Read files data
			filesData, _, err := msgp.ReadBytesZC(data)
			if err != nil {
				results <- layerResult{index: idx, err: fmt.Errorf("failed to read files data for layer %d: %w", idx, err)}
				return
			}

			// Unmarshal reference
			ref, err := refSerializer.Unmarshal(refData)
			if err != nil {
				results <- layerResult{index: idx, err: fmt.Errorf("failed to unmarshal ref for layer %d: %w", idx, err)}
				return
			}

			// Deserialize files
			files, err := DeserializeFiles(filesData)
			if err != nil {
				results <- layerResult{index: idx, err: fmt.Errorf("failed to deserialize files for layer %d: %w", idx, err)}
				return
			}

			// Files are already sorted from AddLayer, but ensure they're sorted
			files.SortByName()

			results <- layerResult{
				index: idx,
				layer: layer[T]{
					index:    files,
					ref:      ref,
					isDelete: isDelete,
				},
			}
		}(i, layerBytes)
	}

	// Wait for all goroutines to complete
	wg.Wait()
	close(results)

	// Collect results in order
	layers := make([]layer[T], len(layerData))
	for res := range results {
		if res.err != nil {
			return nil, res.err
		}
		layers[res.index] = res.layer
	}

	// Create the layered index
	l := NewLayeredIndex[T]()
	l.layers = layers

	return l, nil
}
