package config

import (
	"bytes"
	"fmt"
	"os"
	"reflect"
	"time"

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
	Log       LogConfig          `mapstructure:"log"`
	Grafana   GrafanaConfig      `mapstructure:"grafana"`
	MetricsDB DbConnectionConfig `mapstructure:"metrics-db"`
}

// LogConfig defines logging parameters
type LogConfig struct {
	Level  string `mapstructure:"level"`  // debug, info, warn, error
	Format string `mapstructure:"format"` // json, text
}

// DbConnectionConfig defines database connection parameters for Grafana DataSource
type DbConnectionConfig struct {
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	User     string `mapstructure:"user"`
	Password string `mapstructure:"password"`
	DbName   string `mapstructure:"dbname"`
	SslMode  string `mapstructure:"sslmode"`
}

// Dashboard defines parameters of grafana dashboard
type Dashboard struct {
	Name      string `mapstructure:"name"`
	File      string `mapstructure:"file"`
	ImportVar string `mapstructure:"import-var"`
}

// Dashboard defines parameters of grafana datasource
type DataSource struct {
	Name      string `mapstructure:"name"`
}

// GrafanaConfig defines parameters for Grafana API client and provisioning
type GrafanaConfig struct {
	URL            string        `mapstructure:"url"`
	Token          string        `mapstructure:"token"`
	Timeout        Duration      `mapstructure:"timeout"`
	Retries        int           `mapstructure:"retries"`
	RetryDelay     Duration      `mapstructure:"retry-delay"`
	Dashboard      Dashboard     `mapstructure:"dashboard"`
	DataSource     DataSource    `mapstructure:"datasource"`
}


func (d *Duration) Decode(value interface{}) error {
	var s string
	switch v := value.(type) {
	case string:
		s = v
	default:
		return fmt.Errorf("invalid type for Duration: %T", value)
	}

	dur, err := time.ParseDuration(s)
	if err != nil {
		return err
	}
	d.Duration = dur
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
	durationHook := mapstructure.ComposeDecodeHookFunc(
		mapstructure.StringToTimeDurationHookFunc(), // This is useful but not strictly needed for your custom type
		func(f reflect.Type, t reflect.Type, data interface{}) (interface{}, error) {
			// Check if the source is a string and the target is the config.Duration type
			if f.Kind() == reflect.String && t == reflect.TypeOf(Duration{}) {
				d := Duration{}
				if err := d.Decode(data); err != nil {
					return nil, err
				}
				return d, nil
			}
			return data, nil
		},
	)

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

	// Validate essential settings
	if cfg.Grafana.Token == "" {
		return nil, fmt.Errorf("grafana.token is required (e.g., GF_ADMIN_TOKEN)")
	}

	return &cfg, nil
}