package cli

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"reflect"
	"regexp"
	"strings"
	"time"
	"unicode"

	"github.com/ake-persson/mapslice-json"
	"github.com/urfave/cli/v2"
	altsrc "github.com/urfave/cli/v2/altsrc"
)

// Invoked before normal cli parsing, adds flags from config struct if available
func (b *Builder) preConfig() {
	if b.config == nil {
		return
	}
	b.app.Flags = append(b.app.Flags, StringFlag("config-file", "To specify which configuration to be used"))
	b.app.Flags = append(b.app.Flags, BooleanFlag("dump-config", "Dumps configuration to file"))
	b.app.Flags = append(b.app.Flags, BooleanFlag("show-config", "Shows the loaded configuration"))
	valueOfConfig := reflect.ValueOf(b.config)
	if valueOfConfig.Kind() == reflect.Ptr {
		valueOfConfig = valueOfConfig.Elem()
	}
	if valueOfConfig.Type().Kind() != reflect.Struct {
		panic("config is not a non pointer struct")
	}
	b.preConfigRecursiveScan(valueOfConfig, "")
}

// Extracts struct fields into flags
func (b *Builder) preConfigRecursiveScan(valueOfStruct reflect.Value, prefix string) {
	if valueOfStruct.Kind() == reflect.Ptr {
		valueOfStruct = valueOfStruct.Elem()
	}
	typeOfStruct := valueOfStruct.Type()
	for i := 0; i < typeOfStruct.NumField(); i++ {
		fieldOfField := typeOfStruct.Field(i)
		valueOfField := valueOfStruct.Field(i)
		aliases := aliases(fieldOfField.Tag.Lookup("flag"))
		flagName := fieldOfField.Name
		if len(aliases) > 0 {
			flagName = aliases[0]
			aliases = aliases[1:]
		}
		if len(prefix) > 0 {
			flagName = fmt.Sprintf("%s-%s", prefix, flagName)
		}
		envName := env(flagName)
		flagName = dash(flagName)
		switch valueOfField.Kind() {
		case reflect.Struct:
			switch valueOfField.Interface().(type) {
			case time.Time:
				b.app.Flags = append(b.app.Flags, altsrc.NewStringFlag(&cli.StringFlag{
					Name:    flagName,
					EnvVars: envVars(envName, fieldOfField.Tag.Get("env")),
					Value:   valueOfField.String(),
					Aliases: aliases,
					Usage:   fieldOfField.Tag.Get("help"),
				}))
				b.configStructure = append(b.configStructure, mapslice.MapItem{
					Key:   flagName,
					Value: valueOfField.String(),
				})
			default:
				b.preConfigRecursiveScan(valueOfField, flagName)
			}
		case reflect.Int:
			b.app.Flags = append(b.app.Flags, altsrc.NewIntFlag(&cli.IntFlag{
				Name:    flagName,
				EnvVars: envVars(envName, fieldOfField.Tag.Get("env")),
				Value:   int(valueOfField.Int()),
				Aliases: aliases,
				Usage:   fieldOfField.Tag.Get("help"),
			}))
			b.configStructure = append(b.configStructure, mapslice.MapItem{
				Key:   flagName,
				Value: int(valueOfField.Int()),
			})
		case reflect.String:
			b.app.Flags = append(b.app.Flags, altsrc.NewStringFlag(&cli.StringFlag{
				Name:    flagName,
				EnvVars: envVars(envName, fieldOfField.Tag.Get("env")),
				Value:   valueOfField.String(),
				Aliases: aliases,
				Usage:   fieldOfField.Tag.Get("help"),
			}))
			b.configStructure = append(b.configStructure, mapslice.MapItem{
				Key:   flagName,
				Value: valueOfField.String(),
			})
		case reflect.Bool:
			b.app.Flags = append(b.app.Flags, altsrc.NewBoolFlag(&cli.BoolFlag{
				Name:    flagName,
				EnvVars: envVars(envName, fieldOfField.Tag.Get("env")),
				Value:   valueOfField.Bool(),
				Aliases: aliases,
				Usage:   fieldOfField.Tag.Get("help"),
			}))
			b.configStructure = append(b.configStructure, mapslice.MapItem{
				Key:   flagName,
				Value: valueOfField.Bool(),
			})
		case reflect.Int64:
			if v, ok := valueOfField.Interface().(time.Duration); ok {
				b.app.Flags = append(b.app.Flags, altsrc.NewDurationFlag(&cli.DurationFlag{
					Name:    flagName,
					EnvVars: envVars(envName, fieldOfField.Tag.Get("env")),
					Value:   v,
					Aliases: aliases,
					Usage:   fieldOfField.Tag.Get("help"),
				}))
				b.configStructure = append(b.configStructure, mapslice.MapItem{
					Key:   flagName,
					Value: v.String(),
				})
			}
		case reflect.Slice:
			b.app.Flags = append(b.app.Flags, altsrc.NewStringSliceFlag(&cli.StringSliceFlag{
				Name:    flagName,
				EnvVars: envVars(envName, fieldOfField.Tag.Get("env")),
				Aliases: aliases,
				Usage:   fieldOfField.Tag.Get("help"),
			}))
			b.configStructure = append(b.configStructure, mapslice.MapItem{
				Key:   flagName,
				Value: []string{},
			})
		}
	}
}

