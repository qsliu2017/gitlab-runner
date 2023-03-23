package common

import (
	"encoding/json"
	"sync/atomic"

	jsonschema_generator "github.com/invopop/jsonschema"
	jsonschema_validator "github.com/santhosh-tekuri/jsonschema/v5"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
)

var configSchema *jsonschema_validator.Schema

func init() {
	defer func() {
		if r := recover(); r != nil {
			// Config validation is best-effort
			logrus.Warningf("Something went wrong creating config schema: %v", r)
		}
	}()

	r := &jsonschema_generator.Reflector{
		RequiredFromJSONSchemaTags: true,
	}
	schema, err := json.Marshal(r.Reflect(&Config{}))
	if err != nil {
		panic(err)
	}
	configSchema = jsonschema_validator.MustCompileString("config_schema.json", string(schema))
}

func Validate(config *Config) error {
	defer func() {
		if r := recover(); r != nil {
			// Config validation is best-effort
			logrus.Warningf("Something went wrong validating config: %v", r)
		}
	}()

	// Validation must be done on generic types so we re-unmarshal the config into an interface{}
	configString, err := json.Marshal(config)
	if err != nil {
		panic(err)
	}
	var jsonValue interface{}
	err = json.Unmarshal(configString, &jsonValue)
	if err != nil {
		panic(err)
	}

	return configSchema.Validate(jsonValue)
}

var (
	validateConfigFlagName string = "validate-config"
	validateConfigEnabled  atomic.Bool
)

func ConfigValidationEnabled() bool {
	return validateConfigEnabled.Load()
}

func ConfigureValidation(app *cli.App) {
	app.Flags = append(app.Flags, cli.BoolFlag{
		Name:  validateConfigFlagName,
		Usage: "Enable config validation",
	})

	appBefore := app.Before
	app.Before = func(cliCtx *cli.Context) error {
		if cliCtx.Bool(validateConfigFlagName) {
			validateConfigEnabled.Store(true)
		}
		if appBefore != nil {
			return appBefore(cliCtx)
		}
		return nil
	}
}
