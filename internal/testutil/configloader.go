package testutil

import (
	"os"
	"strings"

	"github.com/gostratum/core/configx"
	"github.com/spf13/viper"
)

// NewConfigLoader returns a configx.Loader backed by a viper instance preloaded
// with the provided YAML. This is a test helper to emulate application loader
// behavior (including env bindings).
func NewConfigLoader(configYAML string) (configx.Loader, error) {
	v := viper.New()
	v.SetConfigType("yaml")
	// Mirror core/configx viper defaults so test loader behaves similarly.
	v.SetEnvPrefix("STRATUM")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_", "-", "_"))
	v.AutomaticEnv()
	if err := v.ReadConfig(strings.NewReader(configYAML)); err != nil {
		return nil, err
	}

	return &viperLoaderWrapper{v: v}, nil
}

// NewViper returns a fresh viper instance for tests with default settings.
func NewViper() *viper.Viper {
	return viper.New()
}

// viperLoaderWrapper adapts viper to configx.Loader for tests
type viperLoaderWrapper struct {
	v *viper.Viper
}

func (w *viperLoaderWrapper) Bind(c configx.Configurable) error {
	return w.v.UnmarshalKey(c.Prefix(), c)
}

func (w *viperLoaderWrapper) BindEnv(key string, envVars ...string) error {
	if len(envVars) == 0 {
		return w.v.BindEnv(key)
	}
	// Bind with explicit env var names
	if err := w.v.BindEnv(append([]string{key}, envVars...)...); err != nil {
		return err
	}

	// If any of the provided env vars are already set, write the value into viper
	// so UnmarshalKey will pick it up immediately.
	for _, env := range envVars {
		if val, ok := os.LookupEnv(env); ok {
			w.v.Set(key, val)
			// also set with module prefix so UnmarshalKey("db", ...) can find it
			w.v.Set("db."+key, val)
			break
		}
	}
	return nil
}
