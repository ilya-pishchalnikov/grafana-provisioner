package config

import (
	"bytes"
	"fmt"
	"os"
	"reflect"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/go-viper/mapstructure/v2"
	"github.com/joho/godotenv"
	"github.com/spf13/viper"
)

// DurationAlias allows parsing duration from YAML/config
type Duration struct {
	time.Duration
}

// AppConfig is the root structure containing all application configuration
type AppConfig struct {
	Log         LogConfig      `mapstructure:"log"`
	Grafana     GrafanaConfig  `mapstructure:"grafana" validate:"required"`
	Folders     []FolderConfig `mapstructure:"folders"`
	DataSources []DataSource   `mapstructure:"datasources"`
	Dashboards  []Dashboard    `mapstructure:"dashboards"`
}

// LogConfig defines logging parameters
type LogConfig struct {
	Level  string `mapstructure:"level" validate:"oneof=debug info warn error"`  // debug, info, warn, error
	Format string `mapstructure:"format" validate:"oneof=debug json text"` // json, text
	File   string `mapstructure:"file"`
}

// DbConnectionConfig defines grafana folder parameters
type FolderConfig struct {
	Name string `mapstructure:"name" validate:"required"`
}

// Dashboard defines parameters of grafana dashboard
type Dashboard struct {
	Name       string `mapstructure:"name" validate:"required"`
	Folder     string `mapstructure:"folder"`
	File       string `mapstructure:"file" validate:"required"`
	DataSource string `mapstructure:"datasource"`
	ImportVar  string `mapstructure:"import-var"`
}

// Datasource defines parameters of grafana datasource
type DataSource struct {
	Name      string `mapstructure:"name" validate:"required"`
    Host     string `mapstructure:"host" validate:"required"`
    Port     int    `mapstructure:"port" validate:"required,min=1,max=65535"`
    User     string `mapstructure:"user" validate:"required"`
    Password string `mapstructure:"password" validate:"required"`
    DbName   string `mapstructure:"dbname" validate:"required"`
    SslMode  string `mapstructure:"sslmode" validate:"oneof=disable require verify-ca verify-full"`
}

// GrafanaConfig defines parameters for Grafana API client and provisioning
type GrafanaConfig struct {
	URL            string        `mapstructure:"url" validate:"required"`
	Token          string        `mapstructure:"token" validate:"required"`
	Timeout        Duration      `mapstructure:"timeout" validate:"gt=0"`
	Retries        int           `mapstructure:"retries" validate:"gt=0"`
	RetryDelay     Duration      `mapstructure:"retry-delay" validate:"gt=0"`
}


// customDurationHook is a mapstructure hook for parsing time strings
func customDurationHook() mapstructure.DecodeHookFunc {
	return func(f reflect.Type, t reflect.Type, data interface{}) (interface{}, error) {
		if t != reflect.TypeOf(Duration{}) {
			return data, nil
		}
		if f.Kind() != reflect.String {
			return data, nil
		}
		d, err := time.ParseDuration(data.(string))
		if err != nil {
			return nil, err
		}
		return Duration{Duration: d}, nil
	}
}

// durationValueRetriever is a custom function to help validator/v10
// understand how to get the value from our custom 'Duration' type.
// It extracts the embedded time.Duration for validation.
func durationValueRetriever(field reflect.Value) interface{} {
	if field.Type() == reflect.TypeOf(Duration{}) {
		// Return the embedded time.Duration value, which is an int64 (nanoseconds)
		return field.Field(0).Interface()
	}
	return nil
}


// Load reads and parses the configuration file
func Load(configPath string) (*AppConfig, error) {
	// Load environment variables from .env file (if present)
	if err := godotenv.Load(); err != nil {
		fmt.Println("INFO: .env file not found, using system environment variables for secrets")
	}

	// Read raw file
	rawContent, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file '%s': %w", configPath, err)
	}

	// Expand environment variables of format ${VAR}
	expandedContent := os.ExpandEnv(string(rawContent))

	// Initialize Viper
	v := viper.New()
	v.SetConfigType("yaml")
	if err := v.ReadConfig(bytes.NewBufferString(expandedContent)); err != nil {
		return nil, fmt.Errorf("failed to parse configuration: %w", err)
	}

	// New (Working) configuration:
	var cfg AppConfig

	// Define the DecodeHook to handle string-to-Duration conversion
	durationHook := mapstructure.ComposeDecodeHookFunc(customDurationHook())

	decoderConfig := mapstructure.DecoderConfig{
		WeaklyTypedInput: true,
		Result:           &cfg,
		// Add the custom DecodeHook
		DecodeHook: durationHook,
	}

	decoder, err := mapstructure.NewDecoder(&decoderConfig)
	if err!=nil {
		return nil, fmt.Errorf("failed to add duration decoder: %w", err)
	}

	if err := decoder.Decode(v.AllSettings()); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	validate := validator.New()

	validate.RegisterCustomTypeFunc(durationValueRetriever, Duration{})

	if err := validate.Struct(cfg); err != nil {
		return nil, fmt.Errorf("config validation error: %w", err)
	}

	return &cfg, nil
}