# Model Package

This package contains the domain models for the inventory API, implementing Domain-Driven Design (DDD) principles with a focus on immutability, validation, and maintainability.

## Design Principles

### 1. Domain-Driven Design (DDD) Value Objects

Both `CommonRepresentation` and `ReporterRepresentation` are implemented as **immutable value objects** following DDD principles:

```go
// CommonRepresentation is an immutable value object representing common resource data.
// It follows DDD principles where value objects are immutable and should be created
// through factory methods that enforce validation rules.
// Note: Fields are exported for GORM compatibility but should not be modified directly.
type CommonRepresentation struct {
    // ... fields
}
```

**Why this approach:**
- **Immutability**: Prevents accidental state changes and ensures data integrity
- **Value semantics**: Objects are compared by value, not identity
- **Domain modeling**: Represents business concepts accurately
- **Thread safety**: Immutable objects are inherently thread-safe

### 2. Factory Method Pattern

All models use factory methods instead of direct struct initialization:

```go
func NewCommonRepresentation(
    resourceId uuid.UUID,
    data JsonObject,
    resourceType string,
    version uint,
    reportedByReporterType string,
    reportedByReporterInstance string,
) (*CommonRepresentation, error)
```

**Why factory methods:**
- **Validation enforcement**: Ensures all instances are valid at creation time
- **Encapsulation**: Hides complex initialization logic
- **Consistency**: Guarantees proper object construction
- **Error handling**: Provides clear feedback on validation failures
- **"Tell, Don't Ask" principle**: Client code tells the factory what to create, doesn't ask how

### 3. Exported Fields with Usage Guidelines

Fields are exported for GORM compatibility but with clear usage guidelines:

```go
// Note: Fields are exported for GORM compatibility but should not be modified directly.
type CommonRepresentation struct {
    ResourceId uuid.UUID `gorm:"type:text;column:id;primary_key"`
    // ... other fields
}
```

**Why this approach:**
- **GORM compatibility**: ORM requires exported fields for reflection
- **Pragmatic trade-off**: Balances immutability ideals with technical constraints
- **Clear documentation**: Comments explain intended usage
- **Validation at boundaries**: Factory methods ensure validity

### 4. Comprehensive Validation

Each model has a dedicated validation function used by both factory methods and tests:

```go
func ValidateCommonRepresentation(cr *CommonRepresentation) error {
    if cr.ResourceId == uuid.Nil {
        return ValidationError{Field: "ResourceId", Message: "cannot be empty"}
    }
    // ... more validations
}
```

**Why comprehensive validation:**
- **Data integrity**: Ensures all business rules are enforced
- **Early error detection**: Catches issues at creation time
- **Consistent validation**: Same rules applied everywhere
- **Clear error messages**: Specific feedback for each validation failure
- **Testability**: Validation logic is easily unit tested

### 5. Constants Instead of Magic Numbers

All magic numbers have been replaced with named constants:

```go
// constants.go
const (
    MaxFieldSize128 = 128 // For most string fields like IDs, types, etc.
    MaxFieldSize512 = 512 // For URL fields like APIHref, ConsoleHref
    MinVersionValue = 1   // Version must be positive (> 0)
)

// Usage in validation
if len(cr.ResourceType) > MaxResourceTypeLength {
    return ValidationError{
        Field: "ResourceType", 
        Message: fmt.Sprintf("exceeds maximum length of %d characters", MaxResourceTypeLength)
    }
}
```

**Why constants over magic numbers:**
- **Single source of truth**: All limits defined in one place
- **Easy maintenance**: Change a constant and all related code updates
- **Consistent validation**: Same limits used in struct tags and validation
- **Better error messages**: Dynamic messages showing actual limits
- **Code readability**: Meaningful names instead of cryptic numbers

### 6. Composite Primary Keys

Models use composite primary keys for versioning and uniqueness:

```go
// CommonRepresentation uses ResourceId + Version
ResourceId uuid.UUID `gorm:"type:text;column:id;primary_key"`
Version    uint      `gorm:"type:bigint;column:version;primary_key;check:version > 0"`

// ReporterRepresentation uses a 6-field composite unique index
LocalResourceID    string `gorm:"size:128;column:local_resource_id;index:reporter_rep_unique_idx,unique"`
ReporterType       string `gorm:"size:128;column:reporter_type;index:reporter_rep_unique_idx,unique"`
// ... more fields in the same unique index
```

**Why composite keys:**
- **Versioning support**: Allows multiple versions of the same resource
- **Business logic modeling**: Reflects real-world uniqueness constraints
- **Data integrity**: Database-level enforcement of business rules
- **Query optimization**: Efficient lookups on common access patterns

