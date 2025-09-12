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
	"encoding/binary"
	"fmt"
	"reflect"
	"slices"
	"sort"
	"testing"
)

// Test basic creation and layer management
func TestLayeredIndex_Basic(t *testing.T) {
	l := NewLayeredIndex[string]()

	// Test empty index
	if !l.IsEmpty() {
		t.Error("New layered index should be empty")
	}
	if l.LayerCount() != 0 {
		t.Errorf("Expected 0 layers, got %d", l.LayerCount())
	}
	if l.FileCount() != 0 {
		t.Errorf("Expected 0 files, got %d", l.FileCount())
	}

	// Add first layer
	files1 := Files{
		{Name: "file1.txt", CompressedSize64: 100, UncompressedSize64: 150},
		{Name: "file2.txt", CompressedSize64: 200, UncompressedSize64: 250},
	}
	err := l.AddLayer(files1, "layer1")
	if err != nil {
		t.Errorf("Failed to add layer: %v", err)
	}

	if l.IsEmpty() {
		t.Error("Index should not be empty after adding files")
	}
	if l.LayerCount() != 1 {
		t.Errorf("Expected 1 layer, got %d", l.LayerCount())
	}
	if l.FileCount() != 2 {
		t.Errorf("Expected 2 files, got %d", l.FileCount())
	}

	// Test duplicate reference
	err = l.AddLayer(Files{}, "layer1")
	if err == nil {
		t.Error("Should have failed to add layer with duplicate reference")
	}

	// Add second layer
	files2 := Files{
		{Name: "file3.txt", CompressedSize64: 300, UncompressedSize64: 350},
	}
	err = l.AddLayer(files2, "layer2")
	if err != nil {
		t.Errorf("Failed to add second layer: %v", err)
	}

	if l.LayerCount() != 2 {
		t.Errorf("Expected 2 layers, got %d", l.LayerCount())
	}
	if l.FileCount() != 3 {
		t.Errorf("Expected 3 files, got %d", l.FileCount())
	}
}

// Test file override semantics
func TestLayeredIndex_Override(t *testing.T) {
	l := NewLayeredIndex[int]()

	// Add base layer
	files1 := Files{
		{Name: "file1.txt", CompressedSize64: 100, CRC32: 1111},
		{Name: "file2.txt", CompressedSize64: 200, CRC32: 2222},
		{Name: "file3.txt", CompressedSize64: 300, CRC32: 3333},
	}
	l.AddLayer(files1, 1)

	// Add override layer
	files2 := Files{
		{Name: "file2.txt", CompressedSize64: 999, CRC32: 9999}, // Override file2
		{Name: "file4.txt", CompressedSize64: 400, CRC32: 4444}, // New file
	}
	l.AddLayer(files2, 2)

	// Check that file2 was overridden
	file, found := l.Find("file2.txt")
	if !found {
		t.Error("file2.txt should exist")
	}
	if file.CRC32 != 9999 {
		t.Errorf("file2.txt should have been overridden, got CRC %d", file.CRC32)
	}
	if file.LayerRef != 2 {
		t.Errorf("file2.txt should be from layer 2, got layer %d", file.LayerRef)
	}

	// Check that file1 is still from layer 1
	file, found = l.Find("file1.txt")
	if !found {
		t.Error("file1.txt should exist")
	}
	if file.LayerRef != 1 {
		t.Errorf("file1.txt should be from layer 1, got layer %d", file.LayerRef)
	}

	// Check total file count
	if l.FileCount() != 4 {
		t.Errorf("Expected 4 unique files, got %d", l.FileCount())
	}
}

// Test delete layer semantics
func TestLayeredIndex_Delete(t *testing.T) {
	l := NewLayeredIndex[string]()

	// Add base layer
	files1 := Files{
		{Name: "file1.txt", CompressedSize64: 100},
		{Name: "file2.txt", CompressedSize64: 200},
		{Name: "file3.txt", CompressedSize64: 300},
	}
	l.AddLayer(files1, "base")

	// Add delete layer
	deleteFiles := Files{
		{Name: "file2.txt"},
	}
	err := l.AddDeleteLayer(deleteFiles, "delete1")
	if err != nil {
		t.Errorf("Failed to add delete layer: %v", err)
	}

	// Check that file2 was deleted
	_, found := l.Find("file2.txt")
	if found {
		t.Error("file2.txt should have been deleted")
	}

	// Check that other files still exist
	if !l.HasFile("file1.txt") {
		t.Error("file1.txt should still exist")
	}
	if !l.HasFile("file3.txt") {
		t.Error("file3.txt should still exist")
	}

	// Check file count
	if l.FileCount() != 2 {
		t.Errorf("Expected 2 files after deletion, got %d", l.FileCount())
	}

	// Add file2 back in a new layer - it should reappear
	files2 := Files{
		{Name: "file2.txt", CompressedSize64: 999},
	}
	l.AddLayer(files2, "restore")

	// Check that file2 is back
	file, found := l.Find("file2.txt")
	if !found {
		t.Error("file2.txt should exist after being re-added")
	}
	if file.LayerRef != "restore" {
		t.Errorf("file2.txt should be from 'restore' layer, got %s", file.LayerRef)
	}
	if file.CompressedSize64 != 999 {
		t.Errorf("file2.txt should have new size 999, got %d", file.CompressedSize64)
	}
}