// Invoked after normal cli parsing, parses flags into struct if available
func (b *Builder) postConfig(c *cli.Context) {
	if b.config == nil {
		return
	}
	valueOfConfig := reflect.ValueOf(b.config)
	if valueOfConfig.Type().Kind() != reflect.Struct {
		panic("config is not a non pointer struct")
	}
	p := reflect.New(reflect.TypeOf(b.config))
	p.Elem().Set(reflect.ValueOf(b.config))
	b.runner.config = p.Interface()
	valueOfConfig = reflect.ValueOf(b.runner.config)
	if valueOfConfig.Kind() == reflect.Ptr {
		valueOfConfig = valueOfConfig.Elem()
	}

	configFile := strings.TrimSpace(c.String("config-file"))
	if len(configFile) == 0 {
		configFile = "config.json"
	}
	c.Set("config-file", configFile)
	if _, err := os.Stat(configFile); err == nil {
		if err := altsrc.InitInputSourceWithContext(b.app.Flags, altsrc.NewJSONSourceFromFlagFunc("config-file"))(c); err != nil {
			if len(c.String("config-file")) > 0 {
				b.runner.Exit(err)
			}
		}
	}

	b.postConfigRecursiveScan(c, valueOfConfig, "")
	b.runner.config = reflect.ValueOf(b.runner.config).Elem().Interface()
	if c.Bool("show-config") {
		bts, _ := json.MarshalIndent(b.runner.flatConfig, "", "  ")
		fmt.Println(string(bts))
		os.Exit(0)
	} else if c.Bool("dump-config") {
		bts, _ := json.MarshalIndent(b.runner.flatConfig, "", "  ")
		ioutil.WriteFile(configFile, bts, 0644)
		os.Exit(0)
	}
}

