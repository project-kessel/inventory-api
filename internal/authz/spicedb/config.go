package spicedb

// Config holds the configuration for SpiceDB client
type Config struct {
	Endpoint        string
	Token           string
	TokenFile       string
	SchemaFile      string
	UseTLS          bool
	FullyConsistent bool
}