// Test directory handling with deletions
func TestLayeredIndex_DirectoryHandling(t *testing.T) {
	l := NewLayeredIndex[string]()

	// Add files with directory structure
	files1 := Files{
		{Name: "dir1/"},
		{Name: "dir1/file1.txt"},
		{Name: "dir1/file2.txt"},
		{Name: "dir1/subdir/"},
		{Name: "dir1/subdir/file3.txt"},
		{Name: "dir2/"},
		{Name: "dir2/file4.txt"},
	}
	l.AddLayer(files1, "base")

	if l.FileCount() != 7 {
		t.Errorf("Expected 7 entries, got %d", l.FileCount())
	}

	// Delete a file from dir1/subdir
	deleteFiles := Files{
		{Name: "dir1/subdir/file3.txt"},
	}
	l.AddDeleteLayer(deleteFiles, "delete1")

	// Check that the file is deleted but empty directory is also removed
	if l.HasFile("dir1/subdir/file3.txt") {
		t.Error("dir1/subdir/file3.txt should be deleted")
	}
	if l.HasFile("dir1/subdir/") {
		t.Error("dir1/subdir/ should be deleted as it's now empty")
	}

	// dir1/ should still exist as it has other files
	if !l.HasFile("dir1/") {
		t.Error("dir1/ should still exist")
	}
	if !l.HasFile("dir1/file1.txt") {
		t.Error("dir1/file1.txt should still exist")
	}

	// Delete all files from dir1
	deleteFiles2 := Files{
		{Name: "dir1/file1.txt"},
		{Name: "dir1/file2.txt"},
	}
	l.AddDeleteLayer(deleteFiles2, "delete2")

	// Now dir1/ should also be removed
	if l.HasFile("dir1/") {
		t.Error("dir1/ should be deleted as it's now empty")
	}

	// dir2 should still exist
	if !l.HasFile("dir2/") {
		t.Error("dir2/ should still exist")
	}
	if !l.HasFile("dir2/file4.txt") {
		t.Error("dir2/file4.txt should still exist")
	}
}

// Test Files() method
func TestLayeredIndex_Files(t *testing.T) {
	l := NewLayeredIndex[int]()

	// Add multiple layers
	l.AddLayer(Files{
		{Name: "a.txt", CompressedSize64: 1},
		{Name: "b.txt", CompressedSize64: 2},
	}, 1)

	l.AddLayer(Files{
		{Name: "b.txt", CompressedSize64: 22}, // Override
		{Name: "c.txt", CompressedSize64: 3},
	}, 2)

	l.AddDeleteLayer(Files{
		{Name: "a.txt"},
	}, 3)

	l.AddLayer(Files{
		{Name: "d.txt", CompressedSize64: 4},
		{Name: "a.txt", CompressedSize64: 11}, // Re-add after delete
	}, 4)

	files := l.Files()

	// Should have 4 files: a.txt (from layer 4), b.txt (from layer 2), c.txt (from layer 2), d.txt (from layer 4)
	if len(files) != 4 {
		t.Errorf("Expected 4 files, got %d", len(files))
	}

	// Check files are sorted by name
	expectedNames := []string{"a.txt", "b.txt", "c.txt", "d.txt"}
	for i, f := range files {
		if f.Name != expectedNames[i] {
			t.Errorf("Expected file %s at index %d, got %s", expectedNames[i], i, f.Name)
		}
	}

	// Check layer references
	fileMap := make(map[string]int)
	for _, f := range files {
		fileMap[f.Name] = f.LayerRef
	}

	if fileMap["a.txt"] != 4 {
		t.Errorf("a.txt should be from layer 4, got %d", fileMap["a.txt"])
	}
	if fileMap["b.txt"] != 2 {
		t.Errorf("b.txt should be from layer 2, got %d", fileMap["b.txt"])
	}
	if fileMap["c.txt"] != 2 {
		t.Errorf("c.txt should be from layer 2, got %d", fileMap["c.txt"])
	}
	if fileMap["d.txt"] != 4 {
		t.Errorf("d.txt should be from layer 4, got %d", fileMap["d.txt"])
	}
}

// Test FilesIter iterator
func TestLayeredIndex_FilesIter(t *testing.T) {
	l := NewLayeredIndex[string]()

	l.AddLayer(Files{
		{Name: "file1.txt", CompressedSize64: 100},
		{Name: "file2.txt", CompressedSize64: 200},
	}, "layer1")

	l.AddLayer(Files{
		{Name: "file3.txt", CompressedSize64: 300},
	}, "layer2")

	// Collect all files using iterator
	var collected []struct {
		ref  string
		name string
		size uint64
	}

	for ref, file := range l.FilesIter() {
		collected = append(collected, struct {
			ref  string
			name string
			size uint64
		}{ref, file.Name, file.CompressedSize64})
	}

	if len(collected) != 3 {
		t.Errorf("Expected 3 files from iterator, got %d", len(collected))
	}

	// Files should be in name order
	expectedNames := []string{"file1.txt", "file2.txt", "file3.txt"}
	for i, item := range collected {
		if item.name != expectedNames[i] {
			t.Errorf("Expected %s at index %d, got %s", expectedNames[i], i, item.name)
		}
	}

	// Test early termination
	count := 0
	for range l.FilesIter() {
		count++
		if count == 2 {
			break
		}
	}
	if count != 2 {
		t.Errorf("Expected to break after 2 iterations, got %d", count)
	}
}

