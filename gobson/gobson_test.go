// gobson - BSON library for Go.
// 
// Copyright (c) 2010-2011 - Gustavo Niemeyer <gustavo@niemeyer.net>
// 
// All rights reserved.
// 
// Redistribution and use in source and binary forms, with or without
// modification, are permitted provided that the following conditions are met:
// 
//     * Redistributions of source code must retain the above copyright notice,
//       this list of conditions and the following disclaimer.
//     * Redistributions in binary form must reproduce the above copyright notice,
//       this list of conditions and the following disclaimer in the documentation
//       and/or other materials provided with the distribution.
//     * Neither the name of the copyright holder nor the names of its
//       contributors may be used to endorse or promote products derived from
//       this software without specific prior written permission.
// 
// THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS
// "AS IS" AND ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT
// LIMITED TO, THE IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS FOR
// A PARTICULAR PURPOSE ARE DISCLAIMED. IN NO EVENT SHALL THE COPYRIGHT OWNER OR
// CONTRIBUTORS BE LIABLE FOR ANY DIRECT, INDIRECT, INCIDENTAL, SPECIAL,
// EXEMPLARY, OR CONSEQUENTIAL DAMAGES (INCLUDING, BUT NOT LIMITED TO,
// PROCUREMENT OF SUBSTITUTE GOODS OR SERVICES; LOSS OF USE, DATA, OR
// PROFITS; OR BUSINESS INTERRUPTION) HOWEVER CAUSED AND ON ANY THEORY OF
// LIABILITY, WHETHER IN CONTRACT, STRICT LIABILITY, OR TORT (INCLUDING
// NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY OUT OF THE USE OF THIS
// SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF SUCH DAMAGE.

package bson_test

import (
	. "launchpad.net/gocheck"
	"encoding/binary"
	"json"
	"testing"
	"reflect"
	"time"
	//"launchpad.net/gobson/bson"
	"github.com/CloudMarc/mgo/gobson"
	"os"
)


func TestAll(t *testing.T) {
	TestingT(t)
}

type S struct{}

var _ = Suite(&S{})


// Wrap up the document elements contained in data, prepending the int32
// length of the data, and appending the '\x00' value closing the document.
func wrapInDoc(data string) string {
	result := make([]byte, len(data)+5)
	binary.LittleEndian.PutUint32(result, uint32(len(result)))
	copy(result[4:], []byte(data))
	return string(result)
}

func makeZeroDoc(value interface{}) (zero interface{}) {
	v := reflect.ValueOf(value)
	t := v.Type()
	if t.Kind() == reflect.Map {
		mv := reflect.MakeMap(t)
		zero = mv.Interface()
	} else {
		pv := reflect.New(v.Type().Elem())
		zero = pv.Interface()
	}
	return zero
}

func testUnmarshal(c *C, data string, obj interface{}) {
	zero := makeZeroDoc(obj)
	err := bson.Unmarshal([]byte(data), zero)
	c.Assert(err, IsNil)
	c.Assert(zero, Equals, obj)
}


type testItemType struct {
	obj  interface{}
	data string
}

// --------------------------------------------------------------------------
// Samples from bsonspec.org:

var sampleItems = []testItemType{
	{bson.M{"hello": "world"},
		"\x16\x00\x00\x00\x02hello\x00\x06\x00\x00\x00world\x00\x00"},
	{bson.M{"BSON": []interface{}{"awesome", float64(5.05), 1986}},
		"1\x00\x00\x00\x04BSON\x00&\x00\x00\x00\x020\x00\x08\x00\x00\x00" +
			"awesome\x00\x011\x00333333\x14@\x102\x00\xc2\x07\x00\x00\x00\x00"},
}

func (s *S) TestMarshalSampleItems(c *C) {
	for i, item := range sampleItems {
		data, err := bson.Marshal(item.obj)
		c.Assert(err, IsNil)
		c.Assert(string(data), Equals, item.data,
			Bug("Failed on item %d", i))
	}
}

func (s *S) TestUnmarshalSampleItems(c *C) {
	for i, item := range sampleItems {
		value := bson.M{}
		err := bson.Unmarshal([]byte(item.data), value)
		c.Assert(err, IsNil)
		c.Assert(value, Equals, item.obj,
			Bug("Failed on item %d", i))
	}
}

// --------------------------------------------------------------------------
// Every type, ordered by the type flag. These are not wrapped with the
// length and last \x00 from the document. wrapInDoc() computes them.
// Note that all of them should be supported as two-way conversions.

var allItems = []testItemType{
	{bson.M{},
		""},
	{bson.M{"_": float64(5.05)},
		"\x01_\x00333333\x14@"},
	{bson.M{"_": "yo"},
		"\x02_\x00\x03\x00\x00\x00yo\x00"},
	{bson.M{"_": bson.M{"a": true}},
		"\x03_\x00\x09\x00\x00\x00\x08a\x00\x01\x00"},
	{bson.M{"_": []interface{}{true, false}},
		"\x04_\x00\r\x00\x00\x00\x080\x00\x01\x081\x00\x00\x00"},
	{bson.M{"_": []byte("yo")},
		"\x05_\x00\x02\x00\x00\x00\x00yo"},
	{bson.M{"_": bson.Binary{0x80, []byte("udef")}},
		"\x05_\x00\x04\x00\x00\x00\x80udef"},
	{bson.M{"_": bson.Undefined}, // Obsolete, but still seen in the wild.
		"\x06_\x00"},
	{bson.M{"_": bson.ObjectId("0123456789ab")},
		"\x07_\x000123456789ab"},
	{bson.M{"_": false},
		"\x08_\x00\x00"},
	{bson.M{"_": true},
		"\x08_\x00\x01"},
	{bson.M{"_": bson.Timestamp(258e6)}, // Note the NS <=> MS conversion.
		"\x09_\x00\x02\x01\x00\x00\x00\x00\x00\x00"},
	{bson.M{"_": nil},
		"\x0A_\x00"},
	{bson.M{"_": bson.RegEx{"ab", "cd"}},
		"\x0B_\x00ab\x00cd\x00"},
	{bson.M{"_": bson.JS{"code", nil}},
		"\x0D_\x00\x05\x00\x00\x00code\x00"},
	{bson.M{"_": bson.Symbol("sym")},
		"\x0E_\x00\x04\x00\x00\x00sym\x00"},
	{bson.M{"_": bson.JS{"code", bson.M{"": nil}}},
		"\x0F_\x00\x14\x00\x00\x00\x05\x00\x00\x00code\x00" +
			"\x07\x00\x00\x00\x0A\x00\x00"},
	{bson.M{"_": 258},
		"\x10_\x00\x02\x01\x00\x00"},
	{bson.M{"_": bson.MongoTimestamp(258)},
		"\x11_\x00\x02\x01\x00\x00\x00\x00\x00\x00"},
	{bson.M{"_": int64(258)},
		"\x12_\x00\x02\x01\x00\x00\x00\x00\x00\x00"},
	{bson.M{"_": int64(258 << 32)},
		"\x12_\x00\x00\x00\x00\x00\x02\x01\x00\x00"},
	{bson.M{"_": bson.MaxKey},
		"\x7F_\x00"},
	{bson.M{"_": bson.MinKey},
		"\xFF_\x00"},
}

