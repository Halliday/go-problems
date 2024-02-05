package main

// I never knew me a better time and I guess I never will
// Oh, lawdy mama those Friday nights

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/BurntSushi/toml"
	"gopkg.in/yaml.v2"
)

const Command = "goproblems"

const defaultOutput = "errors.go"
const defaultInput = "errors.csv"
const defaultPackage = "main"

const defaultCasing = "kebab"

const defaultCSVComment = "#"
const defaultCSVComma = ","

const defaultErrPrefix = "Err"
const defaultErrType = "Error"

var i *string // input file name
var o *string // output file name
var p *string // package name

var casing *string

var csvTrimLeadingSpace *bool
var csvComment *string
var csvLazyQuotes *bool
var csvComma *string

var errPrefix *string
var errType *string

var withErrorsMap *bool
var withStruct *bool

func main() {
	i = flag.String("i", defaultInput, "input file")
	o = flag.String("o", defaultOutput, "output file")
	p = flag.String("p", defaultPackage, "package name")

	casing = flag.String("casing", defaultCasing, "casing style (snake, kebab, camel) in csv")

	csvTrimLeadingSpace = flag.Bool("csv-trim-leading-space", false, "trim leading space in csv")
	csvComment = flag.String("csv-comment", defaultCSVComment, "comment in csv")
	csvLazyQuotes = flag.Bool("csv-lazy-quotes", false, "lazy quotes in csv")
	csvComma = flag.String("csv-comma", defaultCSVComma, "comma in csv")

	errPrefix = flag.String("err-prefix", defaultErrPrefix, "error prefix")
	errType = flag.String("err-type", defaultErrType, "error type")

	withErrorsMap = flag.Bool("with-errors-map", false, "generate errors map")
	withStruct = flag.Bool("with-struct", false, "generate struct")

	flag.Parse()

	catalog, err := readCatalog(*i)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: Can not read input file: %v", err)
		os.Exit(1)
	}

	if err := writeCatalog(catalog, *o); err != nil {
		fmt.Fprintf(os.Stderr, "Error: Can not write output file: %v", err)
		os.Exit(1)
	}
}

func currentCommand() string {
	var b bytes.Buffer
	b.WriteString(Command)
	if *o != defaultOutput {
		b.WriteString(" -o \"" + *o + "\"")
	}
	if *i != defaultInput {
		b.WriteString(" -i \"" + *i + "\"")
	}
	if *p != defaultPackage {
		b.WriteString(" -p " + *p)
	}

	if *casing != defaultCasing {
		b.WriteString(" -casing " + *casing)
	}

	if *csvTrimLeadingSpace {
		b.WriteString(" -csv-trim-leading-space")
	}
	if *csvComment != defaultCSVComment {
		b.WriteString(" -csv-comment \"" + *csvComment + "\"")
	}
	if *csvLazyQuotes {
		b.WriteString(" -csv-lazy-quotes")
	}
	if *csvComma != defaultCSVComma {
		b.WriteString(" -csv-comma \"" + *csvComma + "\"")
	}

	if *errPrefix != defaultErrPrefix {
		b.WriteString(" -err-prefix " + *errPrefix)
	}
	if *errType != defaultErrType {
		b.WriteString(" -err-type " + *errType)
	}

	if *withErrorsMap {
		b.WriteString(" -with-errors-map")
	}
	if *withStruct {
		b.WriteString(" -with-struct")
	}

	return b.String()
}

var kebabRegexp = regexp.MustCompile(`(^|-)[a-z]`)
var snakeRegexp = regexp.MustCompile(`(^|_)[a-z]`)

func snakeToCamel(s string) string {
	return snakeRegexp.ReplaceAllStringFunc(s, func(s string) string {
		return strings.ToUpper(strings.TrimPrefix(s, "_"))
	})
}

func kebabToCamel(s string) string {
	return kebabRegexp.ReplaceAllStringFunc(s, func(s string) string {
		return strings.ToUpper(strings.TrimPrefix(s, "-"))
	})
}

type Catalog []*Error