// Test layer management methods
func TestLayeredIndex_LayerManagement(t *testing.T) {
	l := NewLayeredIndex[string]()

	// Add multiple layers
	l.AddLayer(Files{{Name: "f1.txt"}}, "layer1")
	l.AddLayer(Files{{Name: "f2.txt"}}, "layer2")
	l.AddLayer(Files{{Name: "f3.txt"}}, "layer3")
	l.AddLayer(Files{{Name: "f4.txt"}}, "layer4")

	// Test GetLayerRef
	ref, ok := l.GetLayerRef(1)
	if !ok || ref != "layer2" {
		t.Errorf("Expected layer2 at index 1, got %s", ref)
	}

	_, ok = l.GetLayerRef(10)
	if ok {
		t.Error("GetLayerRef should return false for out of bounds index")
	}

	// Test RemoveLayer
	err := l.RemoveLayer(1) // Remove layer2
	if err != nil {
		t.Errorf("Failed to remove layer: %v", err)
	}

	if l.LayerCount() != 3 {
		t.Errorf("Expected 3 layers after removal, got %d", l.LayerCount())
	}

	if l.HasFile("f2.txt") {
		t.Error("f2.txt should not exist after removing its layer")
	}

	// Test RemoveLayerByRef
	l.AddLayer(Files{{Name: "dup1.txt"}}, "duplicate")
	l.AddLayer(Files{{Name: "other.txt"}}, "other")
	l.AddLayer(Files{{Name: "dup2.txt"}}, "duplicate2") // Different ref

	// Can't add duplicate ref
	err = l.AddLayer(Files{{Name: "dup3.txt"}}, "duplicate")
	if err == nil {
		t.Error("Should not allow duplicate layer reference")
	}

	removed := l.RemoveLayerByRef("duplicate")
	if removed != 1 {
		t.Errorf("Expected to remove 1 layer, removed %d", removed)
	}

	if l.HasFile("dup1.txt") {
		t.Error("dup1.txt should not exist after removing its layer")
	}

	// Test Clear
	l.Clear()
	if l.LayerCount() != 0 {
		t.Errorf("Expected 0 layers after clear, got %d", l.LayerCount())
	}
	if !l.IsEmpty() {
		t.Error("Index should be empty after clear")
	}
}

// Test FindInLayer
func TestLayeredIndex_FindInLayer(t *testing.T) {
	l := NewLayeredIndex[string]()

	l.AddLayer(Files{
		{Name: "file1.txt", CompressedSize64: 100},
		{Name: "shared.txt", CompressedSize64: 200},
	}, "layer1")

	l.AddLayer(Files{
		{Name: "file2.txt", CompressedSize64: 300},
		{Name: "shared.txt", CompressedSize64: 400}, // Override
	}, "layer2")

	// Find in specific layer
	file, found := l.FindInLayer("shared.txt", "layer1")
	if !found {
		t.Error("shared.txt should exist in layer1")
	}
	if file.CompressedSize64 != 200 {
		t.Errorf("Expected size 200 in layer1, got %d", file.CompressedSize64)
	}

	file, found = l.FindInLayer("shared.txt", "layer2")
	if !found {
		t.Error("shared.txt should exist in layer2")
	}
	if file.CompressedSize64 != 400 {
		t.Errorf("Expected size 400 in layer2, got %d", file.CompressedSize64)
	}

	// File only in one layer
	_, found = l.FindInLayer("file1.txt", "layer2")
	if found {
		t.Error("file1.txt should not exist in layer2")
	}

	// Non-existent layer
	_, found = l.FindInLayer("file1.txt", "nonexistent")
	if found {
		t.Error("Should not find file in non-existent layer")
	}
}

// Test ToSingleIndex
func TestLayeredIndex_ToSingleIndex(t *testing.T) {
	l := NewLayeredIndex[int]()

	l.AddLayer(Files{
		{Name: "a.txt", CompressedSize64: 1, CRC32: 1111},
		{Name: "b.txt", CompressedSize64: 2, CRC32: 2222},
	}, 1)

	l.AddLayer(Files{
		{Name: "b.txt", CompressedSize64: 22, CRC32: 2223}, // Override
		{Name: "c.txt", CompressedSize64: 3, CRC32: 3333},
	}, 2)

	l.AddDeleteLayer(Files{{Name: "a.txt"}}, 3)

	single := l.ToSingleIndex()

	// Should have b.txt (overridden) and c.txt, but not a.txt (deleted)
	if len(single) != 2 {
		t.Errorf("Expected 2 files in single index, got %d", len(single))
	}

	// Check the files
	fileMap := make(map[string]File)
	for _, f := range single {
		fileMap[f.Name] = f
	}

	if _, exists := fileMap["a.txt"]; exists {
		t.Error("a.txt should not exist in merged index")
	}

	if b, exists := fileMap["b.txt"]; !exists {
		t.Error("b.txt should exist in merged index")
	} else if b.CRC32 != 2223 {
		t.Errorf("b.txt should have overridden CRC 2223, got %d", b.CRC32)
	}

	if _, exists := fileMap["c.txt"]; !exists {
		t.Error("c.txt should exist in merged index")
	}
}