func (s *S) TestMarshalAllItems(c *C) {
	for i, item := range allItems {
		data, err := bson.Marshal(item.obj)
		c.Assert(err, IsNil)
		c.Assert(string(data), Equals, wrapInDoc(item.data), Bug("Failed on item %d: %#v", i, item))
	}
}

func (s *S) TestUnmarshalAllItems(c *C) {
	for i, item := range allItems {
		value := bson.M{}
		err := bson.Unmarshal([]byte(wrapInDoc(item.data)), value)
		c.Assert(err, IsNil)
		c.Assert(value, Equals, item.obj, Bug("Failed on item %d: %#v", i, item))
	}
}

func (s *S) TestUnmarshalRawAllItems(c *C) {
	for i, item := range allItems {
		if len(item.data) == 0 {
			continue
		}
		value := item.obj.(bson.M)["_"]
		if value == nil {
			continue
		}
		pv := reflect.New(reflect.ValueOf(value).Type())
		raw := bson.Raw{item.data[0], []byte(item.data[3:])}
		c.Logf("Unmarshal raw: %#v, %#v", raw, pv.Interface())
		err := raw.Unmarshal(pv.Interface())
		c.Assert(err, IsNil)
		c.Assert(pv.Elem().Interface(), Equals, value, Bug("Failed on item %d: %#v", i, item))
	}
}

func (s *S) TestUnmarshalRawIncompatible(c *C) {
	raw := bson.Raw{0x08, []byte{0x01}} // true
	err := raw.Unmarshal(&struct{}{})
	c.Assert(err, Matches, "BSON kind 0x08 isn't compatible with type struct { }")
}

// --------------------------------------------------------------------------
// Some one way marshaling operations which would unmarshal differently.

var oneWayMarshalItems = []testItemType{
	// These are being passed as pointers, and will unmarshal as values.
	{bson.M{"": &bson.Binary{0x02, []byte("old")}},
		"\x05\x00\x07\x00\x00\x00\x02\x03\x00\x00\x00old"},
	{bson.M{"": &bson.Binary{0x80, []byte("udef")}},
		"\x05\x00\x04\x00\x00\x00\x80udef"},
	{bson.M{"": &bson.RegEx{"ab", "cd"}},
		"\x0B\x00ab\x00cd\x00"},
	{bson.M{"": &bson.JS{"code", nil}},
		"\x0D\x00\x05\x00\x00\x00code\x00"},
	{bson.M{"": &bson.JS{"code", bson.M{"": nil}}},
		"\x0F\x00\x14\x00\x00\x00\x05\x00\x00\x00code\x00" +
			"\x07\x00\x00\x00\x0A\x00\x00"},

	// There's no float32 type in BSON.  Will encode as a float64.
	{bson.M{"": float32(5.05)},
		"\x01\x00\x00\x00\x00@33\x14@"},

	// The array will be unmarshaled as a slice instead.
	{bson.M{"": [2]bool{true, false}},
		"\x04\x00\r\x00\x00\x00\x080\x00\x01\x081\x00\x00\x00"},

	// The typed slice will be unmarshaled as []interface{}.
	{bson.M{"": []bool{true, false}},
		"\x04\x00\r\x00\x00\x00\x080\x00\x01\x081\x00\x00\x00"},

	// Will unmarshal as a []byte.
	{bson.M{"": bson.Binary{0x00, []byte("yo")}},
		"\x05\x00\x02\x00\x00\x00\x00yo"},
	{bson.M{"": bson.Binary{0x02, []byte("old")}},
		"\x05\x00\x07\x00\x00\x00\x02\x03\x00\x00\x00old"},

	// No way to preserve the type information here. We might encode as a zero
	// value, but this would mean that pointer values in structs wouldn't be
	// able to correctly distinguish between unset and set to the zero value.
	{bson.M{"": (*byte)(nil)},
		"\x0A\x00"},

	// No int types smaller than int32 in BSON. Could encode this as a char,
	// but it would still be ambiguous, take more, and be awkward in Go when
	// loaded without typing information.
	{bson.M{"": byte(8)},
		"\x10\x00\x08\x00\x00\x00"},

	// There are no unsigned types in BSON.  Will unmarshal as int32 or int64.
	{bson.M{"": uint32(258)},
		"\x10\x00\x02\x01\x00\x00"},
	{bson.M{"": uint64(258)},
		"\x12\x00\x02\x01\x00\x00\x00\x00\x00\x00"},
	{bson.M{"": uint64(258 << 32)},
		"\x12\x00\x00\x00\x00\x00\x02\x01\x00\x00"},

	// This will unmarshal as int.
	{bson.M{"": int32(258)},
		"\x10\x00\x02\x01\x00\x00"},

	// That's a special case. The unsigned value is too large for an int32,
	// so an int64 is used instead.
	{bson.M{"": uint32(1<<32 - 1)},
		"\x12\x00\xFF\xFF\xFF\xFF\x00\x00\x00\x00"},
	{bson.M{"": uint(1<<32 - 1)},
		"\x12\x00\xFF\xFF\xFF\xFF\x00\x00\x00\x00"},
}

