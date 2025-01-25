package arq5

import (
	"bytes"
	"encoding/binary"
	"reflect"
	"strings"
	"testing"
	"time"
)

func createBinaryData(data interface{}) []byte {
	buf := new(bytes.Buffer)
	binary.Write(buf, binary.BigEndian, data)
	return buf.Bytes()
}

type byteSliceStruct struct {
	B []byte `arq5:"len64"`
}

func TestDecodeBasicTypes(t *testing.T) {
	tests := []struct {
		name     string
		input    []byte
		target   interface{}
		expected interface{}
	}{
		{
			name:     "decode bool true",
			input:    []byte{1},
			target:   new(bool),
			expected: true,
		},
		{
			name:     "decode bool false",
			input:    []byte{0},
			target:   new(bool),
			expected: false,
		},
		{
			name:     "decode int8",
			input:    []byte{0x7F},
			target:   new(int8),
			expected: int8(127),
		},
		{
			name:     "decode uint8",
			input:    []byte{0xFF},
			target:   new(uint8),
			expected: uint8(255),
		},
		{
			name:     "decode int32",
			input:    createBinaryData(int32(2147483647)),
			target:   new(int32),
			expected: int32(2147483647),
		},
		{
			name:     "decode uint32",
			input:    createBinaryData(uint32(4294967295)),
			target:   new(uint32),
			expected: uint32(4294967295),
		},
		{
			name:     "decode int64",
			input:    createBinaryData(int64(9223372036854775807)),
			target:   new(int64),
			expected: int64(9223372036854775807),
		},
		{
			name:     "decode uint64",
			input:    createBinaryData(uint64(18446744073709551615)),
			target:   new(uint64),
			expected: uint64(18446744073709551615),
		},
		{
			name:     "decode empty string",
			input:    append([]byte{1}, createBinaryData(uint64(0))...),
			target:   new(string),
			expected: "",
		},
		{
			name:   "decode null string",
			input:  []byte{0},
			target: new(string),

			expected: "",
		},
		{
			name:     "decode hello world",
			input:    append([]byte{1}, append(createBinaryData(uint64(11)), []byte("Hello World")...)...),
			target:   new(string),
			expected: "Hello World",
		},
		{
			name:     "decode empty byte array",
			input:    createBinaryData(uint64(0)),
			target:   new(byteSliceStruct),
			expected: byteSliceStruct{[]byte{}},
		},
		{
			name:     "decode byte array",
			input:    append(createBinaryData(uint64(1)), []byte{0x42}...),
			target:   new(byteSliceStruct),
			expected: byteSliceStruct{[]byte{0x42}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Unmarshal(tt.input, tt.target)
			if err != nil {
				t.Errorf("Unmarshal() error = %v", err)
				return
			}

			actual := reflect.ValueOf(tt.target).Elem().Interface()
			if !reflect.DeepEqual(actual, tt.expected) {
				t.Errorf("Unmarshal() = %v, want %v", actual, tt.expected)
			}
		})
	}
}

func TestDecodeSlice(t *testing.T) {
	// TODO: add tests for len64
	type testStruct struct {
		Numbers []int32 `arq5:"len32"`
	}

	input := []byte{
		0, 0, 0, 3, // length
		0, 0, 0, 1, // first number
		0, 0, 0, 2, // second number
		0, 0, 0, 3, // third number,
	}

	expected := testStruct{Numbers: []int32{1, 2, 3}}

	var actual testStruct
	if err := Unmarshal(input, &actual); err != nil {
		t.Errorf("Unmarshal() error = %v", err)
		return
	}

	if !reflect.DeepEqual(actual, expected) {
		t.Errorf("Unmarshal() = %v, want %v", actual, expected)
	}
}

func TestDecodeTime(t *testing.T) {
	now := time.Now().Round(time.Millisecond)
	msec := now.UnixNano() / int64(time.Millisecond)

	input := new(bytes.Buffer)
	binary.Write(input, binary.BigEndian, true) // not null
	binary.Write(input, binary.BigEndian, msec)

	var actual time.Time
	err := Unmarshal(input.Bytes(), &actual)
	if err != nil {
		t.Errorf("Unmarshal() error = %v", err)
		return
	}

	if !actual.Equal(now) {
		t.Errorf("Unmarshal() = %v, want %v", actual, now)
	}
}

func TestDecodeStruct(t *testing.T) {
	type nested struct {
		Value int32
	}

	type testStruct struct {
		Name    string
		Age     int32
		Numbers []int32 `arq5:"len32"`
		Nested  nested
	}

	input := new(bytes.Buffer)
	// Name
	binary.Write(input, binary.BigEndian, true)      // not null
	binary.Write(input, binary.BigEndian, uint64(4)) // length
	input.Write([]byte("John"))
	// Age
	binary.Write(input, binary.BigEndian, int32(30))
	// Numbers
	binary.Write(input, binary.BigEndian, uint32(2)) // length
	binary.Write(input, binary.BigEndian, int32(1))
	binary.Write(input, binary.BigEndian, int32(2))
	// Nested
	binary.Write(input, binary.BigEndian, int32(42))

	expected := testStruct{
		Name:    "John",
		Age:     30,
		Numbers: []int32{1, 2},
		Nested:  nested{Value: 42},
	}

	var actual testStruct
	err := Unmarshal(input.Bytes(), &actual)
	if err != nil {
		t.Errorf("Unmarshal() error = %v", err)
		return
	}

	if !reflect.DeepEqual(actual, expected) {
		t.Errorf("Unmarshal() = %v, want %v", actual, expected)
	}
}

func TestDecodeErrors(t *testing.T) {
	tests := []struct {
		name        string
		input       []byte
		target      interface{}
		expectedErr string
	}{
		{
			name:        "non-pointer target",
			input:       []byte{0},
			target:      struct{}{},
			expectedErr: "non-pointer passed to Decode",
		},
		{
			name:        "missing slice tag",
			input:       []byte{0},
			target:      &struct{ Numbers []int32 }{},
			expectedErr: "missing `arq5` tag to decode",
		},
		{
			name:        "unsupported type",
			input:       []byte{0},
			target:      &struct{ Ch chan int }{},
			expectedErr: "unsupported type",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Unmarshal(tt.input, tt.target)
			if err == nil {
				t.Error("Expected error, got nil")
				return
			}
			if !strings.Contains(err.Error(), tt.expectedErr) {
				t.Errorf("Expected error containing %q, got %q", tt.expectedErr, err.Error())
			}
		})
	}
}
