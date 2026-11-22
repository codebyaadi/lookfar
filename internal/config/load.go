package config

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"
	_ "time/tzdata" // IANA zones must resolve on hosts without a zoneinfo database

	"github.com/go-playground/validator/v10"
	"github.com/goccy/go-yaml"
	"github.com/goccy/go-yaml/ast"
	"github.com/goccy/go-yaml/parser"
)

// DefaultPath is used when neither --config nor LOOKFAR_CONFIG is set.
const DefaultPath = "lookfar.yaml"

// EnvVar names the environment variable that selects the config file.
const EnvVar = "LOOKFAR_CONFIG"

// ResolvePath picks the config file: flag value, then LOOKFAR_CONFIG, then ./lookfar.yaml.
func ResolvePath(flagValue string, getenv func(string) string) string {
	if flagValue != "" {
		return flagValue
	}
	if v := getenv(EnvVar); v != "" {
		return v
	}
	return DefaultPath
}

// Error is one problem in the config file, with its YAML position when known.
type Error struct {
	File   string
	Line   int
	Column int
	Path   string // YAML path such as stack.components[0].name
	Msg    string
}

func (e Error) Error() string {
	var b strings.Builder
	b.WriteString(e.File)
	if e.Line > 0 {
		fmt.Fprintf(&b, ":%d:%d", e.Line, e.Column)
	}
	b.WriteString(": ")
	if e.Path != "" {
		b.WriteString(e.Path + ": ")
	}
	b.WriteString(e.Msg)
	return b.String()
}

// Errors is a list of config problems.
type Errors []Error

func (es Errors) Error() string {
	lines := make([]string, len(es))
	for i, e := range es {
		lines[i] = e.Error()
	}
	return strings.Join(lines, "\n")
}

// Load reads and validates the config file at path. lookupEnv is typically os.LookupEnv.
func Load(path string, lookupEnv func(string) (string, bool)) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}
	cfg, err := Parse(path, data, lookupEnv)
	if cfg != nil {
		if abs, aerr := filepath.Abs(path); aerr == nil {
			cfg.Dir = filepath.Dir(abs)
		}
	}
	return cfg, err
}

// Parse decodes, interpolates and validates config bytes. name is used in error messages.
func Parse(name string, data []byte, lookupEnv func(string) (string, bool)) (*Config, error) {
	if strings.TrimSpace(string(data)) == "" {
		return nil, Errors{{File: name, Msg: "config is empty"}}
	}
	file, err := parser.ParseBytes(data, 0)
	if err != nil {
		return nil, Errors{{File: name, Msg: "invalid YAML:\n" + err.Error()}}
	}
	cfg := Defaults()
	if err := yaml.UnmarshalWithOptions(data, &cfg, yaml.Strict()); err != nil {
		return nil, Errors{{File: name, Msg: "invalid config:\n" + err.Error()}}
	}

	loc := locator{name: name, file: file}
	var errs Errors
	interpolate(reflect.ValueOf(&cfg).Elem(), "", "", lookupEnv, func(path, msg string) {
		errs = append(errs, loc.errorAt(path, msg))
	})
	if len(errs) > 0 {
		return nil, errs
	}

	if err := newValidator().Struct(&cfg); err != nil {
		var verrs validator.ValidationErrors
		if !errors.As(err, &verrs) {
			return nil, err
		}
		for _, fe := range verrs {
			errs = append(errs, loc.errorAt(yamlPath(fe.Namespace()), message(fe)))
		}
		return nil, errs
	}
	for _, pe := range checkComponents(&cfg) {
		errs = append(errs, loc.errorAt(pe.path, pe.msg))
	}
	if len(errs) > 0 {
		return nil, errs
	}
	return &cfg, nil
}

// locator maps YAML paths to source positions.
type locator struct {
	name string
	file *ast.File
}

func (l locator) errorAt(path, msg string) Error {
	e := Error{File: l.name, Path: path, Msg: msg}
	// Walk up the path until a node exists: a missing key reports its parent position.
	for p := path; ; {
		if node := l.find(p); node != nil {
			if tok := node.GetToken(); tok != nil {
				e.Line, e.Column = tok.Position.Line, tok.Position.Column
				return e
			}
		}
		i := strings.LastIndexAny(p, ".[")
		if i < 0 {
			return e
		}
		p = p[:i]
	}
}

func (l locator) find(path string) ast.Node {
	yp := "$"
	if path != "" {
		yp = "$." + path
	}
	p, err := yaml.PathString(yp)
	if err != nil {
		return nil
	}
	node, err := p.FilterFile(l.file)
	if err != nil {
		return nil
	}
	return node
}