func (s *S) TestOneWayMarshalItems(c *C) {
	for i, item := range oneWayMarshalItems {
		data, err := bson.Marshal(item.obj)
		c.Assert(err, IsNil)
		c.Assert(string(data), Equals, wrapInDoc(item.data),
			Bug("Failed on item %d", i))
	}
}


// --------------------------------------------------------------------------
// Two-way tests for user-defined structures using the samples
// from bsonspec.org.

type specSample1 struct {
	Hello string
}

type specSample2 struct {
	BSON []interface{} "BSON"
}

var structSampleItems = []testItemType{
	{&specSample1{"world"},
		"\x16\x00\x00\x00\x02hello\x00\x06\x00\x00\x00world\x00\x00"},
	{&specSample2{[]interface{}{"awesome", float64(5.05), 1986}},
		"1\x00\x00\x00\x04BSON\x00&\x00\x00\x00\x020\x00\x08\x00\x00\x00" +
			"awesome\x00\x011\x00333333\x14@\x102\x00\xc2\x07\x00\x00\x00\x00"},
}


func (s *S) TestMarshalStructSampleItems(c *C) {
	for i, item := range structSampleItems {
		data, err := bson.Marshal(item.obj)
		c.Assert(err, IsNil)
		c.Assert(string(data), Equals, item.data,
			Bug("Failed on item %d", i))
	}
}

func (s *S) TestUnmarshalStructSampleItems(c *C) {
	for _, item := range structSampleItems {
		testUnmarshal(c, item.data, item.obj)
	}
}


// --------------------------------------------------------------------------
// Generic two-way struct marshaling tests.

var bytevar = byte(8)
var byteptr = &bytevar

var structItems = []testItemType{
	{&struct{ Ptr *byte }{nil},
		"\x0Aptr\x00"},
	{&struct{ Ptr *byte }{&bytevar},
		"\x10ptr\x00\x08\x00\x00\x00"},
	{&struct{ Ptr **byte }{&byteptr},
		"\x10ptr\x00\x08\x00\x00\x00"},
	{&struct{ Byte byte }{8},
		"\x10byte\x00\x08\x00\x00\x00"},
	{&struct{ Byte byte }{0},
		"\x10byte\x00\x00\x00\x00\x00"},
	{&struct {
		V byte "Tag"
	}{8},
		"\x10Tag\x00\x08\x00\x00\x00"},
	{&struct {
		V *struct {
			Byte byte
		}
	}{&struct{ Byte byte }{8}},
		"\x03v\x00" + "\x0f\x00\x00\x00\x10byte\x00\b\x00\x00\x00\x00"},
	{&struct{ priv byte }{}, ""},

	// The order of the dumped fields should be the same in the struct.
	{&struct{ A, C, B, D, F, E *byte }{},
		"\x0Aa\x00\x0Ac\x00\x0Ab\x00\x0Ad\x00\x0Af\x00\x0Ae\x00"},

	{&struct{ V bson.Raw }{bson.Raw{0x03, []byte("\x0f\x00\x00\x00\x10byte\x00\b\x00\x00\x00\x00")}},
		"\x03v\x00" + "\x0f\x00\x00\x00\x10byte\x00\b\x00\x00\x00\x00"},
	{&struct{ V bson.Raw }{bson.Raw{0x10, []byte("\x00\x00\x00\x00")}},
		"\x10v\x00" + "\x00\x00\x00\x00"},

	// Byte arrays.
	{&struct{ V [2]byte }{[2]byte{'y', 'o'}},
		"\x05v\x00\x02\x00\x00\x00\x00yo"},
}

func (s *S) TestMarshalStructItems(c *C) {
	for i, item := range structItems {
		data, err := bson.Marshal(item.obj)
		c.Assert(err, IsNil)
		c.Assert(string(data), Equals, wrapInDoc(item.data),
			Bug("Failed on item %d", i))
	}
}

func (s *S) TestUnmarshalStructItems(c *C) {
	for _, item := range structItems {
		testUnmarshal(c, wrapInDoc(item.data), item.obj)
	}
}

func (s *S) TestUnmarshalRawStructItems(c *C) {
	for i, item := range structItems {
		raw := bson.Raw{0x03, []byte(wrapInDoc(item.data))}
		zero := makeZeroDoc(item.obj)
		err := raw.Unmarshal(zero)
		c.Assert(err, IsNil)
		c.Assert(zero, Equals, item.obj, Bug("Failed on item %d: %#v", i, item))
	}
}

func (s *S) TestUnmarshalRawNil(c *C) {
	// Regression test: shouldn't try to nil out the pointer itself,
	// as it's not settable.
	raw := bson.Raw{0x0A, []byte{}}
	err := raw.Unmarshal(&struct{}{})
	c.Assert(err, IsNil)
}

// --------------------------------------------------------------------------
// One-way marshaling tests.

type dOnIface struct {
	D interface{}
}

var marshalItems = []testItemType{
	// Ordered document dump.  Will unmarshal as a dictionary by default.
	{bson.D{{"a", nil}, {"c", nil}, {"b", nil}, {"d", nil}, {"f", nil}, {"e", true}},
		"\x0Aa\x00\x0Ac\x00\x0Ab\x00\x0Ad\x00\x0Af\x00\x08e\x00\x01"},
	{&dOnIface{bson.D{{"a", nil}, {"c", nil}, {"b", nil}, {"d", true}}},
		"\x03d\x00" + wrapInDoc("\x0Aa\x00\x0Ac\x00\x0Ab\x00\x08d\x00\x01")},

	// Marshalling a Raw document does nothing.
	{bson.Raw{0x03, []byte(wrapInDoc("anything"))},
		"anything"},
	{bson.Raw{Data: []byte(wrapInDoc("anything"))},
		"anything"},
}

func (s *S) TestMarshalOneWayItems(c *C) {
	for _, item := range marshalItems {
		data, err := bson.Marshal(item.obj)
		c.Assert(err, IsNil)
		c.Assert(string(data), Equals, wrapInDoc(item.data))
	}
}