// Test edge cases
func TestLayeredIndex_EdgeCases(t *testing.T) {
	t.Run("EmptyLayers", func(t *testing.T) {
		l := NewLayeredIndex[int]()

		// Add empty layer
		err := l.AddLayer(Files{}, 1)
		if err != nil {
			t.Errorf("Should allow empty layer: %v", err)
		}

		if !l.IsEmpty() {
			t.Error("Index should still be empty with empty layer")
		}

		// Add empty delete layer
		err = l.AddDeleteLayer(Files{}, 2)
		if err != nil {
			t.Errorf("Should allow empty delete layer: %v", err)
		}
	})

	t.Run("DeleteNonExistent", func(t *testing.T) {
		l := NewLayeredIndex[int]()

		l.AddLayer(Files{{Name: "exists.txt"}}, 1)

		// Delete non-existent file
		l.AddDeleteLayer(Files{{Name: "nonexistent.txt"}}, 2)

		// Should not affect existing files
		if !l.HasFile("exists.txt") {
			t.Error("exists.txt should still exist")
		}
		if l.FileCount() != 1 {
			t.Errorf("Expected 1 file, got %d", l.FileCount())
		}
	})

	t.Run("MultipleOverrides", func(t *testing.T) {
		l := NewLayeredIndex[int]()

		// Add same file in multiple layers
		for i := 1; i <= 5; i++ {
			l.AddLayer(Files{{
				Name:             "file.txt",
				CompressedSize64: uint64(i * 100),
			}}, i)
		}

		file, found := l.Find("file.txt")
		if !found {
			t.Error("file.txt should exist")
		}
		if file.CompressedSize64 != 500 {
			t.Errorf("Expected size 500 from last layer, got %d", file.CompressedSize64)
		}
		if file.LayerRef != 5 {
			t.Errorf("Expected layer 5, got %d", file.LayerRef)
		}
	})

	t.Run("DeleteAndReAddMultiple", func(t *testing.T) {
		l := NewLayeredIndex[string]()

		// Complex scenario with multiple add/delete cycles
		l.AddLayer(Files{{Name: "cycle.txt", CRC32: 1}}, "v1")
		l.AddDeleteLayer(Files{{Name: "cycle.txt"}}, "del1")
		l.AddLayer(Files{{Name: "cycle.txt", CRC32: 2}}, "v2")
		l.AddDeleteLayer(Files{{Name: "cycle.txt"}}, "del2")
		l.AddLayer(Files{{Name: "cycle.txt", CRC32: 3}}, "v3")

		file, found := l.Find("cycle.txt")
		if !found {
			t.Error("cycle.txt should exist after re-adding")
		}
		if file.CRC32 != 3 {
			t.Errorf("Expected CRC 3 from final add, got %d", file.CRC32)
		}
	})
}

// Test with custom comparable types
func TestLayeredIndex_CustomTypes(t *testing.T) {
	type Version struct {
		Major, Minor int
	}

	l := NewLayeredIndex[Version]()

	l.AddLayer(Files{{Name: "readme.txt"}}, Version{1, 0})
	l.AddLayer(Files{{Name: "changelog.txt"}}, Version{1, 1})
	l.AddLayer(Files{{Name: "readme.txt", CompressedSize64: 999}}, Version{2, 0})

	file, found := l.Find("readme.txt")
	if !found {
		t.Error("readme.txt should exist")
	}
	if file.LayerRef != (Version{2, 0}) {
		t.Errorf("Expected version 2.0, got %v", file.LayerRef)
	}

	ref, ok := l.GetLayerRef(0)
	if !ok || ref != (Version{1, 0}) {
		t.Errorf("Expected version 1.0 at index 0, got %v", ref)
	}
}

// Benchmark Files() method
func BenchmarkLayeredIndex_Files(b *testing.B) {
	l := NewLayeredIndex[int]()

	// Create layers with many files
	for layer := 0; layer < 10; layer++ {
		var files Files
		for i := 0; i < 1000; i++ {
			files = append(files, File{
				Name:             fmt.Sprintf("layer%02d_file_%04d.txt", layer, i),
				CompressedSize64: uint64(i),
			})
		}
		l.AddLayer(files, layer)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = l.Files()
	}
}

// Benchmark Find() method with binary search optimization
func BenchmarkLayeredIndex_Find(b *testing.B) {
	l := NewLayeredIndex[int]()

	// Create layers with many files
	var testFiles []string
	for layer := 0; layer < 10; layer++ {
		var files Files
		for i := 0; i < 1000; i++ {
			name := fmt.Sprintf("layer%02d_file_%04d.txt", layer, i)
			files = append(files, File{
				Name:             name,
				CompressedSize64: uint64(i),
			})
			if layer == 5 && i%100 == 0 {
				testFiles = append(testFiles, name)
			}
		}
		l.AddLayer(files, layer)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, name := range testFiles {
			_, _ = l.Find(name)
		}
	}
}

// Benchmark FindInLayer() method with binary search optimization
func BenchmarkLayeredIndex_FindInLayer(b *testing.B) {
	l := NewLayeredIndex[int]()

	// Create a layer with many files
	var files Files
	var testFiles []string
	for i := 0; i < 10000; i++ {
		name := fmt.Sprintf("file_%05d.txt", i)
		files = append(files, File{
			Name:             name,
			CompressedSize64: uint64(i),
		})
		if i%100 == 0 {
			testFiles = append(testFiles, name)
		}
	}
	l.AddLayer(files, 1)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, name := range testFiles {
			_, _ = l.FindInLayer(name, 1)
		}
	}
}

// Test complex directory deletion scenarios
func TestLayeredIndex_ComplexDirectoryDeletion(t *testing.T) {
	l := NewLayeredIndex[string]()

	// Create a complex directory structure
	files := Files{
		{Name: "root/"},
		{Name: "root/file1.txt"},
		{Name: "root/dir1/"},
		{Name: "root/dir1/file2.txt"},
		{Name: "root/dir1/file3.txt"},
		{Name: "root/dir1/subdir/"},
		{Name: "root/dir1/subdir/file4.txt"},
		{Name: "root/dir2/"},
		{Name: "root/dir2/file5.txt"},
		{Name: "root/dir2/subdir/"},
		{Name: "root/dir2/subdir/file6.txt"},
		{Name: "other/"},
		{Name: "other/file7.txt"},
	}
	l.AddLayer(files, "base")

	// Delete file from nested directory
	l.AddDeleteLayer(Files{{Name: "root/dir1/subdir/file4.txt"}}, "del1")

	// Verify the nested empty directory is removed
	if l.HasFile("root/dir1/subdir/file4.txt") {
		t.Error("file4.txt should be deleted")
	}
	if l.HasFile("root/dir1/subdir/") {
		t.Error("root/dir1/subdir/ should be deleted as it's empty")
	}

	// But parent directories with files should remain
	if !l.HasFile("root/dir1/") {
		t.Error("root/dir1/ should still exist")
	}
	if !l.HasFile("root/") {
		t.Error("root/ should still exist")
	}

	// Delete multiple files that empty different directories
	l.AddDeleteLayer(Files{
		{Name: "root/dir2/file5.txt"},
		{Name: "root/dir2/subdir/file6.txt"},
	}, "del2")

	// Both dir2 and its subdir should be gone
	if l.HasFile("root/dir2/") {
		t.Error("root/dir2/ should be deleted")
	}
	if l.HasFile("root/dir2/subdir/") {
		t.Error("root/dir2/subdir/ should be deleted")
	}

	// But root should still exist (has dir1 and file1)
	if !l.HasFile("root/") {
		t.Error("root/ should still exist")
	}
}

