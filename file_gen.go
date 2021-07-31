package zipindex

// Code generated by github.com/tinylib/msgp DO NOT EDIT.

import (
	"github.com/tinylib/msgp/msgp"
)

// DecodeMsg implements msgp.Decodable
func (z *File) DecodeMsg(dc *msgp.Reader) (err error) {
	var zb0001 uint32
	zb0001, err = dc.ReadArrayHeader()
	if err != nil {
		err = msgp.WrapError(err)
		return
	}
	if zb0001 != 8 {
		err = msgp.ArrayError{Wanted: 8, Got: zb0001}
		return
	}
	z.Name, err = dc.ReadString()
	if err != nil {
		err = msgp.WrapError(err, "Name")
		return
	}
	z.CompressedSize64, err = dc.ReadUint64()
	if err != nil {
		err = msgp.WrapError(err, "CompressedSize64")
		return
	}
	z.UncompressedSize64, err = dc.ReadUint64()
	if err != nil {
		err = msgp.WrapError(err, "UncompressedSize64")
		return
	}
	z.Offset, err = dc.ReadInt64()
	if err != nil {
		err = msgp.WrapError(err, "Offset")
		return
	}
	z.CRC32, err = dc.ReadUint32()
	if err != nil {
		err = msgp.WrapError(err, "CRC32")
		return
	}
	z.Method, err = dc.ReadUint16()
	if err != nil {
		err = msgp.WrapError(err, "Method")
		return
	}
	z.Flags, err = dc.ReadUint16()
	if err != nil {
		err = msgp.WrapError(err, "Flags")
		return
	}
	var zb0002 uint32
	zb0002, err = dc.ReadMapHeader()
	if err != nil {
		err = msgp.WrapError(err, "Custom")
		return
	}
	if z.Custom == nil {
		z.Custom = make(map[string]string, zb0002)
	} else if len(z.Custom) > 0 {
		for key := range z.Custom {
			delete(z.Custom, key)
		}
	}
	for zb0002 > 0 {
		zb0002--
		var za0001 string
		var za0002 string
		za0001, err = dc.ReadString()
		if err != nil {
			err = msgp.WrapError(err, "Custom")
			return
		}
		za0002, err = dc.ReadString()
		if err != nil {
			err = msgp.WrapError(err, "Custom", za0001)
			return
		}
		z.Custom[za0001] = za0002
	}
	return
}

// EncodeMsg implements msgp.Encodable
func (z *File) EncodeMsg(en *msgp.Writer) (err error) {
	// array header, size 8
	err = en.Append(0x98)
	if err != nil {
		return
	}
	err = en.WriteString(z.Name)
	if err != nil {
		err = msgp.WrapError(err, "Name")
		return
	}
	err = en.WriteUint64(z.CompressedSize64)
	if err != nil {
		err = msgp.WrapError(err, "CompressedSize64")
		return
	}
	err = en.WriteUint64(z.UncompressedSize64)
	if err != nil {
		err = msgp.WrapError(err, "UncompressedSize64")
		return
	}
	err = en.WriteInt64(z.Offset)
	if err != nil {
		err = msgp.WrapError(err, "Offset")
		return
	}
	err = en.WriteUint32(z.CRC32)
	if err != nil {
		err = msgp.WrapError(err, "CRC32")
		return
	}
	err = en.WriteUint16(z.Method)
	if err != nil {
		err = msgp.WrapError(err, "Method")
		return
	}
	err = en.WriteUint16(z.Flags)
	if err != nil {
		err = msgp.WrapError(err, "Flags")
		return
	}
	err = en.WriteMapHeader(uint32(len(z.Custom)))
	if err != nil {
		err = msgp.WrapError(err, "Custom")
		return
	}
	for za0001, za0002 := range z.Custom {
		err = en.WriteString(za0001)
		if err != nil {
			err = msgp.WrapError(err, "Custom")
			return
		}
		err = en.WriteString(za0002)
		if err != nil {
			err = msgp.WrapError(err, "Custom", za0001)
			return
		}
	}
	return
}

