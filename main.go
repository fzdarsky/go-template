package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/Masterminds/sprig/v3"
	"github.com/pkg/errors"
	"github.com/spf13/pflag"
	"gopkg.in/yaml.v3"
)

type Options struct {
	TemplateFiles []string // list of template files
	Values        []string // --set
	ValueFiles    []string // --set-from-file
	OutputFile    string
}

func NewOptions() Options {
	return Options{}
}

func main() {
	opts, err := parseOptions()
	if err != nil {
		usageAndExit(err)
	}

	parameters, err := populateParameterMap(opts)
	if err != nil {
		errorAndExit(err)
	}

	// create the base template and load the function map
	t := template.New("base").Option("missingkey=error")
	includedNames := make(map[string]int)
	funcMap := sprig.FuncMap()
	funcMap["include"] = includeFun(t, includedNames)
	t = t.Funcs(funcMap)

	// read the specified templates
	for _, path := range opts.TemplateFiles {
		templateName := filepath.Base(path)

		buf, err := os.ReadFile(filepath.Clean(path))
		if err != nil {
			errorAndExit(fmt.Errorf("reading template file: %w", err))
		}

		if _, err := t.New(templateName).Parse(string(buf)); err != nil {
			errorAndExit(fmt.Errorf("parsing template file: %w", err))
		}
	}

	// execute templates if they are not libary templates
	for _, path := range opts.TemplateFiles {
		templateName := filepath.Base(path)

		if strings.HasPrefix(filepath.Base(path), "_") {
			continue
		}

		var buf strings.Builder
		if err := t.ExecuteTemplate(&buf, templateName, parameters); err != nil {
			errorAndExit(fmt.Errorf("executing template: %w", err))
		}

		if len(opts.OutputFile) > 0 {
			os.WriteFile(opts.OutputFile, []byte(buf.String()), 0600)
		} else {
			fmt.Fprintln(os.Stderr, buf.String())
		}
	}
}

func parseOptions() (Options, error) {
	opts := NewOptions()
	pflag.CommandLine.Init(os.Args[0], pflag.ContinueOnError)
	pflag.Usage = func() {
		fmt.Fprintf(os.Stderr, `Usage: %[1]s [flags] TEMPLATES...

Render Go TEMPLATES to stdout, parameterising them using values read from command line or YAML/JSON files.
`, filepath.Base(os.Args[0]))
		pflag.PrintDefaults()
	}
	pflag.StringVarP(&opts.OutputFile, "output", "o", opts.OutputFile, "output rendered templates to file rather than stdout")
	pflag.StringSliceVar(&opts.Values, "set", opts.Values, "set the parameter at .Values.KEY to a given VALUE (can specify multiple or separate with commas: key1=val1,key2=val2)")
	pflag.StringSliceVar(&opts.ValueFiles, "set-from-file", opts.ValueFiles, "set the parameter at .Values.KEY to the YAML or JSON object read from FILE (can specify multiple or separate with commas: key1=file1,key2=file2)")
	if err := pflag.CommandLine.Parse(os.Args[1:]); err != nil {
		return Options{}, err
	}
	if pflag.NArg() < 1 {
		return Options{}, fmt.Errorf("need at least one template file")
	}
	opts.TemplateFiles = pflag.Args()
	return opts, nil
}

func populateParameterMap(opts Options) (map[string]interface{}, error) {
	valueMap := map[string]interface{}{}
	parameterMap := map[string]interface{}{}
	parameterMap["Values"] = valueMap

	values, err := stringArrayToMap(opts.Values)
	if err != nil {
		return nil, err
	}
	for key, value := range values {
		valueMap[key] = value
	}

	values, err = stringArrayToMap(opts.ValueFiles)
	if err != nil {
		return nil, err
	}
	for key, path := range values {
		buf, err := os.ReadFile(filepath.Clean(path))
		if err != nil {
			return nil, fmt.Errorf("reading values file: %w", err)
		}

		value := make(map[string]interface{})
		if err := yaml.Unmarshal(buf, value); err != nil {
			return nil, fmt.Errorf("unmarshaling values file: %w", err)
		}
		valueMap[key] = value
	}

	return parameterMap, nil
}

func usageAndExit(err error) {
	exitCode := 0
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s\n\n", err.Error())
		exitCode = 2
	}

	pflag.Usage()
	os.Exit(exitCode)
}

func errorAndExit(err error) {
	fmt.Fprintf(os.Stderr, "Error: %s\n", err.Error())
	os.Exit(1)
}

func stringArrayToMap(kvStrings []string) (map[string]string, error) {
	result := map[string]string{}
	for _, kvString := range kvStrings {
		parts := strings.SplitN(kvString, "=", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("expected key=value string, got %q", kvString)
		}
		if parts[0] == "" || parts[1] == "" {
			return nil, fmt.Errorf("key or value cannot be empty, got %q", kvString)
		}
		result[parts[0]] = parts[1]
	}
	return result, nil
}

const recursionMaxNums = 1000

// from github.com/helm/helm/pkg/engine/engine.go
func includeFun(t *template.Template, includedNames map[string]int) func(string, interface{}) (string, error) {
	return func(name string, data interface{}) (string, error) {
		var buf strings.Builder
		if v, ok := includedNames[name]; ok {
			if v > recursionMaxNums {
				return "", errors.Wrapf(fmt.Errorf("unable to execute template"), "rendering template has a nested reference name: %s", name)
			}
			includedNames[name]++
		} else {
			includedNames[name] = 1
		}
		err := t.ExecuteTemplate(&buf, name, data)
		includedNames[name]--
		strData := buf.String()
		return strData, err
	}
}
