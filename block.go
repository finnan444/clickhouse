package clickhouse

import (
	"bytes"
	"database/sql/driver"
	"fmt"
	"io"
	"sync"
)

// Recycle column buffers, preallocate column buffers
var bufferPool = sync.Pool{
	New: func() interface{} {
		return bytes.NewBuffer(make([]byte, 0, 256*1024))
	},
}

// data block
type block struct {
	table       string
	info        blockInfo
	numRows     uint64
	numColumns  uint64
	columnNames []string
	columnTypes []string
	columnInfo  []interface{}
	columns     [][]interface{}
	offsets     [][]uint64
	buffers     []*bytes.Buffer
}

type blockInfo struct {
	num1        uint64
	isOverflows bool
	num2        uint64
	bucketNum   int32
	num3        uint64
}

func (info *blockInfo) read(r io.Reader) error {
	var err error
	if info.num1, err = readUvarint(r); err != nil {
		return err
	}
	if info.isOverflows, err = readBool(r); err != nil {
		return err
	}
	if info.num2, err = readUvarint(r); err != nil {
		return err
	}
	if info.bucketNum, err = readInt32(r); err != nil {
		return err
	}
	if info.num3, err = readUvarint(r); err != nil {
		return err
	}
	return nil
}

func (info *blockInfo) write(w io.Writer) error {
	if err := writeUvarint(w, info.num1); err != nil {
		return err
	}
	if info.num1 != 0 {
		if err := writeBool(w, info.isOverflows); err != nil {
			return err
		}
		if err := writeUvarint(w, info.num2); err != nil {
			return err
		}
		if err := writeInt32(w, info.bucketNum); err != nil {
			return err
		}
		if err := writeUvarint(w, info.num3); err != nil {
			return err
		}
	}
	return nil
}

func (b *block) read(revision uint64, r io.Reader) error {
	var err error
	if revision >= DBMS_MIN_REVISION_WITH_TEMPORARY_TABLES {
		if b.table, err = readString(r); err != nil {
			return err
		}
	}
	if revision >= DBMS_MIN_REVISION_WITH_BLOCK_INFO {
		if err := b.info.read(r); err != nil {
			return err
		}
	}
	if b.numColumns, err = readUvarint(r); err != nil {
		return err
	}
	if b.numRows, err = readUvarint(r); err != nil {
		return err
	}
	b.columns = make([][]interface{}, b.numColumns)
	for i := 0; i < int(b.numColumns); i++ {
		var columnName, columnType string

		if columnName, err = readString(r); err != nil {
			return err
		}
		if columnType, err = readString(r); err != nil {
			return err
		}
		// Coerce column type to Go type
		columnInfo, err := toColumnType(columnType)
		if err != nil {
			return err
		}
		b.columnInfo = append(b.columnInfo, columnInfo)
		b.columnNames = append(b.columnNames, columnName)
		b.columnTypes = append(b.columnTypes, columnType)
		switch info := columnInfo.(type) {
		case array:
			offsets := make([]uint64, 0, b.numRows)
			for row := 0; row < int(b.numRows); row++ {
				offset, err := readUInt64(r)
				if err != nil {
					return err
				}
				offsets = append(offsets, offset)
			}
			for n, offset := range offsets {
				len := offset
				if n != 0 {
					len = len - offsets[n-1]
				}
				value, err := readArray(r, info.baseType, len)
				if err != nil {
					return err
				}
				b.columns[i] = append(b.columns[i], value)
			}
		default:
			for row := 0; row < int(b.numRows); row++ {
				value, err := read(r, columnInfo)
				if err != nil {
					return err
				}
				b.columns[i] = append(b.columns[i], value)
			}
		}
	}
	return nil
}

