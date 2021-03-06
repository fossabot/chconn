package chconn

import (
	"bytes"
	"encoding/binary"
	"io"
	"math"
	"reflect"
	"unsafe"
)

func NewWriter() *Writer {
	return &Writer{
		output: &bytes.Buffer{},
	}
}

type Writer struct {
	output                *bytes.Buffer
	offset                uint64
	isLowCardinality      bool
	keyStringDictionaryLC map[string]int
	keyLC                 []int
	stringDictionaryLC    []string
	scratch               [binary.MaxVarintLen64]byte
}

// Uvarint write a uint64 into writer and get error
func (w *Writer) Uvarint(v uint64) {
	ln := binary.PutUvarint(w.scratch[:binary.MaxVarintLen64], v)
	w.Write(w.scratch[:ln])
}

func (w *Writer) Bool(v bool) {
	if v {
		w.Uint8(1)
		return
	}
	w.Uint8(0)
}

func (w *Writer) Int8(v int8) {
	w.Uint8(uint8(v))
}

func (w *Writer) Int16(v int16) {
	w.Uint16(uint16(v))
}

func (w *Writer) Int32(v int32) {
	w.Uint32(uint32(v))
}

func (w *Writer) Int64(v int64) {
	w.Uint64(uint64(v))
}

func (w *Writer) Uint8(v uint8) {
	w.output.WriteByte(v)
}

func (w *Writer) Uint16(v uint16) {
	w.scratch[0] = byte(v)
	w.scratch[1] = byte(v >> 8)
	w.Write(w.scratch[:2])
}

func (w *Writer) Uint32(v uint32) {
	w.scratch[0] = byte(v)
	w.scratch[1] = byte(v >> 8)
	w.scratch[2] = byte(v >> 16)
	w.scratch[3] = byte(v >> 24)
	w.Write(w.scratch[:4])
}

func (w *Writer) Uint64(v uint64) {
	w.scratch[0] = byte(v)
	w.scratch[1] = byte(v >> 8)
	w.scratch[2] = byte(v >> 16)
	w.scratch[3] = byte(v >> 24)
	w.scratch[4] = byte(v >> 32)
	w.scratch[5] = byte(v >> 40)
	w.scratch[6] = byte(v >> 48)
	w.scratch[7] = byte(v >> 56)
	w.Write(w.scratch[:8])
}

func (w *Writer) Float32(v float32) {
	w.Uint32(math.Float32bits(v))
}

func (w *Writer) Float64(v float64) {
	w.Uint64(math.Float64bits(v))
}

func (w *Writer) AddLen(v uint64) {
	w.offset += v
	w.Uint64(w.offset)
}

func (w *Writer) AddStringLowCardinality(v string) {
	w.isLowCardinality = true
	if w.keyStringDictionaryLC == nil {
		w.keyStringDictionaryLC = make(map[string]int)
	}

	key, ok := w.keyStringDictionaryLC[v]
	if !ok {
		key = len(w.keyStringDictionaryLC)
		w.keyStringDictionaryLC[v] = key
		w.stringDictionaryLC = append(w.stringDictionaryLC, v)
	}
	w.keyLC = append(w.keyLC, key)
}

func (w *Writer) FlushStringLowCardinality() {
	intType := int(math.Log2(float64(len(w.stringDictionaryLC))) / 8)
	stype := serializationType | intType
	w.Int64(int64(stype))
	w.Int64(int64(len(w.stringDictionaryLC)))
	for _, val := range w.stringDictionaryLC {
		w.String(val)
	}
	w.Int64(int64(len(w.keyLC)))
	switch intType {
	case 0:
		for _, val := range w.keyLC {
			w.Uint8(uint8(val))
		}
	case 1:
		for _, val := range w.keyLC {
			w.Uint16(uint16(val))
		}
	case 2:
		for _, val := range w.keyLC {
			w.Uint32(uint32(val))
		}
	case 3:
		for _, val := range w.keyLC {
			w.Uint64(uint64(val))
		}
	}
}

func (w *Writer) String(v string) {
	str := str2Bytes(v)
	w.Uvarint(uint64(len(str)))
	w.Write(str)
}

func (w *Writer) Buffer(str []byte) {
	w.Uvarint(uint64(len(str)))
	w.Write(str)
}

func (w *Writer) Write(b []byte) {
	w.output.Write(b)
}

func (w *Writer) WriteTo(wt io.Writer) (int64, error) {
	return w.output.WriteTo(wt)
}

func (w *Writer) Reset() {
	w.offset = 0
	if w.stringDictionaryLC != nil {
		w.stringDictionaryLC = w.stringDictionaryLC[:0]
	}
	if w.keyStringDictionaryLC != nil {
		w.keyStringDictionaryLC = nil
	}

	w.keyLC = w.keyLC[:0]
	w.output.Reset()
}

func str2Bytes(str string) []byte {
	header := (*reflect.SliceHeader)(unsafe.Pointer(&str))
	header.Len = len(str)
	header.Cap = header.Len
	return *(*[]byte)(unsafe.Pointer(header))
}