// --------------------------------------------------------------------------
// One-way unmarshaling tests.

var unmarshalItems = []testItemType{
	// Field is private.  Should not attempt to unmarshal it.
	{&struct{ priv byte }{},
		"\x10priv\x00\x08\x00\x00\x00"},

	// Wrong casing. Field names are lowercased.
	{&struct{ Byte byte }{},
		"\x10Byte\x00\x08\x00\x00\x00"},

	// Ignore non-existing field.
	{&struct{ Byte byte }{9},
		"\x10boot\x00\x08\x00\x00\x00" + "\x10byte\x00\x09\x00\x00\x00"},

	// Ignore unsuitable types silently.
	{map[string]string{"str": "s"},
		"\x02str\x00\x02\x00\x00\x00s\x00" + "\x10int\x00\x01\x00\x00\x00"},
	{map[string][]int{"array": []int{5, 9}},
		"\x04array\x00" + wrapInDoc("\x100\x00\x05\x00\x00\x00"+ "\x021\x00\x02\x00\x00\x00s\x00"+ "\x102\x00\x09\x00\x00\x00")},

	// Wrong type. Shouldn't init pointer.
	{&struct{ Str *byte }{},
		"\x02str\x00\x02\x00\x00\x00s\x00"},
	{&struct{ Str *struct{ Str string } }{},
		"\x02str\x00\x02\x00\x00\x00s\x00"},

	// Ordered document.
	{&struct{ bson.D }{bson.D{{"a", nil}, {"c", nil}, {"b", nil}, {"d", true}}},
		"\x03d\x00" + wrapInDoc("\x0Aa\x00\x0Ac\x00\x0Ab\x00\x08d\x00\x01")},

	// Raw document.
	{&bson.Raw{0x03, []byte(wrapInDoc("\x10byte\x00\x08\x00\x00\x00"))},
		"\x10byte\x00\x08\x00\x00\x00"},

	// Decode old binary.
	{bson.M{"_": []byte("old")},
		"\x05_\x00\x07\x00\x00\x00\x02\x03\x00\x00\x00old"},
}


func (s *S) TestUnmarshalOneWayItems(c *C) {
	for _, item := range unmarshalItems {
		testUnmarshal(c, wrapInDoc(item.data), item.obj)
	}
}

func (s *S) TestUnmarshalNilInStruct(c *C) {
	// Nil is the default value, so we need to ensure it's indeed being set.
	b := byte(1)
	v := &struct{ Ptr *byte }{&b}
	err := bson.Unmarshal([]byte(wrapInDoc("\x0Aptr\x00")), v)
	c.Assert(err, IsNil)
	c.Assert(v, Equals, &struct{ Ptr *byte }{nil})
}

// --------------------------------------------------------------------------
// Marshalling error cases.

type structWithDupKeys struct {
	Name  byte
	Other byte "name" // Tag should precede.
}

var marshalErrorItems = []testItemType{
	{bson.M{"": uint64(1 << 63)},
		"BSON has no uint64 type, and value is too large to fit correctly in an int64"},
	{bson.M{"": bson.ObjectId("tooshort")},
		"ObjectIDs must be exactly 12 bytes long \\(got 8\\)"},
	{int64(123),
		"Can't marshal int64 as a BSON document"},
	{bson.M{"": 1i},
		"Can't marshal complex128 in a BSON document"},
	{&structWithDupKeys{},
		"Duplicated key 'name' in struct bson_test.structWithDupKeys"},
	{bson.Raw{0x0A, []byte{}},
		"Attempted to unmarshal Raw kind 10 as a document"},
	{&inlineCantPtr{&struct{ A, B int }{1, 2}},
		"Option ,inline needs a struct value field"},
	{&inlineDupName{1, struct{ A, B int }{2, 3}},
		"Duplicated key 'a' in struct bson_test.inlineDupName"},
}

func (s *S) TestMarshalErrorItems(c *C) {
	for _, item := range marshalErrorItems {
		data, err := bson.Marshal(item.obj)
		c.Assert(err, Matches, item.data)
		c.Assert(data, IsNil)
	}
}

// --------------------------------------------------------------------------
// Unmarshalling error cases.

type unmarshalErrorType struct {
	obj   interface{}
	data  string
	error string
}

var unmarshalErrorItems = []unmarshalErrorType{
	// Tag name conflicts with existing parameter.
	{&structWithDupKeys{},
		"\x10name\x00\x08\x00\x00\x00",
		"Duplicated key 'name' in struct bson_test.structWithDupKeys"},

	// Non-string map key.
	{map[int]interface{}{},
		"\x10name\x00\x08\x00\x00\x00",
		"BSON map must have string keys. Got: map\\[int\\] interface \\{ \\}"},

	{nil,
		"\xEEname\x00",
		"Unknown element kind \\(0xEE\\)"},

	{struct{ Name bool }{},
		"\x10name\x00\x08\x00\x00\x00",
		"Unmarshal can't deal with struct values. Use a pointer."},

	{123,
		"\x10name\x00\x08\x00\x00\x00",
		"Unmarshal needs a map or a pointer to a struct."},
}


func (s *S) TestUnmarshalErrorItems(c *C) {
	for _, item := range unmarshalErrorItems {
		data := []byte(wrapInDoc(item.data))
		var value interface{}
		switch reflect.ValueOf(item.obj).Kind() {
		case reflect.Map, reflect.Ptr:
			value = makeZeroDoc(item.obj)
		case reflect.Invalid:
			value = bson.M{}
		default:
			value = item.obj
		}
		err := bson.Unmarshal(data, value)
		c.Assert(err, Matches, item.error)
	}
}


type unmarshalRawErrorType struct {
	obj   interface{}
	raw   bson.Raw
	error string
}

