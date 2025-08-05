package internal

// =============================================================================
// Utility Helper Functions
// =============================================================================

// stringPtr returns a pointer to the given string
func StringPtr(s string) *string {
	return &s
}

type JsonObject map[string]interface{}
