package arq5

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"reflect"
	"time"
)

type CompressionType int32

const (
	CompressionNone CompressionType = iota
	CompressionGzip
	CompressionLZ4
)

type Decoder struct {
	r   io.Reader
	err error
}

func NewDecoder(r io.Reader) *Decoder {
	return &Decoder{r: r}
}

func (d *Decoder) Decode(v interface{}) error {
	val := reflect.ValueOf(v)
	if val.Kind() != reflect.Ptr {
		return errors.New("non-pointer passed to Decode")
	}
	return d.decode(val.Elem(), "")
}

func (d *Decoder) decode(v reflect.Value, tag string) error {
	switch v.Kind() {
	case reflect.Int8:
		return decodeInt[int8](d.r, v)
	case reflect.Uint8:
		return decodeUint[uint8](d.r, v)
	case reflect.Int32:
		return decodeInt[int32](d.r, v)
	case reflect.Uint32:
		return decodeUint[uint32](d.r, v)
	case reflect.Int64:
		return decodeInt[int64](d.r, v)
	case reflect.Uint64:
		return decodeUint[uint64](d.r, v)
	case reflect.String:
		return d.decodeString(v)
	case reflect.Struct:
		return d.decodeStruct(v)
	case reflect.Slice:
		return d.decodeSlice(v, tag)
	case reflect.Array:
		return d.decodeArray(v)
	default:
		return d.decodeBasicType(v)
	}
}

func (d *Decoder) decodeArray(v reflect.Value) error {
	length := v.Len()
	arrayType := reflect.ArrayOf(length, v.Type().Elem())
	array := reflect.New(arrayType).Elem()

	for i := 0; i < length; i++ {
		if err := d.decode(array.Index(i), ""); err != nil {
			return err
		}
	}
	v.Set(array)

	return nil
}

func (d *Decoder) decodeStruct(v reflect.Value) error {
	if v.Type() == reflect.TypeOf(time.Time{}) {
		return d.decodeDate(v)
	}

	for i := 0; i < v.NumField(); i++ {
		field := v.Field(i)
		tag := v.Type().Field(i).Tag.Get("arq5")

		if err := d.decode(field, tag); err != nil {
			return err
		}
	}
	return nil
}

func (d *Decoder) decodeSlice(v reflect.Value, tag string) error {
	var length uint64

	switch tag {
	case "len32":
		var l uint32
		if err := binary.Read(d.r, binary.BigEndian, &l); err != nil {
			return err
		}
		length = uint64(l)
	case "len64":
		if err := binary.Read(d.r, binary.BigEndian, &length); err != nil {
			return err
		}
	default:
		return fmt.Errorf("%v missing `arq5` tag to decode, found '%v'", v.Type(), tag)
	}

	slice := reflect.MakeSlice(v.Type(), int(length), int(length))
	for i := 0; i < int(length); i++ {
		if err := d.decode(slice.Index(i), ""); err != nil {
			return err
		}
	}
	v.Set(slice)
	return nil
}

func (d *Decoder) decodeString(v reflect.Value) error {
	var isNotNull bool
	if err := binary.Read(d.r, binary.BigEndian, &isNotNull); err != nil {
		return err
	}
	if isNotNull == false {
		v.SetString("")
		return nil
	}

	var length uint64
	if err := binary.Read(d.r, binary.BigEndian, &length); err != nil {
		return err
	}

	data := make([]byte, length)
	if _, err := io.ReadFull(d.r, data); err != nil {
		return err
	}

	v.SetString(string(data))
	return nil
}

func (d *Decoder) decodeDate(v reflect.Value) error {
	var isNotNull bool
	if err := binary.Read(d.r, binary.BigEndian, &isNotNull); err != nil {
		return err
	}
	if isNotNull == false {
		v.SetZero()
		return nil
	}

	var msec int64
	if err := binary.Read(d.r, binary.BigEndian, &msec); err != nil {
		return err
	}

	t := time.Unix(0, msec*int64(time.Millisecond))
	v.Set(reflect.ValueOf(t))

	return nil
}

func decodeInt[T int8 | int32 | int64](r io.Reader, v reflect.Value) error {
	var value T
	if err := binary.Read(r, binary.BigEndian, &value); err != nil {
		return err
	}
	v.SetInt(int64(value))
	return nil
}

func decodeUint[T uint8 | uint32 | uint64](r io.Reader, v reflect.Value) error {
	var value T
	if err := binary.Read(r, binary.BigEndian, &value); err != nil {
		return err
	}
	v.SetUint(uint64(value))
	return nil
}

func (d *Decoder) decodeBasicType(v reflect.Value) error {
	switch v.Type() {
	case reflect.TypeFor[bool]():
		var value bool
		if err := binary.Read(d.r, binary.BigEndian, &value); err != nil {
			return err
		}
		v.SetBool(value)
	case reflect.TypeFor[CompressionType]():
		var value int32
		if err := binary.Read(d.r, binary.BigEndian, &value); err != nil {
			return err
		}
		v.SetInt(int64(value))
	default:
		return fmt.Errorf("unsupported type %v", v.Type())
	}
	return nil
}

func Unmarshal(data []byte, v interface{}) error {
	return NewDecoder(bytes.NewReader(data)).Decode(v)
}

func Read(r io.Reader, v interface{}) error {
	return NewDecoder(r).Decode(v)
}