var unmarshalRawErrorItems = []unmarshalRawErrorType{
	// Tag name conflicts with existing parameter.
	{&structWithDupKeys{},
		bson.Raw{0x03, []byte("\x10byte\x00\x08\x00\x00\x00")},
		"Duplicated key 'name' in struct bson_test.structWithDupKeys"},

	{&struct{}{},
		bson.Raw{0xEE, []byte{}},
		"Unknown element kind \\(0xEE\\)"},

	{struct{ Name bool }{},
		bson.Raw{0x10, []byte("\x08\x00\x00\x00")},
		"Raw Unmarshal can't deal with struct values. Use a pointer."},

	{123,
		bson.Raw{0x10, []byte("\x08\x00\x00\x00")},
		"Raw Unmarshal needs a map or a valid pointer."},
}

func (s *S) TestUnmarshalRawErrorItems(c *C) {
	for i, item := range unmarshalRawErrorItems {
		err := item.raw.Unmarshal(item.obj)
		c.Assert(err, Matches, item.error, Bug("Failed on item %d: %#v\n", i, item))
	}
}


var corruptedData = []string{
	"\x04\x00\x00\x00\x00",         // Shorter than minimum
	"\x06\x00\x00\x00\x00",         // Not enough data
	"\x05\x00\x00",                 // Broken length
	"\x05\x00\x00\x00\xff",         // Corrupted termination
	"\x0A\x00\x00\x00\x0Aooop\x00", // Unfinished C string

	// Array end past end of string (s[2]=0x07 is correct)
	wrapInDoc("\x04\x00\x09\x00\x00\x00\x0A\x00\x00"),

	// Array end within string, but past acceptable.
	wrapInDoc("\x04\x00\x08\x00\x00\x00\x0A\x00\x00"),

	// Document end within string, but past acceptable.
	wrapInDoc("\x03\x00\x08\x00\x00\x00\x0A\x00\x00"),

	// String with corrupted end.
	wrapInDoc("\x02\x00\x03\x00\x00\x00yo\xFF"),
}


func (s *S) TestUnmarshalMapDocumentTooShort(c *C) {
	for _, data := range corruptedData {
		err := bson.Unmarshal([]byte(data), bson.M{})
		c.Assert(err, Matches, "Document is corrupted")

		err = bson.Unmarshal([]byte(data), &struct{}{})
		c.Assert(err, Matches, "Document is corrupted")
	}
}


// --------------------------------------------------------------------------
// Setter test cases.

var setterResult = map[string]os.Error{}

type setterType struct {
	received interface{}
}

func (o *setterType) SetBSON(raw bson.Raw) os.Error {
	err := raw.Unmarshal(&o.received)
	if err != nil {
		panic("The panic:" + err.String())
	}
	if s, ok := o.received.(string); ok {
		if result, ok := setterResult[s]; ok {
			return result
		}
	}
	return nil
}

type ptrSetterDoc struct {
	Field *setterType "_"
}

type valSetterDoc struct {
	Field setterType "_"
}

func (s *S) TestUnmarshalAllItemsWithPtrSetter(c *C) {
	for _, item := range allItems {
		for i := 0; i != 2; i++ {
			var field *setterType
			if i == 0 {
				obj := &ptrSetterDoc{}
				err := bson.Unmarshal([]byte(wrapInDoc(item.data)), obj)
				c.Assert(err, IsNil)
				field = obj.Field
			} else {
				obj := &valSetterDoc{}
				err := bson.Unmarshal([]byte(wrapInDoc(item.data)), obj)
				c.Assert(err, IsNil)
				field = &obj.Field
			}
			if item.data == "" {
				// Nothing to unmarshal. Should be untouched.
				if i == 0 {
					c.Assert(field, IsNil)
				} else {
					c.Assert(field.received, IsNil)
				}
			} else {
				expected := item.obj.(bson.M)["_"]
				c.Assert(field, NotNil, Bug("Pointer not initialized (%#v)", expected))
				c.Assert(field.received, Equals, expected)
			}
		}
	}
}

func (s *S) TestUnmarshalWholeDocumentWithSetter(c *C) {
	obj := &setterType{}
	err := bson.Unmarshal([]byte(sampleItems[0].data), obj)
	c.Assert(err, IsNil)
	c.Assert(obj.received, Equals, bson.M{"hello": "world"})
}

func (s *S) TestUnmarshalSetterOmits(c *C) {
	setterResult["2"] = &bson.TypeError{}
	setterResult["4"] = &bson.TypeError{}
	defer func() {
		setterResult["2"] = nil, false
		setterResult["4"] = nil, false
	}()

	m := map[string]*setterType{}
	data := wrapInDoc("\x02abc\x00\x02\x00\x00\x001\x00" +
		"\x02def\x00\x02\x00\x00\x002\x00" +
		"\x02ghi\x00\x02\x00\x00\x003\x00" +
		"\x02jkl\x00\x02\x00\x00\x004\x00")
	err := bson.Unmarshal([]byte(data), m)
	c.Assert(err, IsNil)
	c.Assert(m["abc"], NotNil)
	c.Assert(m["def"], IsNil)
	c.Assert(m["ghi"], NotNil)
	c.Assert(m["jkl"], IsNil)

	c.Assert(m["abc"].received, Equals, "1")
	c.Assert(m["ghi"].received, Equals, "3")
}

func (s *S) TestUnmarshalSetterErrors(c *C) {
	boom := os.NewError("BOOM")
	setterResult["2"] = boom
	defer func() {
		setterResult["2"] = nil, false
	}()

	m := map[string]*setterType{}
	data := wrapInDoc("\x02abc\x00\x02\x00\x00\x001\x00" +
		"\x02def\x00\x02\x00\x00\x002\x00" +
		"\x02ghi\x00\x02\x00\x00\x003\x00")
	err := bson.Unmarshal([]byte(data), m)
	c.Assert(err, Equals, boom)
	c.Assert(m["abc"], NotNil)
	c.Assert(m["def"], IsNil)
	c.Assert(m["ghi"], IsNil)

	c.Assert(m["abc"].received, Equals, "1")
}


func (s *S) TestDMap(c *C) {
	d := bson.D{{"a", 1}, {"b", 2}}
	c.Assert(d.Map(), Equals, bson.M{"a": 1, "b": 2})
}


// --------------------------------------------------------------------------
// Getter test cases.