var envRef = regexp.MustCompile(`\$\{([A-Za-z_][A-Za-z0-9_]*)\}`)

// interpolate expands ${VAR} in every string field. An unset variable is an error
// when the field is tagged `required`; otherwise it expands to empty.
func interpolate(v reflect.Value, path, tag string, lookup func(string) (string, bool), report func(path, msg string)) {
	switch v.Kind() {
	case reflect.Pointer:
		if !v.IsNil() {
			interpolate(v.Elem(), path, tag, lookup, report)
		}
	case reflect.Struct:
		t := v.Type()
		for i := range t.NumField() {
			f := t.Field(i)
			name := yamlName(f)
			if name == "" {
				continue
			}
			interpolate(v.Field(i), join(path, name), f.Tag.Get("validate"), lookup, report)
		}
	case reflect.Slice:
		// For "dive,rule" the element rules follow the dive keyword.
		_, elemTag, _ := strings.Cut(tag, "dive,")
		for i := range v.Len() {
			interpolate(v.Index(i), path+"["+strconv.Itoa(i)+"]", elemTag, lookup, report)
		}
	case reflect.String:
		s := v.String()
		if !strings.Contains(s, "${") {
			return
		}
		required := hasRule(tag, "required")
		out := envRef.ReplaceAllStringFunc(s, func(m string) string {
			name := m[2 : len(m)-1]
			val, ok := lookup(name)
			if !ok || val == "" {
				if required {
					report(path, "environment variable "+name+" is not set")
				}
				return ""
			}
			return val
		})
		v.SetString(out)
	default:
	}
}

func hasRule(tag, rule string) bool {
	return slices.Contains(strings.Split(tag, ","), rule)
}

func join(path, name string) string {
	if path == "" {
		return name
	}
	return path + "." + name
}

func yamlName(f reflect.StructField) string {
	name, _, _ := strings.Cut(f.Tag.Get("yaml"), ",")
	if name == "-" {
		return ""
	}
	return name
}

var (
	domainRe = regexp.MustCompile(`^([a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?\.)+[a-z][a-z0-9-]{1,62}$`)
	brandRe  = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{1,62}$`)
)

func newValidator() *validator.Validate {
	v := validator.New(validator.WithRequiredStructEnabled())
	v.RegisterTagNameFunc(yamlName)
	must := func(tag string, fn validator.Func) {
		if err := v.RegisterValidation(tag, fn); err != nil {
			panic(err)
		}
	}
	must("domain", func(fl validator.FieldLevel) bool { return domainRe.MatchString(fl.Field().String()) })
	must("brandterm", func(fl validator.FieldLevel) bool { return brandRe.MatchString(fl.Field().String()) })
	must("timezone", func(fl validator.FieldLevel) bool {
		name := fl.Field().String()
		if name == "" || name == "Local" {
			return false
		}
		_, err := time.LoadLocation(name)
		return err == nil
	})
	must("smtpurl", func(fl validator.FieldLevel) bool {
		u, err := url.Parse(fl.Field().String())
		return err == nil && (u.Scheme == "smtp" || u.Scheme == "smtps") && u.Host != ""
	})
	return v
}

// yamlPath turns "Config.stack.components[0].name" into "stack.components[0].name".
func yamlPath(ns string) string {
	_, p, _ := strings.Cut(ns, ".")
	return p
}

func message(fe validator.FieldError) string {
	switch fe.Tag() {
	case "required":
		return "is required"
	case "required_unless":
		return "is required unless triage provider is none"
	case "oneof":
		return "must be one of: " + fe.Param()
	case "domain":
		return fmt.Sprintf("%q is not a valid domain: use lowercase like example.com (no scheme, path or port)", fe.Value())
	case "brandterm":
		return fmt.Sprintf("%q is not a valid brand term: use lowercase letters, digits and hyphens (2+ characters)", fe.Value())
	case "timezone":
		return fmt.Sprintf("unknown timezone %q: use an IANA name such as Asia/Kolkata or UTC", fe.Value())
	case "smtpurl":
		return "must be an smtp:// or smtps:// URL"
	case "http_url":
		return fmt.Sprintf("%q is not a valid http(s) URL", fe.Value())
	case "email":
		return fmt.Sprintf("%q is not a valid email address", fe.Value())
	case "min":
		return "must be at least " + fe.Param()
	case "max":
		return "must be at most " + fe.Param()
	default:
		return fmt.Sprintf("failed %q validation", fe.Tag())
	}
}