// Extracts flags into config structure
func (b *Builder) postConfigRecursiveScan(c *cli.Context, valueOfStruct reflect.Value, prefix string) {
	if valueOfStruct.Kind() == reflect.Ptr {
		valueOfStruct = valueOfStruct.Elem()
	}
	typeOfStruct := valueOfStruct.Type()
	for i := 0; i < typeOfStruct.NumField(); i++ {
		fieldOfField := typeOfStruct.Field(i)
		valueOfField := valueOfStruct.Field(i)
		aliases := aliases(fieldOfField.Tag.Lookup("flag"))
		flagName := fieldOfField.Name
		if len(aliases) > 0 {
			flagName = aliases[0]
		}
		if len(prefix) > 0 {
			flagName = fmt.Sprintf("%s-%s", prefix, flagName)
		}
		flagName = dash(flagName)
		switch valueOfField.Kind() {
		case reflect.Struct:
			switch valueOfField.Interface().(type) {
			case time.Time:
				v := strings.TrimSpace(c.String(flagName))
				b.runner.flatConfig = append(b.runner.flatConfig, mapslice.MapItem{
					Key:   flagName,
					Value: c.String(flagName),
				})
				switch {
				case dateTimeRegexp.MatchString(v):
					valueOfField.Set(reflect.ValueOf(parseTime(dateTimeFormat, v)))
				case dateRegexp.MatchString(v):
					valueOfField.Set(reflect.ValueOf(parseTime(dateFormat, v)))
				case timeRegexp.MatchString(v):
					valueOfField.Set(reflect.ValueOf(parseTime(dateTimeFormat, fmt.Sprintf("%s %s", time.Now().Format(dateFormat), v))))
				default:
					os.Exit(1)
				}
			default:
				b.postConfigRecursiveScan(c, valueOfField, flagName)
			}
		case reflect.Int:
			valueOfField.SetInt(int64(c.Int(flagName)))
			b.runner.flatConfig = append(b.runner.flatConfig, mapslice.MapItem{
				Key:   flagName,
				Value: c.Int(flagName),
			})
		case reflect.String:
			valueOfField.SetString(c.String(flagName))
			b.runner.flatConfig = append(b.runner.flatConfig, mapslice.MapItem{
				Key:   flagName,
				Value: c.String(flagName),
			})
		case reflect.Bool:
			valueOfField.SetBool(c.Bool(flagName))
			b.runner.flatConfig = append(b.runner.flatConfig, mapslice.MapItem{
				Key:   flagName,
				Value: c.Bool(flagName),
			})
		case reflect.Int64:
			if _, ok := valueOfField.Interface().(time.Duration); ok {
				valueOfField.Set(reflect.ValueOf(c.Duration(flagName)))
				b.runner.flatConfig = append(b.runner.flatConfig, mapslice.MapItem{
					Key:   flagName,
					Value: c.Duration(flagName).String(),
				})
			}
		case reflect.Slice:
			var values []string
			for _, item := range c.StringSlice(flagName) {
				for _, value := range strings.Split(item, ",") {
					values = append(values, strings.TrimSpace(value))
				}
			}
			valueOfField.Set(reflect.ValueOf(values))
			b.runner.flatConfig = append(b.runner.flatConfig, mapslice.MapItem{
				Key:   flagName,
				Value: strings.Join(values, ","),
			})
		}
	}
}

var dateTimeRegexp = regexp.MustCompile(`^\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2}$`)
var dateTimeFormat = "2006-01-02 15:04:05"
var dateRegexp = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)
var dateFormat = "2006-01-02"
var timeRegexp = regexp.MustCompile(`^\d{2}:\d{2}:\d{2}$`)

func parseTime(layout string, timeString string) time.Time {
	t, _ := time.Parse(layout, timeString)
	return t
}

func dash(name string) string {
	parts := strings.Split(name, "-")
	var dashedName string
	for _, name := range parts {
		if len(dashedName) > 0 {
			dashedName += "-"
		}
		for i := 0; i < len(name); i++ {
			var char, nextChar, prevChar rune
			char = rune(name[i])
			if i > 0 {
				prevChar = rune(name[i-1])
			}
			if i < len(name)-1 {
				nextChar = rune(name[i+1])
			}
			switch {
			case unicode.IsUpper(char) && !unicode.IsUpper(nextChar) && nextChar != 0:
				fallthrough
			case prevChar != 0 && !unicode.IsUpper(prevChar) && unicode.IsUpper(char):
				if len(dashedName) > 0 {
					dashedName += "-"
				}
			}
			dashedName += string(unicode.ToLower(char))
		}
	}
	return strings.ReplaceAll(dashedName, "--", "-")
}

func env(name string) string {
	return strings.ToUpper(strings.ReplaceAll(dash(name), "-", "_"))
}

func aliases(val string, ok bool) (list []string) {
	if !ok {
		return list
	}
	for _, val := range strings.Split(val, ",") {
		list = append(list, strings.Trim(val, " \t"))
	}
	return list
}

func envVars(keyUnderscored string, optionalEnv string) (list []string) {
	keyUnderscored = strings.ToUpper(keyUnderscored)
	list = append(list, keyUnderscored)
	if len(optionalEnv) > 0 {
		for _, val := range strings.Split(optionalEnv, ",") {
			list = append(list, strings.ToUpper(strings.Trim(val, " \t")))
		}
	}
	return list
}