type typeWithGetter struct {
	result interface{}
}

func (t *typeWithGetter) GetBSON() interface{} {
	return t.result
}

type docWithGetterField struct {
	Field *typeWithGetter "_"
}

func (s *S) TestMarshalAllItemsWithGetter(c *C) {
	for i, item := range allItems {
		if item.data == "" {
			continue
		}
		obj := &docWithGetterField{}
		obj.Field = &typeWithGetter{item.obj.(bson.M)["_"]}
		data, err := bson.Marshal(obj)
		c.Assert(err, IsNil)
		c.Assert(string(data), Equals, wrapInDoc(item.data),
			Bug("Failed on item #%d", i))
	}
}

func (s *S) TestMarshalWholeDocumentWithGetter(c *C) {
	obj := &typeWithGetter{sampleItems[0].obj}
	data, err := bson.Marshal(obj)
	c.Assert(err, IsNil)
	c.Assert(string(data), Equals, sampleItems[0].data)
}

type intGetter int64

func (t intGetter) GetBSON() interface{} {
	return int64(t)
}

type typeWithIntGetter struct {
	V intGetter ",minsize"
}

func (s *S) TestMarshalShortWithGetter(c *C) {
	obj := typeWithIntGetter{42}
	data, err := bson.Marshal(obj)
	c.Assert(err, IsNil)
	m := bson.M{}
	err = bson.Unmarshal(data, m)
	c.Assert(m["v"], Equals, 42)
}

// --------------------------------------------------------------------------
// Cross-type conversion tests.

type crossTypeItem struct {
	obj1 interface{}
	obj2 interface{}
}

type condStr struct {
	V string ",omitempty"
}
type condStrNS struct {
	V string `a:"A" bson:",omitempty" b:"B"`
}
type condBool struct {
	V bool ",omitempty"
}
type condInt struct {
	V int ",omitempty"
}
type condUInt struct {
	V uint ",omitempty"
}
type condIface struct {
	V interface{} ",omitempty"
}
type condPtr struct {
	V *bool ",omitempty"
}
type condSlice struct {
	V []string ",omitempty"
}
type condMap struct {
	V map[string]int ",omitempty"
}
type namedCondStr struct {
	V string "myv,omitempty"
}

type shortInt struct {
	V int64 ",minsize"
}
type shortUint struct {
	V uint64 ",minsize"
}
type shortIface struct {
	V interface{} ",minsize"
}
type shortPtr struct {
	V *int64 ",minsize"
}
type shortNonEmptyInt struct {
	V int64 ",minsize,omitempty"
}

type inlineInt struct {
	V struct { A, B int } ",inline"
}
type inlineCantPtr struct {
	V *struct { A, B int } ",inline"
}
type inlineDupName struct {
	A int
	V struct { A, B int } ",inline"
}

var truevar = true
var falsevar = false

var int64var = int64(42)
var int64ptr = &int64var
var intvar = int(42)
var intptr = &intvar