// Test that all methods handle nil/empty states correctly
func TestLayeredIndex_NilEmpty(t *testing.T) {
	l := NewLayeredIndex[string]()

	// Test all methods on empty index
	if !l.IsEmpty() {
		t.Error("Empty index should report IsEmpty")
	}

	if l.FileCount() != 0 {
		t.Error("Empty index should have 0 files")
	}

	if l.LayerCount() != 0 {
		t.Error("Empty index should have 0 layers")
	}

	files := l.Files()
	if len(files) != 0 {
		t.Error("Empty index should return empty Files slice")
	}

	single := l.ToSingleIndex()
	if len(single) != 0 {
		t.Error("Empty index should return empty single index")
	}

	_, found := l.Find("anything")
	if found {
		t.Error("Empty index should not find any file")
	}

	if l.HasFile("anything") {
		t.Error("Empty index should not have any file")
	}

	_, ok := l.GetLayerRef(0)
	if ok {
		t.Error("Empty index should not have any layer refs")
	}

	err := l.RemoveLayer(0)
	if err == nil {
		t.Error("RemoveLayer should error on empty index")
	}

	removed := l.RemoveLayerByRef("anything")
	if removed != 0 {
		t.Error("RemoveLayerByRef should remove 0 from empty index")
	}

	// Clear should work without error
	l.Clear()

	// Iterator should not yield anything
	count := 0
	for range l.FilesIter() {
		count++
	}
	if count != 0 {
		t.Error("Empty index iterator should not yield anything")
	}
}

// Test Files() returns consistent order
func TestLayeredIndex_ConsistentOrder(t *testing.T) {
	l := NewLayeredIndex[int]()

	// Add files in random order across layers
	l.AddLayer(Files{
		{Name: "zebra.txt"},
		{Name: "apple.txt"},
		{Name: "middle.txt"},
	}, 1)

	l.AddLayer(Files{
		{Name: "banana.txt"},
		{Name: "xray.txt"},
	}, 2)

	// Call Files() multiple times and verify order is consistent
	var results [][]string
	for i := 0; i < 5; i++ {
		files := l.Files()
		var names []string
		for _, f := range files {
			names = append(names, f.Name)
		}
		results = append(results, names)
	}

	// All results should be identical
	for i := 1; i < len(results); i++ {
		if !slices.Equal(results[0], results[i]) {
			t.Errorf("Files() returned different order on call %d", i+1)
		}
	}

	// Verify files are sorted
	sorted := slices.Clone(results[0])
	sort.Strings(sorted)
	if !slices.Equal(results[0], sorted) {
		t.Error("Files() should return files in sorted order")
	}
}

// Test that modifications to returned slices don't affect the index
func TestLayeredIndex_ImmutableReturns(t *testing.T) {
	l := NewLayeredIndex[string]()

	l.AddLayer(Files{
		{Name: "file1.txt", CompressedSize64: 100},
		{Name: "file2.txt", CompressedSize64: 200},
	}, "layer1")

	// Get files and modify the returned slice
	files1 := l.Files()
	originalLen := len(files1)
	files1[0].CompressedSize64 = 999
	_ = append(files1, FileWithRef[string]{
		File: File{Name: "injected.txt"},
	})

	// Get files again - should be unaffected
	files2 := l.Files()
	if len(files2) != originalLen {
		t.Error("Modifying returned slice should not affect index")
	}
	if files2[0].CompressedSize64 == 999 {
		t.Error("Modifying returned file should not affect index")
	}

	// Same for ToSingleIndex
	single1 := l.ToSingleIndex()
	single1[0].CompressedSize64 = 888
	_ = append(single1, File{Name: "injected2.txt"})

	single2 := l.ToSingleIndex()
	if len(single2) != originalLen {
		t.Error("Modifying returned single index should not affect index")
	}
	if single2[0].CompressedSize64 == 888 {
		t.Error("Modifying returned file should not affect index")
	}
}

// Test RemoveLayer with various indices
func TestLayeredIndex_RemoveLayerBounds(t *testing.T) {
	l := NewLayeredIndex[int]()

	// Add some layers
	for i := 0; i < 5; i++ {
		l.AddLayer(Files{{Name: "file" + string(rune('0'+i)) + ".txt"}}, i)
	}

	// Test negative index
	err := l.RemoveLayer(-1)
	if err == nil {
		t.Error("RemoveLayer should error on negative index")
	}

	// Test index equal to count
	err = l.RemoveLayer(5)
	if err == nil {
		t.Error("RemoveLayer should error on index >= count")
	}

	// Test index greater than count
	err = l.RemoveLayer(10)
	if err == nil {
		t.Error("RemoveLayer should error on index > count")
	}

	// Remove middle layer
	err = l.RemoveLayer(2)
	if err != nil {
		t.Errorf("Failed to remove middle layer: %v", err)
	}
	if l.LayerCount() != 4 {
		t.Errorf("Expected 4 layers after removal, got %d", l.LayerCount())
	}

	// Verify the right file was removed
	if l.HasFile("file2.txt") {
		t.Error("file2.txt should be removed")
	}

	// Remove first layer
	err = l.RemoveLayer(0)
	if err != nil {
		t.Errorf("Failed to remove first layer: %v", err)
	}

	// Remove last layer
	err = l.RemoveLayer(l.LayerCount() - 1)
	if err != nil {
		t.Errorf("Failed to remove last layer: %v", err)
	}
}

