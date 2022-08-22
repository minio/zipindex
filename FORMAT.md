# Format Specification

v1.0.0 (August 2022): Initial version

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

Type 3 supports up to 100 million entries. The uncompressed serialized data *must* be less than 128MB. 

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
    if i > 0 {
        UncompressedSize[i] = CompressedSize[i] + UncompressedSize[i]
    } 
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

CRCs are stored as a single array of bytes. Each CRC is 4 bytes.

If the 'Data Descriptor' Flag (bit 3) has been set, the CRC *may* have been set to zero, 
since this indicates that the CRC is available as part of the in-stream data descriptor.

### Custom data

Custom data is a blob of encoded `map string -> string` key, values, also encoded as messagepack.

A length 0 blob will be stored if no custom data is present.

There is a maximum of 1000 entries allowed per file.

# Future extensions

Additional types may be added in the future, 
in particular a fully streaming index may be added if the need arises.