type Error struct {
	Type     string         `json:"type" yaml:"type" toml:"type"`
	Title    string         `json:"title" yaml:"title" toml:"title"`
	Status   int            `json:"status" yaml:"status" toml:"status"`
	Detail   string         `json:"detail,omitempty" yaml:"detail,omitempty" toml:"detail,omitempty"`
	Instance string         `json:"instance,omitempty" yaml:"instance,omitempty" toml:"instance,omitempty"`
	Data     map[string]any `json:"data,omitempty" yaml:"data,omitempty" toml:"data,omitempty"`
}

func readCatalog(i string) (Catalog, error) {
	if strings.IndexByte(i, '*') != -1 {
		return readCatalogFromGlob(i)
	}
	info, err := os.Lstat(i)
	if err != nil {
		return nil, err
	}
	if info.IsDir() {
		return readCatalogFromDir(i)
	}
	return readCatalogFromFile(i)
}

func readCatalogFromGlob(pattern string) (Catalog, error) {
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return nil, err
	}
	c := make(Catalog, 0, len(matches))
	var errs []error
	for _, match := range matches {
		e, err := readErrorFromFile(match)
		if err != nil {
			errs = append(errs, err)
		} else {
			c = append(c, e)
		}
	}
	return c, errors.Join(errs...)
}

func readCatalogFromDir(dir string) (Catalog, error) {
	files, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	c := make(Catalog, 0, len(files))
	var errs []error
	for _, file := range files {
		if file.IsDir() {
			continue
		}
		e, err := readErrorFromFile(filepath.Join(dir, file.Name()))
		if err != nil {
			errs = append(errs, err)
		} else {
			c = append(c, e)
		}
	}
	return c, errors.Join(errs...)
}

func readCatalogFromFile(name string) (Catalog, error) {
	ext := filepath.Ext(name)
	file, err := os.ReadFile(name)
	if err != nil {
		return nil, err
	}
	switch ext {
	case ".json":
		c := make(Catalog, 0, 1)
		if err := json.Unmarshal(file, &c); err != nil {
			return nil, fmt.Errorf("can not parse %q as json: %w", name, err)
		}
		return c, nil
	case ".yaml", ".yml":
		c := make(Catalog, 0, 1)
		if err := yaml.Unmarshal(file, &c); err != nil {
			return nil, fmt.Errorf("can not parse %q as yaml: %w", name, err)
		}
		return c, nil
	case ".toml":
		c := make(Catalog, 0, 1)
		if err := toml.Unmarshal(file, &c); err != nil {
			return nil, fmt.Errorf("can not parse %q as toml: %w", name, err)
		}
		return c, nil
	case ".csv":
		c := make(Catalog, 0, 1)
		if err := csvUnmarshalCatalog(file, &c); err != nil {
			return nil, err
		}
		return c, nil

	default:
		return nil, fmt.Errorf("can not parse %q: unsupported file extension %q", name, ext)
	}
}

func csvUnmarshalCatalog(file []byte, c *Catalog) error {
	r := csv.NewReader(bytes.NewBuffer(file))
	r.TrimLeadingSpace = *csvTrimLeadingSpace
	r.Comment = rune((*csvComment)[0])
	r.LazyQuotes = *csvLazyQuotes
	r.Comma = rune((*csvComma)[0])
	r.ReuseRecord = true

	record, err := r.Read()
	if err != nil {
		return fmt.Errorf("can not parse file as csv: %w", err)
	}
	cTyp := findColumn("type", record)
	cTitle := findColumn("title", record)
	cStatus := findColumn("status", record)
	cDetail := findColumn("detail", record)
	cInstance := findColumn("instance", record)
	cData := findColumn("data", record)
	row := 1
	for {
		record, err := r.Read()
		if err != nil {
			if err == io.EOF {
				break
			}
			return fmt.Errorf("can not parse file as csv: line %d: %w", row, err)
		}
		e := new(Error)
		if cTyp != -1 {
			e.Type = strings.TrimSpace(record[cTyp])
		}
		if cTitle != -1 {
			e.Title = strings.TrimSpace(record[cTitle])
		}
		if cStatus != -1 {
			status, err := strconv.Atoi(strings.TrimSpace(record[cStatus]))
			if err != nil {
				return fmt.Errorf("line %d: can not parse 'status' as integer: %w", row, err)
			}
			e.Status = status
		}
		if cDetail != -1 {
			e.Detail = strings.TrimSpace(record[cDetail])
		}
		if cInstance != -1 {
			e.Instance = strings.TrimSpace(record[cInstance])
		}
		if cData != -1 {
			dataStr := strings.TrimSpace(record[cData])
			if dataStr != "" {
				if err := json.Unmarshal([]byte(dataStr), &e.Data); err != nil {
					return fmt.Errorf("line %d: can not parse 'data' as json: %w", row, err)
				}
			}
		}
		*c = append(*c, e)
		row++
	}
	return nil
}