// Test with large number of layers
func TestLayeredIndex_ManyLayers(t *testing.T) {
	l := NewLayeredIndex[int]()

	const numLayers = 100
	for i := 0; i < numLayers; i++ {
		files := Files{
			{Name: "common.txt", CompressedSize64: uint64(i)}, // Will be overridden
			{Name: "layer_" + string(rune(i)) + ".txt"},       // Unique to layer
		}
		err := l.AddLayer(files, i)
		if err != nil {
			t.Fatalf("Failed to add layer %d: %v", i, err)
		}
	}

	if l.LayerCount() != numLayers {
		t.Errorf("Expected %d layers, got %d", numLayers, l.LayerCount())
	}

	// common.txt should be from the last layer
	file, found := l.Find("common.txt")
	if !found {
		t.Error("common.txt should exist")
	}
	if file.LayerRef != numLayers-1 {
		t.Errorf("common.txt should be from layer %d, got %d", numLayers-1, file.LayerRef)
	}
	if file.CompressedSize64 != uint64(numLayers-1) {
		t.Errorf("common.txt should have size %d, got %d", numLayers-1, file.CompressedSize64)
	}
}

// Test that Find and Files agree
func TestLayeredIndex_FindFilesConsistency(t *testing.T) {
	l := NewLayeredIndex[string]()

	l.AddLayer(Files{
		{Name: "a.txt", CRC32: 1},
		{Name: "b.txt", CRC32: 2},
	}, "v1")

	l.AddLayer(Files{
		{Name: "b.txt", CRC32: 22},
		{Name: "c.txt", CRC32: 3},
	}, "v2")

	l.AddDeleteLayer(Files{{Name: "a.txt"}}, "del")

	l.AddLayer(Files{
		{Name: "d.txt", CRC32: 4},
	}, "v3")

	// Get all files
	allFiles := l.Files()

	// For each file in Files(), Find() should return the same data
	for _, fileWithRef := range allFiles {
		found, ok := l.Find(fileWithRef.Name)
		if !ok {
			t.Errorf("Find() failed to find %s that was in Files()", fileWithRef.Name)
			continue
		}

		if found.LayerRef != fileWithRef.LayerRef {
			t.Errorf("File %s: Find() returned layer %v but Files() returned %v",
				fileWithRef.Name, found.LayerRef, fileWithRef.LayerRef)
		}

		if !reflect.DeepEqual(found.File, fileWithRef.File) {
			t.Errorf("File %s: Find() and Files() returned different File data", fileWithRef.Name)
		}
	}

	// Also verify that Find() doesn't find files not in Files()
	deletedFile, found := l.Find("a.txt")
	if found {
		t.Errorf("Find() found deleted file a.txt: %+v", deletedFile)
	}

	// Check this is consistent with HasFile
	for _, fileWithRef := range allFiles {
		if !l.HasFile(fileWithRef.Name) {
			t.Errorf("HasFile() returned false for %s that was in Files()", fileWithRef.Name)
		}
	}

	if l.HasFile("a.txt") {
		t.Error("HasFile() returned true for deleted file a.txt")
	}
}

// Test serialization with string references
func TestLayeredIndex_SerializationString(t *testing.T) {
	// Create serializer for string references
	stringSerializer := RefSerializer[string]{
		Marshal: func(s string) ([]byte, error) {
			return []byte(s), nil
		},
		Unmarshal: func(b []byte) (string, error) {
			return string(b), nil
		},
	}

	// Create a layered index
	l := NewLayeredIndex[string]()

	// Add various layers
	l.AddLayer(Files{
		{Name: "file1.txt", CompressedSize64: 100, CRC32: 1111},
		{Name: "file2.txt", CompressedSize64: 200, CRC32: 2222},
	}, "base")

	l.AddLayer(Files{
		{Name: "file2.txt", CompressedSize64: 250, CRC32: 2223}, // Override
		{Name: "file3.txt", CompressedSize64: 300, CRC32: 3333},
	}, "update1")

	l.AddDeleteLayer(Files{
		{Name: "file1.txt"},
	}, "delete1")

	l.AddLayer(Files{
		{Name: "file4.txt", CompressedSize64: 400, CRC32: 4444},
		{Name: "file1.txt", CompressedSize64: 150, CRC32: 1112}, // Re-add
	}, "update2")

	// Serialize
	data, err := l.SerializeLayered(stringSerializer)
	if err != nil {
		t.Fatalf("Failed to serialize: %v", err)
	}

	// Deserialize
	l2, err := DeserializeLayered(data, stringSerializer)
	if err != nil {
		t.Fatalf("Failed to deserialize: %v", err)
	}

	// Verify layer count
	if l2.LayerCount() != l.LayerCount() {
		t.Errorf("Layer count mismatch: got %d, want %d", l2.LayerCount(), l.LayerCount())
	}

	// Verify layer references
	for i := 0; i < l.LayerCount(); i++ {
		ref1, _ := l.GetLayerRef(i)
		ref2, _ := l2.GetLayerRef(i)
		if ref1 != ref2 {
			t.Errorf("Layer %d ref mismatch: got %s, want %s", i, ref2, ref1)
		}
	}

	// Verify files
	files1 := l.Files()
	files2 := l2.Files()

	if len(files1) != len(files2) {
		t.Errorf("File count mismatch: got %d, want %d", len(files2), len(files1))
	}

	for i := range files1 {
		if files1[i].Name != files2[i].Name {
			t.Errorf("File %d name mismatch: got %s, want %s", i, files2[i].Name, files1[i].Name)
		}
		if files1[i].CompressedSize64 != files2[i].CompressedSize64 {
			t.Errorf("File %d size mismatch: got %d, want %d", i, files2[i].CompressedSize64, files1[i].CompressedSize64)
		}
		if files1[i].CRC32 != files2[i].CRC32 {
			t.Errorf("File %d CRC mismatch: got %d, want %d", i, files2[i].CRC32, files1[i].CRC32)
		}
		if files1[i].LayerRef != files2[i].LayerRef {
			t.Errorf("File %d layer ref mismatch: got %s, want %s", i, files2[i].LayerRef, files1[i].LayerRef)
		}
	}
}

