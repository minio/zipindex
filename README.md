# zipindex

[![Go Reference](https://pkg.go.dev/badge/minio/zipindex.svg)](https://pkg.go.dev/github.com/minio/zipindex)
[![Go](https://github.com/minio/zipindex/actions/workflows/go.yml/badge.svg)](https://github.com/minio/zipindex/actions/workflows/go.yml)

`zipindex` provides a size optimized representation of a zip file directory to allow
decompressing files inside a ZIP file without reading the file index every file.

It will only provide the minimal needed data for successful decompression and CRC checks.

Custom metadata can be stored per file and filtering can be performed on the incoming files.

Currently, up to 100 million files per zip file is supported.
If a streaming format is added, this limit may be lifted.

## Usage

### Indexing

Indexing is performed on the last part of a complete ZIP file.

Three methods can be used:

The `zipindex.ReadDir` function allows parsing from a raw buffer from the end of the file. 
If this isn't enough to read the directory `zipindex.ErrNeedMoreData` is returned, which will
return how much data is needed to read the directory.

Alternatively, `zipindex.ReadFile` will open a file on disk and read the directory from that.

Finally `zipindex.ReaderAt` allows to read the index from anything supporting the 
`io.ReaderAt` interface. 

By default, only "regular" files are indexed, meaning directories and other entries are skipped,
as well as files for which a decompressor isn't registered.

A custom filter function can be provided to change the default filtering.
This also allows adding custom data for each file if more information is needed.

See examples in the [documentation](https://pkg.go.dev/github.com/minio/zipindex)

### Serializing

Before serializing it is recommended to run the `OptimizeSize()` on the returned files.
This will sort the entries and remove any redundant CRC information. 

The files are serialized using the `Serialize()` method.
This will allow the information to be recreated using `zipindex.DeserializeFiles`,
or to find a single file `zipindex.FindSerialized` can be used.

See examples in the [documentation](https://pkg.go.dev/github.com/minio/zipindex)

## Accessing File Content

To read a file, you will need 1) the serialized index and once you have gotten the information
for the file you want to decompress form the ZIP file, you will need to forward the zip file to the offset
specified by the information returned from the index.

A file contains the following information:

```Go
type File struct {
    Name               string // Name of the file as stored in the zip.
    CompressedSize64   uint64 // Size of compressed data, excluding ZIP headers.
    UncompressedSize64 uint64 // Size of the Uncompressed data.
    Offset             int64  // Offset where file data header starts.
    CRC32              uint32 // CRC of the uncompressed data.
    Method             uint16 // Storage method.
    Flags              uint16 // General purpose bit flag

    Custom map[string]string
}
```

First an `io.Reader` *must* be forwarded to the absolute offset in `Offset`. 
It is up to the caller to decide how to achieve that.

To open an individual file from the index use the `(*File).Open(r io.Reader)` with the 
forwarded Reader to open the content.

Similar to [stdlib zip](https://golang.org/pkg/archive/zip/), not all methods/flags may be supported. 

For expert users, `(*File).OpenRaw` allows access to the compressed data.


## Compression Methods

By default, zipindex keeps files stored uncompressed or deflate compressed.
This covers the most commonly seen compression methods.

Furthermore, files compressed with [zstandard](https://facebook.github.io/zstd/)
as method 93 will be preserved and can be read back.

Use [`RegisterDecompressor`](https://pkg.go.dev/github.com/minio/zipindex#RegisterDecompressor)
to register non-standard decompressors.

# Layered Indexes

The `LayeredIndex[T]` type provides a way to combine multiple zip indexes into a single searchable entity. This is useful when you need to overlay multiple archives or apply incremental updates without rebuilding the entire index.

## Key Features

- **Generic type parameter**: Each layer is associated with a comparable reference type `T` (e.g., version number, timestamp, file path)
- **Override semantics**: Files in newer layers override files with the same path in older layers
- **Delete layers**: Special layers that remove files from previous layers
- **Efficient lookups**: Find files across all layers with proper precedence

## Basic Usage

```go
// Create a new layered index with string references
layered := zipindex.NewLayeredIndex[string]()

// Add base layer
baseFiles, _ := zipindex.ReadFile("base.zip")
err := layered.AddLayer(baseFiles, "v1.0")

// Add update layer (overrides files from base)
updateFiles, _ := zipindex.ReadFile("update.zip")
err = layered.AddLayer(updateFiles, "v1.1")

// Add a delete layer (removes specified files)
deleteFiles := zipindex.Files{{Name: "obsolete.txt"}}
err = layered.AddDeleteLayer(deleteFiles, "cleanup")

// Find a file across all layers
file, found := layered.Find("readme.txt")
if found {
    // file.File contains the file info
    // file.LayerRef contains the layer reference (e.g., "v1.1")
}

// Iterate over all files
for ref, file := range layered.FilesIter() {
    fmt.Printf("File %s from layer %v\n", file.Name, ref)
}

// Merge all layers into a single index, this will lose the reference information
merged := layered.ToSingleIndex()
serialized, _ := merged.Serialize()
```

## API Reference

### Creation and Layer Management
- `NewLayeredIndex[T]()` - Create a new empty layered index
- `AddLayer(files, ref)` - Add a layer (returns error if ref already exists)
- `AddDeleteLayer(files, ref)` - Add a delete layer to remove files from previous layers
- `RemoveLayer(index)` - Remove layer by index
- `RemoveLayerByRef(ref)` - Remove all layers with the given reference
- `Clear()` - Remove all layers

### File Access
- `Find(name)` - Find a file across all layers, returns `(*FileWithRef[T], bool)`
- `FindInLayer(name, ref)` - Find a file in a specific layer only
- `FilesIter()` - Iterator that yields `(T, File)` pairs on merged indexes
- `Files()` - Get all files as `[]FileWithRef[T]` after applying layer operations
- `HasFile(name)` - Check if a file exists

### Layer Information
- `LayerCount()` - Number of layers
- `GetLayerRef(index)` - Get reference for a layer
- `FileCount()` - Total unique files after applying operations
- `IsEmpty()` - True if no files remain after applying all operations

### Conversion
- `ToSingleIndex()` - Merge all layers into a single `Files` collection

### Serialization
- `SerializeLayered(RefSerializer[T])` - Serialize the layered index preserving all layers
- `DeserializeLayered[T](data, RefSerializer[T])` - Reconstruct a layered index from serialized data

The serialization requires providing a `RefSerializer[T]` with functions to convert your reference type to/from bytes:

```go
// Example for string references
stringSerializer := RefSerializer[string]{
    Marshal: func(s string) ([]byte, error) {
        return []byte(s), nil
    },
    Unmarshal: func(b []byte) (string, error) {
        return string(b), nil
    },
}

// Serialize
data, err := layered.SerializeLayered(stringSerializer)

// Deserialize
layered2, err := DeserializeLayered(data, stringSerializer)
```

### Important Notes

1. **Deletion semantics**: Delete layers only remove files that exist in *previous* layers. Files added in subsequent layers are not affected.

2. **Directory handling**: When a file is deleted, empty parent directories are automatically removed. A directory is kept if it still contains any files.

3. **Duplicate references**: The same reference cannot be used twice. Attempting to add a layer with an existing reference returns an error.

4. **Performance**: The layered index maintains files in memory. For large numbers of layers or files, consider merging to a single index periodically.

## License

zipindex is released under the Apache License v2.0. You can find the complete text in the file LICENSE.

zipindex contains code that is Copyright (c) 2009 The Go Authors. See `GO_LICENSE` file for license.

## Contributing

Contributions are welcome, please send PRs for any enhancements.