// That's a pretty fun test.  It will dump the first item, generate a zero
// value equivalent to the second one, load the dumped data onto it, and then
// verify that the resulting value is deep-equal to the untouched second value.
// Then, it will do the same in the *opposite* direction!
var twoWayCrossItems = []crossTypeItem{
	// int<=>int
	{&struct{ I int }{42}, &struct{ I int8 }{42}},
	{&struct{ I int }{42}, &struct{ I int32 }{42}},
	{&struct{ I int }{42}, &struct{ I int64 }{42}},
	{&struct{ I int8 }{42}, &struct{ I int32 }{42}},
	{&struct{ I int8 }{42}, &struct{ I int64 }{42}},
	{&struct{ I int32 }{42}, &struct{ I int64 }{42}},

	// uint<=>uint
	{&struct{ I uint }{42}, &struct{ I uint8 }{42}},
	{&struct{ I uint }{42}, &struct{ I uint32 }{42}},
	{&struct{ I uint }{42}, &struct{ I uint64 }{42}},
	{&struct{ I uint8 }{42}, &struct{ I uint32 }{42}},
	{&struct{ I uint8 }{42}, &struct{ I uint64 }{42}},
	{&struct{ I uint32 }{42}, &struct{ I uint64 }{42}},

	// float32<=>float64
	{&struct{ I float32 }{42}, &struct{ I float64 }{42}},

	// int<=>uint
	{&struct{ I uint }{42}, &struct{ I int }{42}},
	{&struct{ I uint }{42}, &struct{ I int8 }{42}},
	{&struct{ I uint }{42}, &struct{ I int32 }{42}},
	{&struct{ I uint }{42}, &struct{ I int64 }{42}},
	{&struct{ I uint8 }{42}, &struct{ I int }{42}},
	{&struct{ I uint8 }{42}, &struct{ I int8 }{42}},
	{&struct{ I uint8 }{42}, &struct{ I int32 }{42}},
	{&struct{ I uint8 }{42}, &struct{ I int64 }{42}},
	{&struct{ I uint32 }{42}, &struct{ I int }{42}},
	{&struct{ I uint32 }{42}, &struct{ I int8 }{42}},
	{&struct{ I uint32 }{42}, &struct{ I int32 }{42}},
	{&struct{ I uint32 }{42}, &struct{ I int64 }{42}},
	{&struct{ I uint64 }{42}, &struct{ I int }{42}},
	{&struct{ I uint64 }{42}, &struct{ I int8 }{42}},
	{&struct{ I uint64 }{42}, &struct{ I int32 }{42}},
	{&struct{ I uint64 }{42}, &struct{ I int64 }{42}},

	// int <=> timestamp.  Note the NS <=> MS conversion.
	{&struct{ I bson.Timestamp }{42e6}, &struct{ I int64 }{42}},
	{&struct{ I bson.Timestamp }{42e6}, &struct{ I int32 }{42}},
	{&struct{ I bson.Timestamp }{42e6}, &struct{ I int }{42}},

	// int <=> float
	{&struct{ I int }{42}, &struct{ I float64 }{42}},

	// int <=> bool
	{&struct{ I int }{1}, &struct{ I bool }{true}},
	{&struct{ I int }{0}, &struct{ I bool }{false}},

	// uint <=> float64
	{&struct{ I uint }{42}, &struct{ I float64 }{42}},

	// uint <=> bool
	{&struct{ I uint }{1}, &struct{ I bool }{true}},
	{&struct{ I uint }{0}, &struct{ I bool }{false}},

	// float64 <=> bool
	{&struct{ I float64 }{1}, &struct{ I bool }{true}},
	{&struct{ I float64 }{0}, &struct{ I bool }{false}},

	// string <=> string and string <=> []byte
	{&struct{ S []byte }{[]byte("abc")}, &struct{ S string }{"abc"}},
	{&struct{ S []byte }{[]byte("def")}, &struct{ S bson.Symbol }{"def"}},
	{&struct{ S string }{"ghi"}, &struct{ S bson.Symbol }{"ghi"}},

	// map <=> struct
	{&struct {
		A struct {
			B, C int
		}
	}{struct{ B, C int }{1, 2}},
		map[string]map[string]int{"a": map[string]int{"b": 1, "c": 2}}},

	{&struct{ A bson.Symbol }{"abc"}, map[string]string{"a": "abc"}},
	{&struct{ A bson.Symbol }{"abc"}, map[string][]byte{"a": []byte("abc")}},
	{&struct{ A []byte }{[]byte("abc")}, map[string]string{"a": "abc"}},
	{&struct{ A uint }{42}, map[string]int{"a": 42}},
	{&struct{ A uint }{42}, map[string]float64{"a": 42}},
	{&struct{ A uint }{1}, map[string]bool{"a": true}},
	{&struct{ A int }{42}, map[string]uint{"a": 42}},
	{&struct{ A int }{42}, map[string]float64{"a": 42}},
	{&struct{ A int }{1}, map[string]bool{"a": true}},
	{&struct{ A float64 }{42}, map[string]float32{"a": 42}},
	{&struct{ A float64 }{42}, map[string]int{"a": 42}},
	{&struct{ A float64 }{42}, map[string]uint{"a": 42}},
	{&struct{ A float64 }{1}, map[string]bool{"a": true}},
	{&struct{ A bool }{true}, map[string]int{"a": 1}},
	{&struct{ A bool }{true}, map[string]uint{"a": 1}},
	{&struct{ A bool }{true}, map[string]float64{"a": 1}},
	{&struct{ A **byte }{&byteptr}, map[string]byte{"a": 8}},

	// Slices
	{&struct{ S []int }{[]int{1, 2, 3}}, map[string][]int{"s": []int{1, 2, 3}}},
	{&struct{ S *[]int }{&[]int{1, 2, 3}}, map[string][]int{"s": []int{1, 2, 3}}},

	// Conditionals
	{&condBool{true}, map[string]bool{"v": true}},
	{&condBool{}, map[string]bool{}},
	{&condInt{1}, map[string]int{"v": 1}},
	{&condInt{}, map[string]int{}},
	{&condUInt{1}, map[string]uint{"v": 1}},
	{&condUInt{}, map[string]uint{}},
	{&condStr{"yo"}, map[string]string{"v": "yo"}},
	{&condStr{}, map[string]string{}},
	{&condStrNS{"yo"}, map[string]string{"v": "yo"}},
	{&condStrNS{}, map[string]string{}},
	{&condSlice{[]string{"yo"}}, map[string][]string{"v": []string{"yo"}}},
	{&condSlice{}, map[string][]string{}},
	{&condMap{map[string]int{"k": 1}}, bson.M{"v": bson.M{"k": 1}}},
	{&condMap{map[string]int{}}, map[string][]string{}},
	{&condMap{}, map[string][]string{}},
	{&condIface{"yo"}, map[string]string{"v": "yo"}},
	{&condIface{""}, map[string]string{"v": ""}},
	{&condIface{}, map[string]string{}},
	{&condPtr{&truevar}, map[string]bool{"v": true}},
	{&condPtr{&falsevar}, map[string]bool{"v": false}},
	{&condPtr{}, map[string]string{}},

	{&namedCondStr{"yo"}, map[string]string{"myv": "yo"}},
	{&namedCondStr{}, map[string]string{}},

	{&shortInt{1}, map[string]interface{}{"v": 1}},
	{&shortInt{1 << 30}, map[string]interface{}{"v": 1 << 30}},
	{&shortInt{1 << 31}, map[string]interface{}{"v": int64(1 << 31)}},
	{&shortUint{1 << 30}, map[string]interface{}{"v": 1 << 30}},
	{&shortUint{1 << 31}, map[string]interface{}{"v": int64(1 << 31)}},
	{&shortIface{int64(1) << 31}, map[string]interface{}{"v": int64(1 << 31)}},
	{&shortPtr{int64ptr}, map[string]interface{}{"v": intvar}},

	{&shortNonEmptyInt{1}, map[string]interface{}{"v": 1}},
	{&shortNonEmptyInt{1 << 31}, map[string]interface{}{"v": int64(1 << 31)}},
	{&shortNonEmptyInt{}, map[string]interface{}{}},

	{&inlineInt{struct{ A, B int}{1, 2}}, map[string]interface{}{"a": 1, "b": 2}},
}

// Same thing, but only one way (obj1 => obj2).
var oneWayCrossItems = []crossTypeItem{
	// map <=> struct
	{map[string]interface{}{"a": 1, "b": "2", "c": 3},
		map[string]int{"a": 1, "c": 3}},

	// Can't decode int into struct.
	{bson.M{"a": bson.M{"b": 2}}, &struct{ A bool }{}},

	// Would get decoded into a int32 too in the opposite direction.
	{&shortIface{int64(1) << 30}, map[string]interface{}{"v": 1 << 30}},
}

func testCrossPair(c *C, dump interface{}, load interface{}, bug interface{}) {
	//c.Logf("")
	zero := makeZeroDoc(load)
	data, err := bson.Marshal(dump)
	c.Assert(err, IsNil, bug)
	c.Logf("Data: %#v", string(data))
	err = bson.Unmarshal(data, zero)
	c.Assert(err, IsNil, bug)
	c.Assert(zero, Equals, load, bug)
}