// Test serialization with integer references
func TestLayeredIndex_SerializationInt(t *testing.T) {
	// Create serializer for int references
	intSerializer := RefSerializer[int]{
		Marshal: func(i int) ([]byte, error) {
			buf := make([]byte, 8)
			binary.BigEndian.PutUint64(buf, uint64(i))
			return buf, nil
		},
		Unmarshal: func(b []byte) (int, error) {
			if len(b) != 8 {
				return 0, fmt.Errorf("invalid int data: expected 8 bytes, got %d", len(b))
			}
			return int(binary.BigEndian.Uint64(b)), nil
		},
	}

	// Create a layered index
	l := NewLayeredIndex[int]()

	// Add layers with int references
	for i := 0; i < 5; i++ {
		var files Files
		for j := 0; j < 10; j++ {
			files = append(files, File{
				Name:             fmt.Sprintf("layer%d_file%d.txt", i, j),
				CompressedSize64: uint64(i*100 + j),
				CRC32:            uint32(i*1000 + j),
			})
		}
		if i == 3 {
			// Make layer 3 a delete layer
			l.AddDeleteLayer(Files{{Name: "layer0_file5.txt"}}, i)
		} else {
			l.AddLayer(files, i)
		}
	}

	// Serialize
	data, err := l.SerializeLayered(intSerializer)
	if err != nil {
		t.Fatalf("Failed to serialize: %v", err)
	}

	// Deserialize
	l2, err := DeserializeLayered(data, intSerializer)
	if err != nil {
		t.Fatalf("Failed to deserialize: %v", err)
	}

	// Verify
	if l2.LayerCount() != l.LayerCount() {
		t.Errorf("Layer count mismatch: got %d, want %d", l2.LayerCount(), l.LayerCount())
	}

	// Check specific file
	file, found := l2.Find("layer0_file5.txt")
	origFile, origFound := l.Find("layer0_file5.txt")
	if found != origFound {
		t.Errorf("File existence mismatch for layer0_file5.txt")
	}
	if found && file.LayerRef != origFile.LayerRef {
		t.Errorf("Layer ref mismatch for layer0_file5.txt")
	}
}

// Test serialization with custom struct references
func TestLayeredIndex_SerializationCustomType(t *testing.T) {
	type Version struct {
		Major int
		Minor int
		Patch int
	}

	// Create serializer for Version
	versionSerializer := RefSerializer[Version]{
		Marshal: func(v Version) ([]byte, error) {
			buf := make([]byte, 12)
			binary.BigEndian.PutUint32(buf[0:4], uint32(v.Major))
			binary.BigEndian.PutUint32(buf[4:8], uint32(v.Minor))
			binary.BigEndian.PutUint32(buf[8:12], uint32(v.Patch))
			return buf, nil
		},
		Unmarshal: func(b []byte) (Version, error) {
			if len(b) != 12 {
				return Version{}, fmt.Errorf("invalid version data: expected 12 bytes, got %d", len(b))
			}
			return Version{
				Major: int(binary.BigEndian.Uint32(b[0:4])),
				Minor: int(binary.BigEndian.Uint32(b[4:8])),
				Patch: int(binary.BigEndian.Uint32(b[8:12])),
			}, nil
		},
	}

	// Create layered index
	l := NewLayeredIndex[Version]()

	l.AddLayer(Files{
		{Name: "main.go", CompressedSize64: 1000},
		{Name: "go.mod", CompressedSize64: 100},
	}, Version{1, 0, 0})

	l.AddLayer(Files{
		{Name: "main.go", CompressedSize64: 1200}, // Updated
		{Name: "feature.go", CompressedSize64: 500},
	}, Version{1, 1, 0})

	l.AddDeleteLayer(Files{
		{Name: "feature.go"}, // Removed in patch
	}, Version{1, 1, 1})

	// Serialize and deserialize
	data, err := l.SerializeLayered(versionSerializer)
	if err != nil {
		t.Fatalf("Failed to serialize: %v", err)
	}

	l2, err := DeserializeLayered(data, versionSerializer)
	if err != nil {
		t.Fatalf("Failed to deserialize: %v", err)
	}

	// Verify versions
	for i := 0; i < l.LayerCount(); i++ {
		v1, _ := l.GetLayerRef(i)
		v2, _ := l2.GetLayerRef(i)
		if v1 != v2 {
			t.Errorf("Version mismatch at layer %d: got %+v, want %+v", i, v2, v1)
		}
	}

	// Check files
	if l2.HasFile("feature.go") {
		t.Error("feature.go should have been deleted")
	}

	file, found := l2.Find("main.go")
	if !found {
		t.Error("main.go should exist")
	} else if file.LayerRef != (Version{1, 1, 0}) {
		t.Errorf("main.go should be from version 1.1.0, got %+v", file.LayerRef)
	}
}

