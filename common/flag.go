package common

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"reflect"
	"strings"

	"gopkg.in/alecthomas/kingpin.v2"
	"gopkg.in/yaml.v2"
)

const (
	NOFLAG = "noflag"
)

type CommandClause kingpin.CmdClause
type FlagClause interface {
	Flag(name, help string) *kingpin.FlagClause
	Command(name, help string) *kingpin.CmdClause
}
type SubConfig interface {
	Key() string
}
type flagValue struct {
	n string
	f *reflect.Value
	v interface{}
}
type FileFlagType interface {
	//Unmarshal file type (json/yaml)
	Type() string
	//Application Help Info
	Help() string
}
type JsonType struct {
}

func (*JsonType) Type() string {
	return "json"
}

type YamlType struct {
}

func (*YamlType) Type() string {
	return "yaml"
}

func ReadFileAndParseFlag(file string, fft FileFlagType) error {
	var f Flag
	//get flag default value to fft
	if err := f.DefaultParse(fft); err != nil {
		return err
	}
	if len(file) > 0 {
		b, err := ioutil.ReadFile(file)
		if err != nil {
			return err
		}
		var Unmarshal func(data []byte, v interface{}) error
		if fft.Type() == "json" {
			Unmarshal = json.Unmarshal
		} else if fft.Type() == "yaml" {
			Unmarshal = yaml.Unmarshal
		}
		if err := Unmarshal(b, fft); err != nil {
			return err
		}
	}
	return f.Parse(fft)
}

type FileFft struct {
	file string
	fft  FileFlagType
	name string
}

func NewFileFft(file string, fft FileFlagType, name string) FileFft {
	return FileFft{
		file, fft, name,
	}
}
func ReadFileAndParseFlagWithArgsSlice(fs []FileFft, args []string) error {
	var f Flag
	//get flag default value to fft
	if err := f.DefaultParseSlice(fs, args); err != nil {
		return err
	}
	for _, f := range fs {
		file := f.file
		fft := f.fft
		if len(file) > 0 {
			b, err := ioutil.ReadFile(file)
			if err != nil {
				return err
			}
			var Unmarshal func(data []byte, v interface{}) error
			if fft.Type() == "json" {
				Unmarshal = json.Unmarshal
			} else if fft.Type() == "yaml" {
				Unmarshal = yaml.Unmarshal
			}
			if err := Unmarshal(b, fft); err != nil {
				return err
			}
		}
	}
	return f.ParseWithArgsSlice(fs, args)
}
func ReadFileAndParseFlagWithArgs(file string, fft FileFlagType, args []string) error {
	var f Flag
	//get flag default value to fft
	if err := f.DefaultParse(fft); err != nil {
		return err
	}
	if len(file) > 0 {
		b, err := ioutil.ReadFile(file)
		if err != nil {
			return err
		}
		var Unmarshal func(data []byte, v interface{}) error
		if fft.Type() == "json" {
			Unmarshal = json.Unmarshal
		} else if fft.Type() == "yaml" {
			Unmarshal = yaml.Unmarshal
		}
		if err := Unmarshal(b, fft); err != nil {
			return err
		}
	}
	return f.ParseWithArgs(fft, args)
}
func ParseFlag(fft FileFlagType) error {
	var f Flag
	return f.Parse(fft)
}
func ParseFlagWithArgs(fft FileFlagType, args []string) error {
	var f Flag
	return f.ParseWithArgs(fft, args)
}
func SetFlagWithFlagClause(fft FileFlagType, sub string, fc FlagClause) error {
	var f Flag
	f.app = fc
	return f.flag(fft, sub, fft)
}

type Flag struct {
	app    FlagClause
	ref    []flagValue
	actual map[string]bool
}

