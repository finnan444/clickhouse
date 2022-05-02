// Code generated by make codegen DO NOT EDIT.
// source: lib/column/codegen/column.tpl

package column

import (
	"math/big"
	"reflect"
	"strings"
	"fmt"
	"time"
	"net"
	"github.com/google/uuid"
	"github.com/paulmach/orb"
	"github.com/shopspring/decimal"
)

func (t Type) Column() (Interface, error) {
	switch t {
{{- range . }}
	case "{{ .ChType }}":
		return &{{ .ChType }}{}, nil
{{- end }}
	case "Int128":
		return &BigInt{
			size: 16,
			chType: t,
		}, nil
	case "Int256":
		return &BigInt{
			size: 32,
			chType: t,
		}, nil
	case "UInt256":
		return &BigInt{
			size: 32,
			chType: t,
		}, nil
	case "IPv4":
		return &IPv4{}, nil
	case "IPv6":
		return &IPv6{}, nil
	case "Bool", "Boolean":
		return &Bool{}, nil
	case "Date":
		return &Date{}, nil
	case "Date32":
		return &Date32{}, nil
	case "UUID":
		return &UUID{}, nil
	case "Nothing":
		return &Nothing{}, nil
	case "Ring":
		v, err := (&Array{}).parse("Array(Point)")
		if err != nil{
			return nil, err
		}
		set := v.(*Array)
		set.chType = "Ring"
		return &Ring{
			set: set,
		}, nil
	case "Polygon":
		v, err := (&Array{}).parse("Array(Ring)")
		if err != nil{
			return nil, err
		}
		set := v.(*Array)
		set.chType = "Polygon"
		return &Polygon{
			set: set,
		}, nil
	case "MultiPolygon":
		v, err := (&Array{}).parse("Array(Polygon)")
		if err != nil{
			return nil, err
		}
		set := v.(*Array)
		set.chType = "MultiPolygon"
		return &MultiPolygon{
			set: set,
		}, nil
	case "Point":
		return &Point{}, nil
	case "String":
		return &String{}, nil
	}

	switch strType := string(t); {
	case strings.HasPrefix(string(t), "Map("):
		return (&Map{}).parse(t)
	case strings.HasPrefix(string(t), "Tuple("):
		return (&Tuple{}).parse(t)
	case strings.HasPrefix(string(t), "Decimal("):
		return (&Decimal{}).parse(t)
	case strings.HasPrefix(string(t), "Array("):
		return (&Array{}).parse(t)
	case strings.HasPrefix(string(t), "Interval"):
		return (&Interval{}).parse(t)
	case strings.HasPrefix(string(t), "Nullable"):
		return (&Nullable{}).parse(t)
	case strings.HasPrefix(string(t), "FixedString"):
		return (&FixedString{}).parse(t)
	case strings.HasPrefix(string(t), "LowCardinality"):
		return (&LowCardinality{}).parse(t)
	case strings.HasPrefix(string(t), "SimpleAggregateFunction"):
		return (&SimpleAggregateFunction{}).parse(t)
	case strings.HasPrefix(string(t), "Enum8") || strings.HasPrefix(string(t), "Enum16"):
		return Enum(t)
	case strings.HasPrefix(string(t), "DateTime64"):
		return (&DateTime64{}).parse(t)
	case strings.HasPrefix(strType, "DateTime") && !strings.HasPrefix(strType, "DateTime64"):
		return (&DateTime{}).parse(t)
	}
	return &UnsupportedColumnType{
		t: t,
	}, nil
}

type (
{{- range . }}
	{{ .ChType }} []{{ .GoType }}
{{- end }}
)

var (
{{- range . }}
	_ Interface = (*{{ .ChType }})(nil)
{{- end }}
)

var (
	{{- range . }}
		scanType{{ .ChType }} = reflect.TypeOf({{ .GoType }}(0))
	{{- end }}
		scanTypeIP      = reflect.TypeOf(net.IP{})
		scanTypeBool    = reflect.TypeOf(true)
		scanTypeByte    = reflect.TypeOf([]byte{})
		scanTypeUUID    = reflect.TypeOf(uuid.UUID{})
		scanTypeTime    = reflect.TypeOf(time.Time{})
		scanTypeRing    = reflect.TypeOf(orb.Ring{})
		scanTypePoint   = reflect.TypeOf(orb.Point{})
		scanTypeSlice   = reflect.TypeOf([]interface{}{})
		scanTypeBigInt  = reflect.TypeOf(&big.Int{})
		scanTypeString  = reflect.TypeOf("")
		scanTypePolygon = reflect.TypeOf(orb.Polygon{})
		scanTypeDecimal = reflect.TypeOf(decimal.Decimal{})
		scanTypeMultiPolygon = reflect.TypeOf(orb.MultiPolygon{})
	)

{{- range . }}

func (col *{{ .ChType }}) Type() Type {
	return "{{ .ChType }}"
}

func (col *{{ .ChType }}) ScanType() reflect.Type {
	return scanType{{ .ChType }}
}

func (col *{{ .ChType }}) Rows() int {
	return len(*col)
}

func (col *{{ .ChType }}) ScanRow(dest interface{}, row int) error {
	value := *col
	switch d := dest.(type) {
	case *{{ .GoType }}:
		*d = value[row]
	case **{{ .GoType }}:
		*d = new({{ .GoType }})
		**d = value[row]
	default:
		return &ColumnConverterError{
			Op:   "ScanRow",
			To:   fmt.Sprintf("%T", dest),
			From: "{{ .ChType }}",
			Hint: fmt.Sprintf("try using *%s", scanType{{ .ChType }}),
		}
	}
	return nil
}

func (col *{{ .ChType }}) Row(i int, ptr bool) interface{} {
	value := *col
	if ptr {
		return &value[i]
	}
	return value[i]
}

func (col *{{ .ChType }}) Append(v interface{}) (nulls []uint8,err error) {
	switch v := v.(type) {
	case []{{ .GoType }}:
		*col, nulls = append(*col, v...), make([]uint8, len(v))
	case []*{{ .GoType }}:
		nulls = make([]uint8, len(v))
		for i, v:= range v {
			switch {
			case v != nil:
				*col = append(*col, *v)
			default:
				*col, nulls[i] = append(*col, 0), 1
			}
		}
	default:
		return nil, &ColumnConverterError{
			Op:   "Append",
			To:   "{{ .ChType }}",
			From: fmt.Sprintf("%T", v),
		}
	}
	return
}

func (col *{{ .ChType }}) AppendRow(v interface{}) error {
	switch v := v.(type) {
	case {{ .GoType }}:
		*col = append(*col, v)
	case *{{ .GoType }}:
		switch {
		case v != nil:
			*col = append(*col, *v)
		default:
			*col = append(*col, 0)
		}
	case nil:
		*col = append(*col, 0)
	default:
		return &ColumnConverterError{
			Op:   "AppendRow",
			To:   "{{ .ChType }}",
			From: fmt.Sprintf("%T", v),
		}
	}
	return nil
}

{{- end }}