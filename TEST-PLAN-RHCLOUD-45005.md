# Test Plan - RHCLOUD-45005
## Add All Required Tests First (Test-First Approach)

**Date:** 2026-04-27  
**Jira:** https://redhat.atlassian.net/browse/RHCLOUD-45005  
**Branch:** RHCLOUD-45005-add-move-logic-model-value-types  
**Approach:** Write ALL tests first, then implement changes in separate PRs

---

## Strategy

1. **Write tests first** - All tests documented below
2. **Tests may initially fail** - Expected for Area 3 (validation)
3. **Tests may call existing code** - For Area 2 (normalization), tests will call current data-layer function
4. **Run test suite** - Establish baseline (some tests will fail, that's OK)
5. **Implement in separate PRs** - With tests already in place

---

## Area 2: NormalizeResourceType - Additional Test Cases

**File:** `internal/biz/model/common_test.go`  
**Function under test:** Initially `internal/data/schema_inmemory.NormalizeResourceType`, will move to model

### Existing Tests (in data layer)
✅ Forward slash replacement: "rhel/host" → "rhel_host"  
✅ No slash: "k8s_cluster" → "k8s_cluster"  
✅ Multiple slashes: "org/team/resource" → "org_team_resource"  
✅ Empty string: "" → ""  
✅ Already normalized: "host" → "host"

### NEW Tests to Add

#### Test: Mixed Case Normalization
```go
func TestResourceType_Normalization_MixedCase(t *testing.T) {
    tests := []struct {
        name     string
        input    string
        expected string
    }{
        {
            name:     "uppercase to lowercase",
            input:    "RHEL/HOST",
            expected: "rhel_host",
        },
        {
            name:     "mixed case to lowercase",
            input:    "RhEl/HoSt",
            expected: "rhel_host",
        },
        {
            name:     "camelCase to lowercase",
            input:    "k8s/MyCluster",
            expected: "k8s_mycluster",
        },
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            // Initially: call data.NormalizeResourceType(tt.input)
            // After refactor: rt, _ := NewResourceType(tt.input); rt.String()
            result := data.NormalizeResourceType(tt.input)
            if result != tt.expected {
                t.Errorf("Expected %q, got %q", tt.expected, result)
            }
        })
    }
}
```

#### Test: Whitespace Handling with Normalization
```go
func TestResourceType_Normalization_Whitespace(t *testing.T) {
    tests := []struct {
        name     string
        input    string
        expected string
    }{
        {
            name:     "leading whitespace",
            input:    "  rhel/host",
            expected: "rhel_host", // After trim in constructor
        },
        {
            name:     "trailing whitespace",
            input:    "rhel/host  ",
            expected: "rhel_host",
        },
        {
            name:     "leading and trailing whitespace",
            input:    "  rhel/host  ",
            expected: "rhel_host",
        },
        {
            name:     "whitespace only",
            input:    "   ",
            expected: "", // Should become empty after trim
        },
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            input := strings.TrimSpace(tt.input)
            if input == "" {
                if tt.expected != "" {
                    t.Errorf("Expected empty result for whitespace-only input")
                }
                return
            }
            result := data.NormalizeResourceType(input)
            if result != tt.expected {
                t.Errorf("Expected %q, got %q", tt.expected, result)
            }
        })
    }
}
```

#### Test: Unicode and Special Characters
```go
func TestResourceType_Normalization_Unicode(t *testing.T) {
    tests := []struct {
        name        string
        input       string
        expected    string
        description string
    }{
        {
            name:        "unicode characters",
            input:       "café/host",
            expected:    "café_host", // Current behavior preserves unicode
            description: "Documents current unicode behavior",
        },
        {
            name:        "emoji in type",
            input:       "🚀/host",
            expected:    "🚀_host", // Current behavior
            description: "Documents emoji handling",
        },
        {
            name:        "multiple forward slashes",
            input:       "a//b///c",
            expected:    "a__b___c", // Each / becomes _
            description: "Multiple consecutive slashes",
        },
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            result := data.NormalizeResourceType(tt.input)
            if result != tt.expected {
                t.Errorf("Expected %q, got %q (%s)", tt.expected, result, tt.description)
            }
        })
    }
}
```

#### Test: Very Long Resource Type Names
```go
func TestResourceType_Normalization_LongNames(t *testing.T) {
    tests := []struct {
        name        string
        input       string
        shouldError bool // Will be true after validation is added
        description string
    }{
        {
            name:        "63 characters (at limit)",
            input:       strings.Repeat("a", 63),
            shouldError: false,
            description: "Just under proposed 64 char limit",
        },
        {
            name:        "64 characters (exact limit)",
            input:       strings.Repeat("a", 64),
            shouldError: false,
            description: "Exactly at proposed limit",
        },
        {
            name:        "65 characters (over limit)",
            input:       strings.Repeat("a", 65),
            shouldError: true, // After validation
            description: "One character over limit",
        },
        {
            name:        "very long with slashes",
            input:       strings.Repeat("a/", 40), // 80 chars
            shouldError: true, // After validation (even normalized would be 80 chars)
            description: "Long name with normalization",
        },
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            result := data.NormalizeResourceType(tt.input)
            // For now, just verify normalization works
            // After validation is added, check error cases
            if len(tt.input) < 65 {
                expected := strings.ToLower(strings.ReplaceAll(tt.input, "/", "_"))
                if result != expected {
                    t.Errorf("Expected %q, got %q", expected, result)
                }
            }
            // TODO: After validation, verify tt.shouldError cases return error
        })
    }
}
```

**Total new tests for Area 2:** ~15 test cases across 4 test functions

---

## Area 3: Validation Tests

### Constants to Define (in common.go or validation.go)

```go
const (
    maxResourceTypeLength       = 64
    maxReporterTypeLength       = 64
    maxReporterInstanceIdLength = 256
    maxLocalResourceIdLength    = 256
    maxConsistencyTokenLength   = 512
    maxTransactionIdLength      = 128
    maxLockIdLength             = 128
    maxLockTokenLength          = 512
)

var (
    // Resource types: lowercase alphanumeric + underscore only (after normalization)
    resourceTypeRegex = regexp.MustCompile(`^[a-z0-9_]+$`)
    
    // Reporter types: lowercase alphanumeric + underscore + dash
    reporterTypeRegex = regexp.MustCompile(`^[a-z0-9_-]+$`)
)
```

---

## Area 3.1: ResourceType Validation Tests

**File:** `internal/biz/model/resource_type_validation_test.go` (new file)

### Test 1: Max Length Validation
```go
func TestResourceType_MaxLength(t *testing.T) {
    t.Parallel()
    
    tests := []struct {
        name        string
        input       string
        expectError bool
        errorType   error
    }{
        {
            name:        "at max length (64 chars)",
            input:       strings.Repeat("a", 64),
            expectError: false,
        },
        {
            name:        "one over max length",
            input:       strings.Repeat("a", 65),
            expectError: true,
            errorType:   ErrInvalidFormat,
        },
        {
            name:        "far over max length",
            input:       strings.Repeat("a", 200),
            expectError: true,
            errorType:   ErrInvalidFormat,
        },
        {
            name:        "max length with valid characters",
            input:       strings.Repeat("a_", 32), // 64 chars total
            expectError: false,
        },
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            t.Parallel()
            
            rt, err := NewResourceType(tt.input)
            
            if tt.expectError {
                if err == nil {
                    t.Errorf("Expected error, got nil")
                }
                if tt.errorType != nil && !errors.Is(err, tt.errorType) {
                    t.Errorf("Expected error type %v, got %v", tt.errorType, err)
                }
            } else {
                if err != nil {
                    t.Errorf("Expected no error, got: %v", err)
                }
                if rt.String() == "" {
                    t.Errorf("Expected non-empty resource type")
                }
            }
        })
    }
}
```

### Test 2: Character Set Validation (After Normalization)
```go
func TestResourceType_CharacterSetValidation(t *testing.T) {
    t.Parallel()
    
    tests := []struct {
        name        string
        input       string
        expectError bool
        description string
    }{
        {
            name:        "valid lowercase alphanumeric",
            input:       "abc123",
            expectError: false,
        },
        {
            name:        "valid with underscores",
            input:       "a_b_c_123",
            expectError: false,
        },
        {
            name:        "slash normalized to underscore",
            input:       "rhel/host", // Becomes "rhel_host"
            expectError: false,
            description: "Forward slash should be normalized",
        },
        {
            name:        "uppercase normalized to lowercase",
            input:       "RHEL", // Becomes "rhel"
            expectError: false,
            description: "Uppercase should be normalized",
        },
        {
            name:        "invalid dash after normalization",
            input:       "abc-def",
            expectError: true, // Dashes not allowed
            description: "Dash should be rejected",
        },
        {
            name:        "invalid space",
            input:       "abc def",
            expectError: true,
            description: "Spaces should be rejected",
        },
        {
            name:        "invalid special characters",
            input:       "abc@def",
            expectError: true,
        },
        {
            name:        "invalid dot",
            input:       "abc.def",
            expectError: true,
        },
        {
            name:        "invalid colon",
            input:       "abc:def",
            expectError: true,
        },
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            t.Parallel()
            
            rt, err := NewResourceType(tt.input)
            
            if tt.expectError {
                if err == nil {
                    t.Errorf("Expected error for input %q, got nil", tt.input)
                }
            } else {
                if err != nil {
                    t.Errorf("Expected no error for input %q, got: %v", tt.input, err)
                }
                // Verify normalized form
                normalized := strings.ToLower(strings.ReplaceAll(tt.input, "/", "_"))
                if rt.String() != normalized {
                    t.Errorf("Expected normalized value %q, got %q", normalized, rt.String())
                }
            }
        })
    }
}
```

### Test 3: Control Characters
```go
func TestResourceType_ControlCharacters(t *testing.T) {
    t.Parallel()
    
    tests := []struct {
        name  string
        input string
    }{
        {name: "newline", input: "abc\ndef"},
        {name: "carriage return", input: "abc\rdef"},
        {name: "tab", input: "abc\tdef"},
        {name: "null byte", input: "abc\x00def"},
        {name: "vertical tab", input: "abc\vdef"},
        {name: "form feed", input: "abc\fdef"},
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            t.Parallel()
            
            _, err := NewResourceType(tt.input)
            if err == nil {
                t.Errorf("Expected error for control character input, got nil")
            }
        })
    }
}
```

### Test 4: Unicode Handling
```go
func TestResourceType_UnicodeHandling(t *testing.T) {
    t.Parallel()
    
    tests := []struct {
        name        string
        input       string
        expectError bool
        description string
    }{
        {
            name:        "unicode letters",
            input:       "café",
            expectError: true, // Should reject non-ASCII after normalization
            description: "Accented characters not in [a-z0-9_]",
        },
        {
            name:        "emoji",
            input:       "🚀rocket",
            expectError: true,
            description: "Emoji should be rejected",
        },
        {
            name:        "chinese characters",
            input:       "主机",
            expectError: true,
            description: "Non-Latin scripts should be rejected",
        },
        {
            name:        "ascii only",
            input:       "host123",
            expectError: false,
            description: "Pure ASCII should be accepted",
        },
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            t.Parallel()
            
            rt, err := NewResourceType(tt.input)
            
            if tt.expectError {
                if err == nil {
                    t.Errorf("Expected error for %s, got nil", tt.description)
                }
            } else {
                if err != nil {
                    t.Errorf("Expected no error for %s, got: %v", tt.description, err)
                }
                if rt.String() == "" {
                    t.Errorf("Expected non-empty resource type")
                }
            }
        })
    }
}
```

### Test 5: Edge Cases and Boundary Conditions
```go
func TestResourceType_EdgeCases(t *testing.T) {
    t.Parallel()
    
    tests := []struct {
        name        string
        input       string
        expectError bool
        expected    string
    }{
        {
            name:        "single character",
            input:       "a",
            expectError: false,
            expected:    "a",
        },
        {
            name:        "single underscore",
            input:       "_",
            expectError: false,
            expected:    "_",
        },
        {
            name:        "single digit",
            input:       "1",
            expectError: false,
            expected:    "1",
        },
        {
            name:        "starts with number",
            input:       "1host",
            expectError: false,
            expected:    "1host",
        },
        {
            name:        "starts with underscore",
            input:       "_host",
            expectError: false,
            expected:    "_host",
        },
        {
            name:        "multiple underscores",
            input:       "a__b__c",
            expectError: false,
            expected:    "a__b__c",
        },
        {
            name:        "normalization creates long name",
            input:       strings.Repeat("a/", 33), // 66 chars → 66 chars after norm
            expectError: true, // Over 64 char limit
        },
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            t.Parallel()
            
            rt, err := NewResourceType(tt.input)
            
            if tt.expectError {
                if err == nil {
                    t.Errorf("Expected error, got nil")
                }
            } else {
                if err != nil {
                    t.Errorf("Expected no error, got: %v", err)
                }
                if tt.expected != "" && rt.String() != tt.expected {
                    t.Errorf("Expected %q, got %q", tt.expected, rt.String())
                }
            }
        })
    }
}
```

**Total tests for ResourceType validation:** ~40 test cases

---

## Area 3.2: TransactionId Validation Tests

**File:** `internal/biz/model/transaction_id_validation_test.go` (new file)

**Current state:** Constructor exists but has TODO comment about validation

### Test 1: Empty and Whitespace
```go
func TestTransactionId_EmptyValidation(t *testing.T) {
    t.Parallel()
    
    tests := []struct {
        name        string
        input       string
        expectError bool
    }{
        {
            name:        "empty string",
            input:       "",
            expectError: true,
        },
        {
            name:        "whitespace only",
            input:       "   ",
            expectError: true,
        },
        {
            name:        "tab only",
            input:       "\t",
            expectError: true,
        },
        {
            name:        "valid non-empty",
            input:       "txn123",
            expectError: false,
        },
        {
            name:        "leading whitespace trimmed",
            input:       "  txn123",
            expectError: false,
        },
        {
            name:        "trailing whitespace trimmed",
            input:       "txn123  ",
            expectError: false,
        },
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            t.Parallel()
            
            txnId, err := NewTransactionId(tt.input)
            
            if tt.expectError {
                if err == nil {
                    t.Errorf("Expected error, got nil")
                }
                if !errors.Is(err, ErrEmpty) {
                    t.Errorf("Expected ErrEmpty, got %v", err)
                }
            } else {
                if err != nil {
                    t.Errorf("Expected no error, got: %v", err)
                }
                if txnId.String() == "" {
                    t.Errorf("Expected non-empty transaction ID")
                }
            }
        })
    }
}
```

### Test 2: Max Length
```go
func TestTransactionId_MaxLength(t *testing.T) {
    t.Parallel()
    
    tests := []struct {
        name        string
        input       string
        expectError bool
    }{
        {
            name:        "at max length (128 chars)",
            input:       strings.Repeat("a", 128),
            expectError: false,
        },
        {
            name:        "one over max",
            input:       strings.Repeat("a", 129),
            expectError: true,
        },
        {
            name:        "far over max",
            input:       strings.Repeat("a", 1000),
            expectError: true,
        },
        {
            name:        "typical UUID length",
            input:       "550e8400-e29b-41d4-a716-446655440000", // 36 chars
            expectError: false,
        },
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            t.Parallel()
            
            _, err := NewTransactionId(tt.input)
            
            if tt.expectError {
                if err == nil {
                    t.Errorf("Expected error for length %d, got nil", len(tt.input))
                }
            } else {
                if err != nil {
                    t.Errorf("Expected no error for length %d, got: %v", len(tt.input), err)
                }
            }
        })
    }
}
```

### Test 3: Control Characters
```go
func TestTransactionId_ControlCharacters(t *testing.T) {
    t.Parallel()
    
    tests := []struct {
        name  string
        input string
    }{
        {name: "newline", input: "txn\n123"},
        {name: "carriage return", input: "txn\r123"},
        {name: "tab", input: "txn\t123"},
        {name: "null byte", input: "txn\x00123"},
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            t.Parallel()
            
            _, err := NewTransactionId(tt.input)
            if err == nil {
                t.Errorf("Expected error for control character, got nil")
            }
        })
    }
}
```

**Total tests for TransactionId:** ~15 test cases

---

## Area 3.3: ConsistencyToken Validation Tests

**File:** `internal/biz/model/consistency_token_validation_test.go` (new file)

### Test: Max Length
```go
func TestConsistencyToken_MaxLength(t *testing.T) {
    t.Parallel()
    
    tests := []struct {
        name        string
        input       string
        expectError bool
    }{
        {
            name:        "at max length (512 chars)",
            input:       strings.Repeat("a", 512),
            expectError: false,
        },
        {
            name:        "one over max",
            input:       strings.Repeat("a", 513),
            expectError: true,
        },
        {
            name:        "typical token length",
            input:       strings.Repeat("a", 100),
            expectError: false,
        },
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            t.Parallel()
            
            _, err := NewConsistencyToken(tt.input)
            
            if tt.expectError {
                if err == nil {
                    t.Errorf("Expected error, got nil")
                }
            } else {
                if err != nil {
                    t.Errorf("Expected no error, got: %v", err)
                }
            }
        })
    }
}
```

**Total tests for ConsistencyToken:** ~10 test cases

---

## Area 3.4: ReporterType Validation Tests

**File:** `internal/biz/model/reporter_type_validation_test.go` (new file)

Similar structure to ResourceType but allows dashes:
- Max length: 64 chars
- Regex: `^[a-z0-9_-]+$`
- ~30 test cases

---

## Area 3.5: ReporterInstanceId Validation Tests

**File:** `internal/biz/model/reporter_instance_id_validation_test.go` (new file)

- Max length: 256 chars
- No control characters
- ~20 test cases

---

## Area 3.6: LocalResourceId Validation Tests

**File:** `internal/biz/model/local_resource_id_validation_test.go` (new file)

- Max length: 256 chars
- URL-safe characters
- ~20 test cases

---

## Area 3.7: LockId and LockToken Validation Tests

**File:** `internal/biz/model/lock_validation_test.go` (new file)

- LockId max length: 128 chars
- LockToken max length: 512 chars
- ~15 test cases total

---

## Error Types Needed

Add to `internal/biz/model/errors.go` (or create if doesn't exist):

```go
var (
    ErrEmpty         = errors.New("value cannot be empty")
    ErrInvalidFormat = errors.New("invalid format")
    ErrInvalidUUID   = errors.New("invalid UUID")
    ErrExceedsMaxLength = errors.New("exceeds maximum length")
    ErrInvalidCharacters = errors.New("contains invalid characters")
    ErrControlCharacters = errors.New("contains control characters")
)
```

---

## Test Execution Plan

### Phase 1: Write All Tests (THIS PR)

1. Create test files:
   - `common_test.go` - add normalization tests
   - `resource_type_validation_test.go`
   - `transaction_id_validation_test.go`
   - `consistency_token_validation_test.go`
   - `reporter_type_validation_test.go`
   - `reporter_instance_id_validation_test.go`
   - `local_resource_id_validation_test.go`
   - `lock_validation_test.go`

2. Run tests - **EXPECTED FAILURES:**
   - ✅ Normalization tests: PASS (call existing function)
   - ❌ Validation tests: FAIL (validation not yet implemented)

3. Document baseline:
   ```bash
   make test > test-baseline-before-implementation.txt
   ```

4. Commit all tests:
   ```
   git add internal/biz/model/*_test.go
   git commit -m "Add comprehensive validation tests for RHCLOUD-45005

   - Add normalization edge case tests (Area 2)
   - Add validation tests for all model types (Area 3)
   - Tests document expected behavior
   - Some tests will fail until validation is implemented
   
   Related to RHCLOUD-45005"
   ```

### Phase 2: Implement (Separate PRs)

- **PR 1:** Move normalization to model (tests should all pass)
- **PR 2:** Implement validation for high-priority types (tests turn green)
- **PR 3:** Implement validation for remaining types (tests turn green)

---

## Summary of Test Count

| Area | Test Cases | Files |
|------|-----------|-------|
| Area 2: Normalization | ~15 | 1 (common_test.go) |
| Area 3.1: ResourceType | ~40 | 1 |
| Area 3.2: TransactionId | ~15 | 1 |
| Area 3.3: ConsistencyToken | ~10 | 1 |
| Area 3.4: ReporterType | ~30 | 1 |
| Area 3.5: ReporterInstanceId | ~20 | 1 |
| Area 3.6: LocalResourceId | ~20 | 1 |
| Area 3.7: Locks | ~15 | 1 |
| **TOTAL** | **~165** | **8 new/modified files** |

---

## Next Steps

1. ✅ Review this test plan
2. Start writing tests in order:
   - Start with Area 2 (normalization) - easier, builds confidence
   - Then Area 3.1-3.2 (high priority)
   - Then Area 3.3-3.7 (remaining)
3. Run tests and document failures
4. Commit all tests
5. Begin implementation in separate PRs

---

## Notes

- **All tests use table-driven approach** for clarity and maintainability
- **All tests are parallel** (t.Parallel()) for speed
- **Tests document expected behavior** even before implementation
- **Some tests call existing code** (normalization tests)
- **Some tests will fail** (validation tests) - that's expected and correct
- **Error types must be checked** with errors.Is() not direct comparison