func Capitalize(str string) string {
	var upperStr string
	vv := []byte(str) //
	for i := 0; i < len(vv); i++ {
		if i == 0 {
			if vv[i] >= 97 && vv[i] <= 122 {
				vv[i] -= 32
				upperStr += string(vv[i])
			} else {
				fmt.Println("Not begins with lowercase letter,")
				return str
			}
		} else {
			upperStr += string(vv[i])
		}
	}
	return upperStr
}
func (th *Flag) trim(needActual bool) {
	if needActual {
		th.parseActual()
	}
	for _, ref := range th.ref {
		if needActual && !th.actualHas(ref.n) {
			continue
		}
		switch ref.f.Kind() {
		case reflect.Int:
			ref.f.SetInt(int64(*(ref.v.(*int))))
		case reflect.Float32:
			ref.f.SetFloat(float64(*(ref.v.(*float32))))
		case reflect.Float64:
			ref.f.SetFloat(*(ref.v.(*float64)))
		case reflect.String:
			ref.f.SetString(*(ref.v.(*string)))
		case reflect.Bool:
			ref.f.SetBool(*(ref.v.(*bool)))
		case reflect.Struct:
			ref.f.Set(ref.v.(reflect.Value).Elem())
		case reflect.Slice:
			ref.f.Set(ref.v.(reflect.Value).Elem())
		default:
			panic(ref.f.Kind().String())
		}
	}
}
func (th *Flag) addRef(fv flagValue) {
	th.ref = append(th.ref, fv)
}
func (th *Flag) flag(config interface{}, sub string, fft FileFlagType) error {
	v := reflect.ValueOf(config).Elem()
	t := reflect.TypeOf(config).Elem()
	for i := 0; i < v.NumField(); i++ {
		if _, ok := t.Field(i).Tag.Lookup(NOFLAG); ok {
			continue
		}
		field := v.Field(i)
		name, okName := t.Field(i).Tag.Lookup(fft.Type())
		if !okName {
			continue
		}
		if len(sub) > 0 {
			name = fmt.Sprintf("%s.%s", sub, name)
		}
		help, _ := t.Field(i).Tag.Lookup("help")
		def, okDef := t.Field(i).Tag.Lookup("default")
		k := field.Type().Kind()
		methodName := Capitalize(k.String())
		switch k {
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64, reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Float32, reflect.Float64, reflect.String, reflect.Bool:
			f := th.app.Flag(name, help)
			if okDef {
				f = f.Default(def)
			} else {
			}
			v := reflect.ValueOf(f).MethodByName(methodName).Call([]reflect.Value{})
			th.addRef(flagValue{name, &field, v[0].Interface()})
		case reflect.Struct:
			//new struct ptr
			v := reflect.New(field.Type())
			if err := th.flag(v.Interface(), name, fft); err != nil {
				return err
			}
			th.addRef(flagValue{name, &field, v})
		case reflect.Ptr:
			if field.IsNil() {
				field.Set(reflect.New(field.Type().Elem()))
			}
			if err := th.flag(field.Interface(), name, fft); err != nil {
				return err
			}
		case reflect.Slice:
			k = field.Type().Elem().Kind()
			methodName := Capitalize(k.String())
			f := th.app.Flag(name, help)
			switch k {
			//no string
			case reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Float32, reflect.Float64, reflect.Bool:
				methodName += "List"
			case reflect.String, reflect.Int, reflect.Uint:
				methodName += "s"
			default:
				return fmt.Errorf("no support []%s", k.String())
			}
			v := reflect.ValueOf(f).MethodByName(methodName + "s").Call([]reflect.Value{})
			th.addRef(flagValue{name, &field, v[0]})
		default:
			return fmt.Errorf("no support %s", k.String())
		}
	}
	return nil
}

//set flag default value to fft
func (th *Flag) DefaultParse(fft FileFlagType) error {
	return th.parse(fft, false)
}
func (th *Flag) DefaultParseSlice(fft []FileFft, args []string) error {
	return th.parseWithArgsSlice(fft, false, args)
}
func (th *Flag) clear() {
	th.app = nil
	th.ref = nil
	th.actual = nil
}
func (th *Flag) parse(fft FileFlagType, needActual bool) error {
	return th.parseWithArgs(fft, needActual, os.Args)
}
func (th *Flag) parseSlice(fft []FileFft, needActual bool) error {
	return th.parseWithArgsSlice(fft, needActual, os.Args)
}
func (th *Flag) parseWithArgs(fft FileFlagType, needActual bool, args []string) error {
	th.clear()
	app := kingpin.New(os.Args[0], fft.Help())
	th.ref = []flagValue{}
	if err := th.flag(fft, "", fft); err != nil {
		return err
	}

	if _, err := app.Parse(args); err != nil {
		return err
	}
	th.trim(needActual)
	th.app = app
	return nil
}
func (th *Flag) parseWithArgsSlice(ffts []FileFft, needActual bool, args []string) error {
	th.clear()
	app := kingpin.New(os.Args[0], "no help info,just parse")
	th.app = app

	th.ref = []flagValue{}
	for _, f := range ffts {
		if err := th.flag(f.fft, f.name, f.fft); err != nil {
			return err
		}
	}
	if _, err := app.Parse(args); err != nil {
		return err
	}
	th.trim(needActual)
	return nil
}
func (th *Flag) Parse(fft FileFlagType) error {
	return th.parse(fft, true)
}
func (th *Flag) ParseSlice(fft []FileFft) error {
	return th.parseSlice(fft, true)
}
func (th *Flag) ParseWithArgs(fft FileFlagType, args []string) error {
	return th.parseWithArgs(fft, true, args)
}
func (th *Flag) ParseWithArgsSlice(fft []FileFft, args []string) error {
	return th.parseWithArgsSlice(fft, true, args)
}

func (th *Flag) parseActual() {
	args := os.Args[1:]
	parseOne := func() bool {
		if len(args) == 0 {
			return false
		}
		s := args[0]
		if len(s) < 2 || s[0] != '-' {
			return false
		}
		numMinuses := 1
		if s[1] == '-' {
			numMinuses++
			if len(s) == 2 { // "--" terminates the flags
				args = args[1:]
				return false
			}
		}
		name := s[numMinuses:]
		if len(name) == 0 || name[0] == '-' || name[0] == '=' {
			return false
		}
		for i := 1; i < len(name); i++ { // equals cannot be first
			if name[i] == '=' {
				name = name[0:i]
				break
			}
		}
		name = strings.Trim(name, "no-")
		// it's a flag. does it have an argument?
		args = args[1:]
		if th.actual == nil {
			th.actual = make(map[string]bool)
		}
		th.actual[name] = true
		return true
	}
	for {
		if parseOne() {
			continue
		}
		break
	}
}
func (th *Flag) actualHas(name string) (has bool) {
	return th.actual[name]
}