// Test serialization error handling
func TestLayeredIndex_SerializationErrors(t *testing.T) {
	l := NewLayeredIndex[string]()
	l.AddLayer(Files{{Name: "test.txt"}}, "test")

	// Test with nil Marshal function
	nilMarshal := RefSerializer[string]{
		Marshal:   nil,
		Unmarshal: func(b []byte) (string, error) { return "", nil },
	}

	_, err := l.SerializeLayered(nilMarshal)
	if err == nil {
		t.Error("Should error with nil Marshal function")
	}

	// Test with nil Unmarshal function
	nilUnmarshal := RefSerializer[string]{
		Marshal:   func(s string) ([]byte, error) { return []byte(s), nil },
		Unmarshal: nil,
	}

	data, _ := l.SerializeLayered(RefSerializer[string]{
		Marshal:   func(s string) ([]byte, error) { return []byte(s), nil },
		Unmarshal: func(b []byte) (string, error) { return string(b), nil },
	})

	_, err = DeserializeLayered(data, nilUnmarshal)
	if err == nil {
		t.Error("Should error with nil Unmarshal function")
	}

	// Test with Marshal error
	errorMarshal := RefSerializer[string]{
		Marshal: func(s string) ([]byte, error) {
			return nil, fmt.Errorf("marshal error")
		},
		Unmarshal: func(b []byte) (string, error) { return string(b), nil },
	}

	_, err = l.SerializeLayered(errorMarshal)
	if err == nil {
		t.Error("Should propagate Marshal error")
	}

	// Test with Unmarshal error
	errorUnmarshal := RefSerializer[string]{
		Marshal: func(s string) ([]byte, error) { return []byte(s), nil },
		Unmarshal: func(b []byte) (string, error) {
			return "", fmt.Errorf("unmarshal error")
		},
	}

	_, err = DeserializeLayered(data, errorUnmarshal)
	if err == nil {
		t.Error("Should propagate Unmarshal error")
	}
}

// Test concurrent serialization performance
func TestLayeredIndex_ConcurrentSerialization(t *testing.T) {
	// Create a large layered index
	l := NewLayeredIndex[int]()

	// Add many layers
	for layer := 0; layer < 20; layer++ {
		var files Files
		for i := 0; i < 500; i++ {
			files = append(files, File{
				Name:             fmt.Sprintf("layer%02d/file%04d.txt", layer, i),
				CompressedSize64: uint64(layer*1000 + i),
				CRC32:            uint32(layer*10000 + i),
				Custom:           map[string]string{"layer": fmt.Sprintf("%d", layer)},
			})
		}
		l.AddLayer(files, layer)
	}

	intSerializer := RefSerializer[int]{
		Marshal: func(i int) ([]byte, error) {
			buf := make([]byte, 8)
			binary.BigEndian.PutUint64(buf, uint64(i))
			return buf, nil
		},
		Unmarshal: func(b []byte) (int, error) {
			return int(binary.BigEndian.Uint64(b)), nil
		},
	}

	// Serialize
	data, err := l.SerializeLayered(intSerializer)
	if err != nil {
		t.Fatalf("Failed to serialize large index: %v", err)
	}

	// Deserialize
	l2, err := DeserializeLayered(data, intSerializer)
	if err != nil {
		t.Fatalf("Failed to deserialize large index: %v", err)
	}

	// Verify counts
	if l2.LayerCount() != 20 {
		t.Errorf("Expected 20 layers, got %d", l2.LayerCount())
	}

	if l2.FileCount() != 20*500 {
		t.Errorf("Expected %d files, got %d", 20*500, l2.FileCount())
	}

	// Spot check some files
	file, found := l2.Find("layer10/file0250.txt")
	if !found {
		t.Error("Should find layer10/file0250.txt")
	} else {
		if file.LayerRef != 10 {
			t.Errorf("Wrong layer ref: got %d, want 10", file.LayerRef)
		}
		if file.CompressedSize64 != 10250 {
			t.Errorf("Wrong size: got %d, want 10250", file.CompressedSize64)
		}
	}
}

// Benchmark serialization
func BenchmarkLayeredIndex_Serialization(b *testing.B) {
	// Create a layered index
	l := NewLayeredIndex[string]()

	for layer := 0; layer < 10; layer++ {
		var files Files
		for i := 0; i < 1000; i++ {
			files = append(files, File{
				Name:             fmt.Sprintf("layer%02d_file_%04d.txt", layer, i),
				CompressedSize64: uint64(i),
			})
		}
		l.AddLayer(files, fmt.Sprintf("layer%d", layer))
	}

	stringSerializer := RefSerializer[string]{
		Marshal: func(s string) ([]byte, error) {
			return []byte(s), nil
		},
		Unmarshal: func(b []byte) (string, error) {
			return string(b), nil
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		data, err := l.SerializeLayered(stringSerializer)
		if err != nil {
			b.Fatal(err)
		}
		_ = data
	}
}

// Benchmark deserialization
func BenchmarkLayeredIndex_Deserialization(b *testing.B) {
	// Create and serialize a layered index
	l := NewLayeredIndex[string]()

	for layer := 0; layer < 10; layer++ {
		var files Files
		for i := 0; i < 1000; i++ {
			files = append(files, File{
				Name:             fmt.Sprintf("layer%02d_file_%04d.txt", layer, i),
				CompressedSize64: uint64(i),
			})
		}
		l.AddLayer(files, fmt.Sprintf("layer%d", layer))
	}

	stringSerializer := RefSerializer[string]{
		Marshal: func(s string) ([]byte, error) {
			return []byte(s), nil
		},
		Unmarshal: func(b []byte) (string, error) {
			return string(b), nil
		},
	}

	data, _ := l.SerializeLayered(stringSerializer)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		l2, err := DeserializeLayered(data, stringSerializer)
		if err != nil {
			b.Fatal(err)
		}
		_ = l2
	}
}