func (b *block) write(revision uint64, w io.Writer) error {
	if err := writeUvarint(w, ClientDataPacket); err != nil {
		return err
	}
	if revision >= DBMS_MIN_REVISION_WITH_TEMPORARY_TABLES {
		if err := writeString(w, b.table); err != nil {
			return err
		}
	}
	if revision >= DBMS_MIN_REVISION_WITH_BLOCK_INFO {
		if err := b.info.write(w); err != nil {
			return err
		}
	}
	if err := writeUvarint(w, b.numColumns); err != nil {
		return err
	}
	if err := writeUvarint(w, b.numRows); err != nil {
		return err
	}
	for i, column := range b.columnNames {
		columnType := b.columnTypes[i]
		if err := writeString(w, column); err != nil {
			return err
		}
		if err := writeString(w, columnType); err != nil {
			return err
		}
		for _, offset := range b.offsets[i] {
			if err := writeUInt64(w, offset); err != nil {
				return err
			}
		}
		if _, err := b.buffers[i].WriteTo(w); err != nil {
			return err
		}
	}
	return nil
}

func (b *block) append(args []driver.Value) error {
	if len(b.buffers) == 0 && len(args) != 0 {
		b.numRows = 0
		b.offsets = make([][]uint64, len(args))
		b.buffers = make([]*bytes.Buffer, len(args))
		for i := range args {
			b.buffers[i] = bufferPool.Get().(*bytes.Buffer)
		}
	}
	b.numRows++
	for columnNum, info := range b.columnInfo {
		var (
			column = b.columnNames[columnNum]
			buffer = b.buffers[columnNum]
		)
		switch v := info.(type) {
		case array:
			array, ok := args[columnNum].([]byte)
			if !ok {
				return fmt.Errorf("Column %s (%s): unexpected type %T of value", column, b.columnTypes[columnNum], args[columnNum])
			}
			ct, arrayLen, data, err := arrayInfo(array)
			if err != nil {
				return err
			}
			if len(b.offsets[columnNum]) == 0 {
				b.offsets[columnNum] = append(b.offsets[columnNum], arrayLen)
			} else {
				b.offsets[columnNum] = append(b.offsets[columnNum], arrayLen+b.offsets[columnNum][len(b.offsets[columnNum])-1])
			}
			switch v := v.baseType.(type) {
			case enum8:
				if data, err = arrayStringToArrayEnum(arrayLen, data, enum(v)); err != nil {
					return err
				}
			case enum16:
				if data, err = arrayStringToArrayEnum(arrayLen, data, enum(v)); err != nil {
					return err
				}
			default:
				if "Array("+ct+")" != b.columnTypes[columnNum] {
					return fmt.Errorf("Column %s (%s): unexpected type %s of value", column, b.columnTypes[columnNum], ct)
				}
			}
			if _, err := buffer.Write(data); err != nil {
				return err
			}
		case enum8:
			ident, ok := args[columnNum].(string)
			if !ok {
				return fmt.Errorf("Column %s (%s): invalid ident type %T", column, b.columnTypes[columnNum], args[columnNum])
			}
			var (
				enum       = enum(v)
				value, err = enum.toValue(ident)
			)
			if err != nil {
				return fmt.Errorf("Column %s (%s): %s", column, b.columnTypes[columnNum], err.Error())
			}
			if err := write(buffer, v, value); err != nil {
				return fmt.Errorf("Column %s (%s): %s", column, b.columnTypes[columnNum], err.Error())
			}
		case enum16:
			ident, ok := args[columnNum].(string)
			if !ok {
				return fmt.Errorf("Column %s (%s): invalid ident type %T", column, b.columnTypes[columnNum], args[columnNum])
			}
			var (
				enum       = enum(v)
				value, err = enum.toValue(ident)
			)
			if err != nil {
				return fmt.Errorf("Column %s (%s): %s", column, b.columnTypes[columnNum], err.Error())
			}
			if err := write(buffer, v, value); err != nil {
				return fmt.Errorf("Column %s (%s): %s", column, b.columnTypes[columnNum], err.Error())
			}
		default:
			if err := write(buffer, info, args[columnNum]); err != nil {
				return fmt.Errorf("Column %s (%s): %s", column, b.columnTypes[columnNum], err.Error())
			}
		}
	}
	return nil
}

// Reset and recycle column buffers
func (b *block) reset() {
	if b == nil {
		return
	}
	for _, b := range b.buffers {
		b.Reset()
		bufferPool.Put(b)
	}
	b.buffers = nil
}