func findColumn(name string, record []string) int {
	for i, column := range record {
		column = strings.TrimSpace(column)
		if strings.EqualFold(column, name) {
			return i
		}
	}
	return -1
}

func readErrorFromFile(name string) (*Error, error) {
	ext := filepath.Ext(name)
	file, err := os.ReadFile(name)
	if err != nil {
		return nil, err
	}
	e := new(Error)
	base := filepath.Base(name)
	e.Type = base[:len(base)-len(ext)]

	switch ext {
	case ".json":
		if err := json.Unmarshal(file, e); err != nil {
			return nil, fmt.Errorf("can not parse %q as json: %w", name, err)
		}
	case ".yaml", ".yml":
		if err := yaml.Unmarshal(file, e); err != nil {
			return nil, fmt.Errorf("can not parse %q as yaml: %w", name, err)
		}
	case ".toml":
		if err := toml.Unmarshal(file, e); err != nil {
			return nil, fmt.Errorf("can not parse %q as toml: %w", name, err)
		}

	case ".md":
		jsonHeader := bytes.Index(file, []byte("{"))
		yamlHeader := bytes.Index(file, []byte("---"))
		tomlHeader := bytes.Index(file, []byte("+++"))
		if jsonHeader != -1 && (jsonHeader < tomlHeader || tomlHeader == -1) && (jsonHeader < yamlHeader || yamlHeader == -1) {
			headerCloser := bytes.Index(file[jsonHeader+3:], []byte("}"))
			if headerCloser == -1 {
				return nil, fmt.Errorf("can not parse %q as markdown with json from matter: missing closing brace '}'", name)
			}
			if err := json.Unmarshal(file[jsonHeader+3:headerCloser+3], e); err != nil {
				return nil, fmt.Errorf("can not parse %q as markdown with json from matter: %w", name, err)
			}
		} else if yamlHeader != -1 && (yamlHeader < tomlHeader || tomlHeader == -1) {
			headerCloser := bytes.Index(file[yamlHeader+3:], []byte("---"))
			if headerCloser == -1 {
				return nil, fmt.Errorf("can not parse %q as markdown with yaml from matter: missing closing '---'", name)
			}
			if err := yaml.Unmarshal(file[yamlHeader+3:headerCloser+3], e); err != nil {
				return nil, fmt.Errorf("can not parse %q as markdown with yaml from matter: %w", name, err)
			}
		} else if tomlHeader != -1 {
			headerCloser := bytes.Index(file[tomlHeader+3:], []byte("+++"))
			if headerCloser == -1 {
				return nil, fmt.Errorf("can not parse %q as markdown with toml from matter: missing closing '+++'", name)
			}
			if err := toml.Unmarshal(file[tomlHeader+3:headerCloser+3], e); err != nil {
				return nil, fmt.Errorf("can not parse %q as markdown with toml from matter: %w", name, err)
			}
		} else {
			return nil, fmt.Errorf("can not parse %q as markdown: missing or unsupported front matter", name)
		}
	default:
		return nil, fmt.Errorf("can not parse %q: unsupported file extension %q", name, ext)
	}

	if e.Title == "" {
		e.Title = http.StatusText(e.Status)
	}

	return e, nil
}

