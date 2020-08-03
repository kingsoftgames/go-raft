package common

import (
	"encoding/json"
	"io"

	"github.com/vmihailenco/msgpack"
)

var codec Codec

func Decode(data []byte, v interface{}) error {
	return codec.Decode(data, v)
}
func Encode(v interface{}) ([]byte, error) {
	return codec.Encode(v)
}
func DecodeFromReader(r io.Reader, v interface{}) error {
	return codec.DecodeFromReader(r, v)
}
func EncodeToWriter(w io.Writer, v interface{}) error {
	return codec.EncodeToWriter(w, v)
}

func InitCodec(codecType string) {
	switch codecType {
	case "msgpack":
		codec = &MsgpackCodec{}
	default:
		codec = &JsonCodec{}
	}
}
func init() {
}

type Codec interface {
	Decode(data []byte, v interface{}) error
	Encode(v interface{}) ([]byte, error)
	DecodeFromReader(r io.Reader, v interface{}) error
	EncodeToWriter(w io.Writer, v interface{}) error
}
type MsgpackCodec struct {
}

func (MsgpackCodec) Decode(data []byte, v interface{}) error {
	return msgpack.Unmarshal(data, v)
}
func (MsgpackCodec) Encode(v interface{}) ([]byte, error) {
	return msgpack.Marshal(v)
}
func (MsgpackCodec) DecodeFromReader(r io.Reader, v interface{}) error {
	return msgpack.NewDecoder(r).Decode(v)
}
func (MsgpackCodec) EncodeToWriter(w io.Writer, v interface{}) error {
	return msgpack.NewEncoder(w).Encode(v)
}

type JsonCodec struct {
}

func (JsonCodec) Decode(data []byte, v interface{}) error {
	return json.Unmarshal(data, v)
}
func (JsonCodec) Encode(v interface{}) ([]byte, error) {
	return json.Marshal(v)
}
func (JsonCodec) DecodeFromReader(r io.Reader, v interface{}) error {
	return json.NewDecoder(r).Decode(v)
}
func (JsonCodec) EncodeToWriter(w io.Writer, v interface{}) error {
	return json.NewEncoder(w).Encode(v)
}