// MarshalMsg implements msgp.Marshaler
func (z *File) MarshalMsg(b []byte) (o []byte, err error) {
	o = msgp.Require(b, z.Msgsize())
	// array header, size 8
	o = append(o, 0x98)
	o = msgp.AppendString(o, z.Name)
	o = msgp.AppendUint64(o, z.CompressedSize64)
	o = msgp.AppendUint64(o, z.UncompressedSize64)
	o = msgp.AppendInt64(o, z.Offset)
	o = msgp.AppendUint32(o, z.CRC32)
	o = msgp.AppendUint16(o, z.Method)
	o = msgp.AppendUint16(o, z.Flags)
	o = msgp.AppendMapHeader(o, uint32(len(z.Custom)))
	for za0001, za0002 := range z.Custom {
		o = msgp.AppendString(o, za0001)
		o = msgp.AppendString(o, za0002)
	}
	return
}

// UnmarshalMsg implements msgp.Unmarshaler
func (z *File) UnmarshalMsg(bts []byte) (o []byte, err error) {
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
	z.Name, bts, err = msgp.ReadStringBytes(bts)
	if err != nil {
		err = msgp.WrapError(err, "Name")
		return
	}
	z.CompressedSize64, bts, err = msgp.ReadUint64Bytes(bts)
	if err != nil {
		err = msgp.WrapError(err, "CompressedSize64")
		return
	}
	z.UncompressedSize64, bts, err = msgp.ReadUint64Bytes(bts)
	if err != nil {
		err = msgp.WrapError(err, "UncompressedSize64")
		return
	}
	z.Offset, bts, err = msgp.ReadInt64Bytes(bts)
	if err != nil {
		err = msgp.WrapError(err, "Offset")
		return
	}
	z.CRC32, bts, err = msgp.ReadUint32Bytes(bts)
	if err != nil {
		err = msgp.WrapError(err, "CRC32")
		return
	}
	z.Method, bts, err = msgp.ReadUint16Bytes(bts)
	if err != nil {
		err = msgp.WrapError(err, "Method")
		return
	}
	z.Flags, bts, err = msgp.ReadUint16Bytes(bts)
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
	if z.Custom == nil {
		z.Custom = make(map[string]string, zb0002)
	} else if len(z.Custom) > 0 {
		for key := range z.Custom {
			delete(z.Custom, key)
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
		z.Custom[za0001] = za0002
	}
	o = bts
	return
}

// Msgsize returns an upper bound estimate of the number of bytes occupied by the serialized message
func (z *File) Msgsize() (s int) {
	s = 1 + msgp.StringPrefixSize + len(z.Name) + msgp.Uint64Size + msgp.Uint64Size + msgp.Int64Size + msgp.Uint32Size + msgp.Uint16Size + msgp.Uint16Size + msgp.MapHeaderSize
	if z.Custom != nil {
		for za0001, za0002 := range z.Custom {
			_ = za0002
			s += msgp.StringPrefixSize + len(za0001) + msgp.StringPrefixSize + len(za0002)
		}
	}
	return
}

// DecodeMsg implements msgp.Decodable
func (z *files) DecodeMsg(dc *msgp.Reader) (err error) {
	var zb0002 uint32
	zb0002, err = dc.ReadArrayHeader()
	if err != nil {
		err = msgp.WrapError(err)
		return
	}
	if cap((*z)) >= int(zb0002) {
		(*z) = (*z)[:zb0002]
	} else {
		(*z) = make(files, zb0002)
	}
	for zb0001 := range *z {
		err = (*z)[zb0001].DecodeMsg(dc)
		if err != nil {
			err = msgp.WrapError(err, zb0001)
			return
		}
	}
	return
}

// EncodeMsg implements msgp.Encodable
func (z files) EncodeMsg(en *msgp.Writer) (err error) {
	err = en.WriteArrayHeader(uint32(len(z)))
	if err != nil {
		err = msgp.WrapError(err)
		return
	}
	for zb0003 := range z {
		err = z[zb0003].EncodeMsg(en)
		if err != nil {
			err = msgp.WrapError(err, zb0003)
			return
		}
	}
	return
}

// MarshalMsg implements msgp.Marshaler
func (z files) MarshalMsg(b []byte) (o []byte, err error) {
	o = msgp.Require(b, z.Msgsize())
	o = msgp.AppendArrayHeader(o, uint32(len(z)))
	for zb0003 := range z {
		o, err = z[zb0003].MarshalMsg(o)
		if err != nil {
			err = msgp.WrapError(err, zb0003)
			return
		}
	}
	return
}

// UnmarshalMsg implements msgp.Unmarshaler
func (z *files) UnmarshalMsg(bts []byte) (o []byte, err error) {
	var zb0002 uint32
	zb0002, bts, err = msgp.ReadArrayHeaderBytes(bts)
	if err != nil {
		err = msgp.WrapError(err)
		return
	}
	if cap((*z)) >= int(zb0002) {
		(*z) = (*z)[:zb0002]
	} else {
		(*z) = make(files, zb0002)
	}
	for zb0001 := range *z {
		bts, err = (*z)[zb0001].UnmarshalMsg(bts)
		if err != nil {
			err = msgp.WrapError(err, zb0001)
			return
		}
	}
	o = bts
	return
}

// Msgsize returns an upper bound estimate of the number of bytes occupied by the serialized message
func (z files) Msgsize() (s int) {
	s = msgp.ArrayHeaderSize
	for zb0003 := range z {
		s += z[zb0003].Msgsize()
	}
	return
}

// DecodeMsg implements msgp.Decodable
func (z *filesAsStructs) DecodeMsg(dc *msgp.Reader) (err error) {
	var zb0001 uint32
	zb0001, err = dc.ReadArrayHeader()
	if err != nil {
		err = msgp.WrapError(err)
		return
	}
	if zb0001 != 8 {
		err = msgp.ArrayError{Wanted: 8, Got: zb0001}
		return
	}
	var zb0002 uint32
	zb0002, err = dc.ReadArrayHeader()
	if err != nil {
		err = msgp.WrapError(err, "Names")
		return
	}
	if cap(z.Names) >= int(zb0002) {
		z.Names = (z.Names)[:zb0002]
	} else {
		z.Names = make([][]byte, zb0002)
	}
	for za0001 := range z.Names {
		z.Names[za0001], err = dc.ReadBytes(z.Names[za0001])
		if err != nil {
			err = msgp.WrapError(err, "Names", za0001)
			return
		}
	}
	var zb0003 uint32
	zb0003, err = dc.ReadArrayHeader()
	if err != nil {
		err = msgp.WrapError(err, "CSizes")
		return
	}
	if cap(z.CSizes) >= int(zb0003) {
		z.CSizes = (z.CSizes)[:zb0003]
	} else {
		z.CSizes = make([]int64, zb0003)
	}
	for za0002 := range z.CSizes {
		z.CSizes[za0002], err = dc.ReadInt64()
		if err != nil {
			err = msgp.WrapError(err, "CSizes", za0002)
			return
		}
	}
	var zb0004 uint32
	zb0004, err = dc.ReadArrayHeader()
	if err != nil {
		err = msgp.WrapError(err, "USizes")
		return
	}
	if cap(z.USizes) >= int(zb0004) {
		z.USizes = (z.USizes)[:zb0004]
	} else {
		z.USizes = make([]int64, zb0004)
	}
	for za0003 := range z.USizes {
		z.USizes[za0003], err = dc.ReadInt64()
		if err != nil {
			err = msgp.WrapError(err, "USizes", za0003)
			return
		}
	}
	var zb0005 uint32
	zb0005, err = dc.ReadArrayHeader()
	if err != nil {
		err = msgp.WrapError(err, "Offsets")
		return
	}
	if cap(z.Offsets) >= int(zb0005) {
		z.Offsets = (z.Offsets)[:zb0005]
	} else {
		z.Offsets = make([]int64, zb0005)
	}
	for za0004 := range z.Offsets {
		z.Offsets[za0004], err = dc.ReadInt64()
		if err != nil {
			err = msgp.WrapError(err, "Offsets", za0004)
			return
		}
	}
	var zb0006 uint32
	zb0006, err = dc.ReadArrayHeader()
	if err != nil {
		err = msgp.WrapError(err, "Methods")
		return
	}
	if cap(z.Methods) >= int(zb0006) {
		z.Methods = (z.Methods)[:zb0006]
	} else {
		z.Methods = make([]uint16, zb0006)
	}
	for za0005 := range z.Methods {
		z.Methods[za0005], err = dc.ReadUint16()
		if err != nil {
			err = msgp.WrapError(err, "Methods", za0005)
			return
		}
	}
	var zb0007 uint32
	zb0007, err = dc.ReadArrayHeader()
	if err != nil {
		err = msgp.WrapError(err, "Flags")
		return
	}
	if cap(z.Flags) >= int(zb0007) {
		z.Flags = (z.Flags)[:zb0007]
	} else {
		z.Flags = make([]uint16, zb0007)
	}
	for za0006 := range z.Flags {
		z.Flags[za0006], err = dc.ReadUint16()
		if err != nil {
			err = msgp.WrapError(err, "Flags", za0006)
			return
		}
	}
	z.Crcs, err = dc.ReadBytes(z.Crcs)
	if err != nil {
		err = msgp.WrapError(err, "Crcs")
		return
	}
	var zb0008 uint32
	zb0008, err = dc.ReadArrayHeader()
	if err != nil {
		err = msgp.WrapError(err, "Custom")
		return
	}
	if cap(z.Custom) >= int(zb0008) {
		z.Custom = (z.Custom)[:zb0008]
	} else {
		z.Custom = make([][]byte, zb0008)
	}
	for za0007 := range z.Custom {
		z.Custom[za0007], err = dc.ReadBytes(z.Custom[za0007])
		if err != nil {
			err = msgp.WrapError(err, "Custom", za0007)
			return
		}
	}
	return
}

// EncodeMsg implements msgp.Encodable
func (z *filesAsStructs) EncodeMsg(en *msgp.Writer) (err error) {
	// array header, size 8
	err = en.Append(0x98)
	if err != nil {
		return
	}
	err = en.WriteArrayHeader(uint32(len(z.Names)))
	if err != nil {
		err = msgp.WrapError(err, "Names")
		return
	}
	for za0001 := range z.Names {
		err = en.WriteBytes(z.Names[za0001])
		if err != nil {
			err = msgp.WrapError(err, "Names", za0001)
			return
		}
	}
	err = en.WriteArrayHeader(uint32(len(z.CSizes)))
	if err != nil {
		err = msgp.WrapError(err, "CSizes")
		return
	}
	for za0002 := range z.CSizes {
		err = en.WriteInt64(z.CSizes[za0002])
		if err != nil {
			err = msgp.WrapError(err, "CSizes", za0002)
			return
		}
	}
	err = en.WriteArrayHeader(uint32(len(z.USizes)))
	if err != nil {
		err = msgp.WrapError(err, "USizes")
		return
	}
	for za0003 := range z.USizes {
		err = en.WriteInt64(z.USizes[za0003])
		if err != nil {
			err = msgp.WrapError(err, "USizes", za0003)
			return
		}
	}
	err = en.WriteArrayHeader(uint32(len(z.Offsets)))
	if err != nil {
		err = msgp.WrapError(err, "Offsets")
		return
	}
	for za0004 := range z.Offsets {
		err = en.WriteInt64(z.Offsets[za0004])
		if err != nil {
			err = msgp.WrapError(err, "Offsets", za0004)
			return
		}
	}
	err = en.WriteArrayHeader(uint32(len(z.Methods)))
	if err != nil {
		err = msgp.WrapError(err, "Methods")
		return
	}
	for za0005 := range z.Methods {
		err = en.WriteUint16(z.Methods[za0005])
		if err != nil {
			err = msgp.WrapError(err, "Methods", za0005)
			return
		}
	}
	err = en.WriteArrayHeader(uint32(len(z.Flags)))
	if err != nil {
		err = msgp.WrapError(err, "Flags")
		return
	}
	for za0006 := range z.Flags {
		err = en.WriteUint16(z.Flags[za0006])
		if err != nil {
			err = msgp.WrapError(err, "Flags", za0006)
			return
		}
	}
	err = en.WriteBytes(z.Crcs)
	if err != nil {
		err = msgp.WrapError(err, "Crcs")
		return
	}
	err = en.WriteArrayHeader(uint32(len(z.Custom)))
	if err != nil {
		err = msgp.WrapError(err, "Custom")
		return
	}
	for za0007 := range z.Custom {
		err = en.WriteBytes(z.Custom[za0007])
		if err != nil {
			err = msgp.WrapError(err, "Custom", za0007)
			return
		}
	}
	return
}

// MarshalMsg implements msgp.Marshaler
func (z *filesAsStructs) MarshalMsg(b []byte) (o []byte, err error) {
	o = msgp.Require(b, z.Msgsize())
	// array header, size 8
	o = append(o, 0x98)
	o = msgp.AppendArrayHeader(o, uint32(len(z.Names)))
	for za0001 := range z.Names {
		o = msgp.AppendBytes(o, z.Names[za0001])
	}
	o = msgp.AppendArrayHeader(o, uint32(len(z.CSizes)))
	for za0002 := range z.CSizes {
		o = msgp.AppendInt64(o, z.CSizes[za0002])
	}
	o = msgp.AppendArrayHeader(o, uint32(len(z.USizes)))
	for za0003 := range z.USizes {
		o = msgp.AppendInt64(o, z.USizes[za0003])
	}
	o = msgp.AppendArrayHeader(o, uint32(len(z.Offsets)))
	for za0004 := range z.Offsets {
		o = msgp.AppendInt64(o, z.Offsets[za0004])
	}
	o = msgp.AppendArrayHeader(o, uint32(len(z.Methods)))
	for za0005 := range z.Methods {
		o = msgp.AppendUint16(o, z.Methods[za0005])
	}
	o = msgp.AppendArrayHeader(o, uint32(len(z.Flags)))
	for za0006 := range z.Flags {
		o = msgp.AppendUint16(o, z.Flags[za0006])
	}
	o = msgp.AppendBytes(o, z.Crcs)
	o = msgp.AppendArrayHeader(o, uint32(len(z.Custom)))
	for za0007 := range z.Custom {
		o = msgp.AppendBytes(o, z.Custom[za0007])
	}
	return
}

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

// Msgsize returns an upper bound estimate of the number of bytes occupied by the serialized message
func (z *filesAsStructs) Msgsize() (s int) {
	s = 1 + msgp.ArrayHeaderSize
	for za0001 := range z.Names {
		s += msgp.BytesPrefixSize + len(z.Names[za0001])
	}
	s += msgp.ArrayHeaderSize + (len(z.CSizes) * (msgp.Int64Size)) + msgp.ArrayHeaderSize + (len(z.USizes) * (msgp.Int64Size)) + msgp.ArrayHeaderSize + (len(z.Offsets) * (msgp.Int64Size)) + msgp.ArrayHeaderSize + (len(z.Methods) * (msgp.Uint16Size)) + msgp.ArrayHeaderSize + (len(z.Flags) * (msgp.Uint16Size)) + msgp.BytesPrefixSize + len(z.Crcs) + msgp.ArrayHeaderSize
	for za0007 := range z.Custom {
		s += msgp.BytesPrefixSize + len(z.Custom[za0007])
	}
	return
}