func (s *S) TestTwoWayCrossPairs(c *C) {
	for i, item := range twoWayCrossItems {
		testCrossPair(c, item.obj1, item.obj2, Bug("#%d obj1 => obj2", i))
		testCrossPair(c, item.obj2, item.obj1, Bug("#%d obj1 <= obj2", i))
	}
}

func (s *S) TestOneWayCrossPairs(c *C) {
	for i, item := range oneWayCrossItems {
		testCrossPair(c, item.obj1, item.obj2, Bug("#%d obj1 => obj2", i))
	}
}

// --------------------------------------------------------------------------
// ObjectId hex representation test.

func (s *S) TestObjectIdHex(c *C) {
	id := bson.ObjectIdHex("4d88e15b60f486e428412dc9")
	c.Assert(id.String(), Equals, `ObjectIdHex("4d88e15b60f486e428412dc9")`)
	c.Assert(id.Hex(), Equals, "4d88e15b60f486e428412dc9")
}

// --------------------------------------------------------------------------
// ObjectId parts extraction tests.

type objectIdParts struct {
	id        bson.ObjectId
	timestamp int32
	machine   []byte
	pid       uint16
	counter   int32
}

var objectIds = []objectIdParts{
	objectIdParts{
		bson.ObjectIdHex("4d88e15b60f486e428412dc9"),
		1300816219,
		[]byte{0x60, 0xf4, 0x86},
		0xe428,
		4271561,
	},
	objectIdParts{
		bson.ObjectIdHex("000000000000000000000000"),
		0,
		[]byte{0x00, 0x00, 0x00},
		0x0000,
		0,
	},
	objectIdParts{
		bson.ObjectIdHex("00000000aabbccddee000001"),
		0,
		[]byte{0xaa, 0xbb, 0xcc},
		0xddee,
		1,
	},
}

func (s *S) TestObjectIdPartsExtraction(c *C) {
	for i, v := range objectIds {
		c.Assert(v.id.Timestamp(), Equals, v.timestamp, Bug("#%d Wrong timestamp value", i))
		c.Assert(v.id.Machine(), Equals, v.machine, Bug("#%d Wrong machine id value", i))
		c.Assert(v.id.Pid(), Equals, v.pid, Bug("#%d Wrong pid value", i))
		c.Assert(v.id.Counter(), Equals, v.counter, Bug("#%d Wrong counter value", i))
	}
}

func (s *S) TestNow(c *C) {
	before := time.Nanoseconds()
	time.Sleep(1e6)
	now := bson.Now()
	time.Sleep(1e6)
	after := time.Nanoseconds()
	c.Assert(reflect.TypeOf(now), Equals, reflect.TypeOf(bson.Timestamp(00)))
	c.Assert(int64(now) > before && int64(now) < after, Equals, true, Bug("now=%d, before=%d, after=%d", now, before, after))
}

// --------------------------------------------------------------------------
// ObjectId generation tests.

func (s *S) TestNewObjectId(c *C) {
	// Generate 10 ids
	ids := make([]bson.ObjectId, 10)
	for i := 0; i < 10; i++ {
		ids[i] = bson.NewObjectId()
	}
	for i := 1; i < 10; i++ {
		prevId := ids[i-1]
		id := ids[i]
		// Test for uniqueness among all other 9 generated ids
		for j, tid := range ids {
			if j != i {
				c.Assert(id, Not(Equals), tid, Bug("Generated ObjectId is not unique"))
			}
		}
		// Check that timestamp was incremented and is within 30 seconds of the previous one
		td := id.Timestamp() - prevId.Timestamp()
		c.Assert((td >= 0 && td <= 30), Equals, true, Bug("Wrong timestamp in generated ObjectId"))
		// Check that machine ids are the same
		c.Assert(id.Machine(), Equals, prevId.Machine())
		// Check that pids are the same
		c.Assert(id.Pid(), Equals, prevId.Pid())
		// Test for proper increment
		delta := int(id.Counter() - prevId.Counter())
		c.Assert(delta, Equals, 1, Bug("Wrong increment in generated ObjectId"))
	}
}

func (s *S) TestNewObjectIdSeconds(c *C) {
	sec := int32(time.Seconds())
	id := bson.NewObjectIdSeconds(sec)
	c.Assert(id.Timestamp(), Equals, sec)
	c.Assert(id.Machine(), Equals, []byte{0x00, 0x00, 0x00})
	c.Assert(int(id.Pid()), Equals, 0)
	c.Assert(int(id.Counter()), Equals, 0)
}

// --------------------------------------------------------------------------
// ObjectId JSON marshalling.

type jsonType struct {
	Id *bson.ObjectId
}

func (s *S) TestObjectIdJSONMarshaling(c *C) {
	id := bson.ObjectIdHex("4d88e15b60f486e428412dc9")
	v := jsonType{Id: &id}
	data, err := json.Marshal(&v)
	c.Assert(err, IsNil)
	c.Assert(string(data), Equals, `{"Id":"4d88e15b60f486e428412dc9"}`)
}

func (s *S) TestObjectIdJSONUnmarshaling(c *C) {
	data := []byte(`{"Id":"4d88e15b60f486e428412dc9"}`)
	v := jsonType{}
	err := json.Unmarshal(data, &v)
	c.Assert(err, IsNil)
	c.Assert(*v.Id, Equals, bson.ObjectIdHex("4d88e15b60f486e428412dc9"))
}

func (s *S) TestObjectIdJSONUnmarshalingError(c *C) {
	v := jsonType{}
	err := json.Unmarshal([]byte(`{"Id":"4d88e15b60f486e428412dc9A"}`), &v)
	c.Assert(err, Matches, `Invalid ObjectId in JSON: "4d88e15b60f486e428412dc9A"`)
	err = json.Unmarshal([]byte(`{"Id":"4d88e15b60f486e428412dcZ"}`), &v)
	c.Assert(err, Matches, `Invalid ObjectId in JSON: "4d88e15b60f486e428412dcZ" \(invalid hex char: 90\)`)
}