### 7. Nullable vs Non-Nullable Fields

Strategic use of pointers for optional fields:

```go
type ReporterRepresentation struct {
    APIHref         string  `gorm:"size:512;column:api_href"`          // Required, empty string allowed
    ConsoleHref     *string `gorm:"size:512;column:console_href"`      // Optional, can be nil
    ReporterVersion *string `gorm:"size:128;column:reporter_version"`  // Optional, can be nil
}
```

**Why this approach:**
- **Clear intent**: Pointers indicate optional fields
- **Database compatibility**: NULL vs empty string distinction
- **Validation clarity**: Different rules for required vs optional fields
- **API design**: Reflects business requirements accurately

### 8. URL Validation

Special validation for URL fields:

```go
if rr.APIHref != "" {
    if len(rr.APIHref) > MaxAPIHrefLength {
        return ValidationError{Field: "APIHref", Message: fmt.Sprintf("exceeds maximum length of %d characters", MaxAPIHrefLength)}
    }
    if err := validateURL(rr.APIHref); err != nil {
        return ValidationError{Field: "APIHref", Message: err.Error()}
    }
}
```

**Why URL validation:**
- **Data quality**: Ensures URLs are well-formed
- **API reliability**: Prevents invalid URLs from causing downstream issues
- **User experience**: Clear feedback on malformed URLs
- **Security**: Basic protection against malformed input

### 9. Whitespace Handling

Explicit whitespace validation for string fields:

```go
if rr.LocalResourceID == "" || strings.TrimSpace(rr.LocalResourceID) == "" {
    return ValidationError{Field: "LocalResourceID", Message: "cannot be empty"}
}
```

**Why whitespace handling:**
- **Data quality**: Prevents whitespace-only values
- **User experience**: Clear validation of meaningful input
- **Consistency**: Uniform handling across all string fields
- **Business logic**: Reflects real-world requirements

### 10. Cross-Database Compatibility

GORM tags designed for multiple database backends:

```go
ResourceId uuid.UUID `gorm:"type:text;column:id;primary_key"`  // Works with PostgreSQL and SQLite
Version    uint      `gorm:"type:bigint;column:version;primary_key;check:version > 0"`
```

**Why cross-database compatibility:**
- **Testing flexibility**: SQLite for tests, PostgreSQL for production
- **Deployment options**: Support for different environments
- **Migration safety**: Consistent behavior across databases
- **Development efficiency**: Local development with SQLite

## File Structure

```
internal/biz/model/
├── constants.go                    # All magic number constants
├── gorm_helpers.go                # Helper functions for GORM tags
├── common_representation.go       # CommonRepresentation model
├── reporter_representation.go     # ReporterRepresentation model
├── base_representation.go         # Shared base struct
├── validation_error.go            # Custom error types
└── README.md                      # This documentation
```

## Usage Examples

### Creating a Valid Instance

```go
// Factory method with validation
cr, err := NewCommonRepresentation(
    uuid.New(),
    JsonObject{"workspace_id": "test-workspace"},
    "host",
    1,
    "hbi",
    "test-instance",
)
if err != nil {
    // Handle validation error
    log.Printf("Validation failed: %v", err)
}
```

### Validation Error Handling

```go
// Custom error type provides structured feedback
if err != nil {
    if validationErr, ok := err.(*ValidationError); ok {
        log.Printf("Field %s: %s", validationErr.Field, validationErr.Message)
    }
}
```

### Using Constants

```go
// Instead of magic numbers
if len(field) > MaxFieldSize128 {
    return fmt.Errorf("field too long, max %d characters", MaxFieldSize128)
}
```

## Testing Strategy

The models are thoroughly tested with:

- **Factory method tests**: Verify valid instance creation and validation
- **Edge case tests**: Unicode characters, maximum lengths, boundary values
- **GORM tag tests**: Ensure proper database mapping
- **Serialization tests**: JSON marshalling/unmarshalling
- **Business rule tests**: Domain-specific validation logic

## Benefits of This Approach

1. **Type Safety**: Compile-time guarantees about data structure
2. **Data Integrity**: Runtime validation ensures business rules
3. **Maintainability**: Clear separation of concerns and single responsibility
4. **Testability**: Easy to unit test with predictable behavior
5. **Documentation**: Self-documenting code with clear intent
6. **Performance**: Immutable objects enable optimizations
7. **Consistency**: Uniform patterns across all models
8. **Error Handling**: Structured error reporting with clear messages

This design balances theoretical purity with practical constraints, providing a robust foundation for the inventory API's domain models while maintaining compatibility with Go's ecosystem and GORM's requirements. 