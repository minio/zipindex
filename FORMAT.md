# Format Specification

v1.0.0 (August 2022): Initial version (Type 1 + 2)
v1.1.0: Added Type 3 (columnar format)
v1.2.0: Added Type 4 (chunked columnar format)
v1.3.0: Added Layered Index format

The format consists of a single byte indicating the following data.

## Type 1 + 2 

If version is 2, payload is compressed. 
The rest of the payload must be decompressed using Zstandard. 
A maximum window size of 8MB is allowed.

Type 1+2 contain an array of entries, each with 8 attributes. `[entries][8]attributes`.

Arrays are encoded as regular messagepack arrays.

No more than 100 entries are allowed as type 1+2. The uncompressed serialized data *must* be less than 128MB.

Entries are stored as messagepack types. The values must fit within the bits specified.

| Name              | Index | Type           | Description                                    |
|-------------------|-------|----------------|------------------------------------------------|
| Name              | 0     | string         | Name of the file as stored in the zip          |
| Compressed Size   | 1     | Uint (64 bits) | Size of compressed data, excluding ZIP headers |
| Uncompressed Size | 2     | Uint (64 bits) | Size of the Uncompressed data                  |
| Offset            | 3     | Int (64 bits)  | Offset where file data header starts.          |
| CRC32             | 4     | Uint (32 bits) | CRC of the uncompressed data                   |
| Method            | 5     | Uint (16 bits) | Storage method                                 |
| Flags             | 6     | Uint (16 bits) | General purpose bit flag                       |
| Custom            | 7     | Map (string)   | Custom data (max 1000 entries)                 |


Values can be used with no further modification.

If the 'Data Descriptor' Flag (bit 3) has been set, the CRC *may* have been set to zero,
since this indicates that the CRC is available as part of the in-stream data descriptor.

## Type 3

This type is always compressed.
The rest of the payload must be decompressed using Zstandard. A maximum window size of 8MB is allowed.

Files are stored as an array of arrays. The arrays size of `Names` indicates the number of entries.

The uncompressed serialized data *must* be less than 128MB. A separate file-count cap of 1 billion entries exists, but the 128MB size limit will be the binding constraint in practice.

All arrays must have the same number of entries. CRCs must have `entries*4` bytes.

| Name              | Index | Type             | Description                                    |
|-------------------|-------|------------------|------------------------------------------------|
| Names             | 0     | []bin            | Name of the file as stored in the zip          |
| Compressed Sizes  | 1     | []Int (64 bits)  | Size of compressed data, excluding ZIP headers |
| Uncompressed Size | 2     | []Int (64 bits)  | Size of the Uncompressed data                  |
| Offsets           | 3     | []Int (64 bits)  | Offset where file data header starts.          |
| Methods           | 4     | []Uint (16 bits) | Storage method                                 |
| Flags             | 5     | []Uint (16 bits) | General purpose bit flag                       |
| CRCs              | 6     | bin              | Binary array of CRCs                           |
| Custom            | 7     | []bin            | Custom data                                    |

It is recommended, but not required to store entries sorted by Offset.

### Names

Names are stored as binary blobs, but contains utf8 strings.

### Compressed Sizes

Compressed sizes contain a delta to the compressed size of the previous file. 
Initial size is assumed to be 0.

Compressed sizes should therefore be accumulated as the file is read.

```
    if i > 0 {
        CompressedSize[i] = CompressedSize[i-1] + CompressedSize[i]
    } 
```

### Uncompressed Sizes

Uncompressed sizes contains the difference to the compressed sizes.

Uncompressed sizes must have the compressed size (after adjustment above) added.

```
    UncompressedSize[i] = CompressedSize[i] + UncompressedSize[i]
```

### Offsets

Offsets are stored as deltas - constant from last file offset plus last file compressed size and name length, 
except the first offset which can be used as is.

```
    if i > 0 {
        Offsets[i] = Offsets[i] + Offsets[i-1] + CompressedSize[i-1] + (len(Names[i-1])) + 46
    }
```

The name length is the *binary length* of the name, not the character count.

### Methods, Flags

Methods and flags are stored as XOR result with previous value. Initial value is 0.

```
    if i > 0 {
	    Methods[i] ^= Methods[i-1] 
	    Flags[i] ^= Flags[i-1] 
    }
```

This means that only differences in these values are stored.

### CRCs

CRCs are stored as a single array of bytes. Each CRC is 4 bytes, little-endian.

If the 'Data Descriptor' Flag (bit 3) has been set, the CRC *may* have been set to zero, 
since this indicates that the CRC is available as part of the in-stream data descriptor.

### Custom data

Custom data is a blob of encoded `map string -> string` key, values, also encoded as messagepack.

A length 0 blob will be stored if no custom data is present.

There is a maximum of 1000 entries allowed per file.

## Type 4

Type 4 is a chunked version of Type 3 for large directories.

The version byte is `4`. The remainder of the payload is **not** compressed at the top level.
Instead, it consists of concatenated messagepack-encoded chunks.

Each chunk is a messagepack **map** with 4 fields:

| Name    | Type   | Description                                           |
|---------|--------|-------------------------------------------------------|
| Files   | Int    | Number of files in this chunk                         |
| First   | string | Name of the first file (lexicographic, for searching) |
| Last    | string | Name of the last file (lexicographic, for searching)  |
| Payload | bin    | Self-contained Type 3 blob (version byte + zstd data) |

Chunks are ordered by name. Within each chunk, entries are sorted by offset.

Each chunk's `Payload` is a complete Type 3 serialization: a `0x03` version byte followed by
Zstandard-compressed `filesAsStructs` data. Chunks can be decompressed independently.

The recommended chunk size is approximately 25,000 entries. If the remaining entries would
produce a chunk between 25,000 and 50,000, it should be split in half.

Each chunk is subject to the same 128MB uncompressed limit as Type 3. 
The 1 billion file-count cap applies across all chunks.

## Layered Index

A layered index wraps multiple file indexes with override/delete semantics.
All values are encoded as messagepack types.

### Header

| Name       | Type   | Description              |
|------------|--------|--------------------------|
| Version    | Uint8  | Currently `1`            |
| NumLayers  | Uint32 | Number of layers         |

### Layers

Layers are concatenated immediately after the header, oldest first.
Each layer consists of three consecutive messagepack values:

| Name      | Type | Description                                         |
|-----------|------|-----------------------------------------------------|
| Ref       | bin  | Opaque reference blob (application-defined)         |
| IsDelete  | bool | If true, files in this layer are deletions          |
| FilesData | bin  | Serialized file index (Type 1/2/3/4 payload)        |

Newer layers override older layers for files with the same name.
A delete layer removes matching files from all earlier layers.
Files within each layer are sorted by name.

# Future extensions

Additional types may be added in the future,
in particular a fully streaming index may be added if the need arises.