func writeCatalog(catalog Catalog, o string) error {
	output, err := os.Create(o)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: Can not create output file.\n%s\n", err)
		os.Exit(1)
	}

	var casingToCamel func(string) string

	switch *casing {
	case "snake":
		casingToCamel = snakeToCamel
	case "kebab":
		casingToCamel = kebabToCamel
	case "camel":
		casingToCamel = func(s string) string { return s }
	default:
		fmt.Fprintf(os.Stderr, "Error: Invalid unknown casing style %q.\n", *casing)
	}

	output.WriteString("//go:generate ")
	output.WriteString(currentCommand())

	output.WriteString("\n\n")
	output.WriteString("package " + *p + "\n\n")
	// output.WriteString("import \"fmt\"\n\n")
	if *withStruct {
		output.WriteString("// " + *errType + " is the generic error type for this package.\ntype " + *errType + ` struct {
	Type     string
	Status   int
	Title    string
	Detail   string
	Instance string
	Data     map[string]any
	Wraps    error
}

// Error implements the error interface.
func (e ` + *errType + `) Error() string {
	return e.Detail
}

// Unwrap implements the errors.Unwrap function.
func (e ` + *errType + `) Unwrap() error {
	return e.Wraps
}

// Problem implements the Problem interface.
func (e ` + *errType + `) Problem() (typ string, title string, status int, detail string, instance string, data map[string]any) {
	return e.Type, e.Title, e.Status, e.Detail, e.Instance, e.Data
}
`)
	}

	for _, p := range catalog {
		variable := *errPrefix + casingToCamel(p.Type)
		output.WriteString("\n// " + variable + " means: \"" + p.Title + "\" Type: \"" + p.Type + "\", Status: " + strconv.Itoa(p.Status) + "\n")
		output.WriteString("var " + variable + " = &" + *errType + "{Type: \"" + p.Type + "\", Status: " + strconv.Itoa(p.Status) + ", Title: \"" + p.Title + "\"")
		if p.Detail != "" {
			output.WriteString(", Detail: \"" + p.Detail + "\"")
		}
		if p.Instance != "" {
			output.WriteString(", Instance: \"" + p.Instance + "\"")
		}
		if p.Data != nil {
			data, err := marshalLiteral(p.Data)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: Can not marshal data for %q.\n%s\n", p.Type, err)
				os.Exit(1)
			}
			output.WriteString(", Data: \"" + string(data) + "\"")
		}
		output.WriteString("}\n")
	}

	if *withErrorsMap {
		output.WriteString("\nvar Errors = map[string]*" + *errType + "{")
		for _, p := range catalog {
			variable := *errPrefix + casingToCamel(p.Type)
			output.WriteString("\n\t\"" + p.Type + "\": " + variable + ",")
		}
		if len(catalog) > 0 {
			output.WriteString("\n")
		}
		output.WriteString("}\n")
	}

	return nil
}

func marshalLiteral(v interface{}) ([]byte, error) {
	var b bytes.Buffer
	err := writeLiteral(&b, v)
	return b.Bytes(), err
}

func writeLiteral(b *bytes.Buffer, v interface{}) error {
	switch v := v.(type) {
	case nil:
		b.WriteString("nil")
	case bool:
		if v {
			b.WriteString("true")
		} else {
			b.WriteString("false")
		}
	case int64:
		b.WriteString(strconv.FormatInt(v, 10))
	case float64:
		b.WriteString(strconv.FormatFloat(v, 'f', -1, 64))
	case string:
		b.WriteString(strconv.Quote(v))
	case []interface{}:
		b.WriteString("[]interface{}{")
		for i, v := range v {
			if i != 0 {
				b.WriteString(", ")
			}
			if err := writeLiteral(b, v); err != nil {
				return err
			}
		}
		b.WriteString("}")
	case map[string]interface{}:
		b.WriteString("map[string]interface{}{")
		i := 0
		for k, v := range v {
			if i != 0 {
				b.WriteString(", ")
			}
			b.WriteString(strconv.Quote(k))
			b.WriteString(": ")
			if err := writeLiteral(b, v); err != nil {
				return err
			}
			i++
		}
		b.WriteString("}")
	default:
		return fmt.Errorf("unsupported type %T", v)
	}
	return nil
}
