package config

// Config contains application-level settings shared by runtime modules.
type Config struct {
	Enabled  bool
	RouterID string
	LogLevel string
}
