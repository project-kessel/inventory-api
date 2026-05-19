# Test Plan - Category 1: Missing Coverage for EXISTING Code
## RHCLOUD-45005 - Add tests that should PASS now

**Date:** 2026-04-28  
**Jira:** https://redhat.atlassian.net/browse/RHCLOUD-45005  
**Branch:** RHCLOUD-45005-add-move-logic-model-value-types  
**Goal:** Add missing test coverage for existing code (all tests should PASS)

---

## Overview

These tests document and verify CURRENT behavior before any refactoring. All tests should pass immediately.

**NOT included here:** New validation logic (max lengths, regex, character sets) - those are Category 2.

---

## Area 1: NormalizeResourceType - Missing Edge Cases

**Current location:** `internal/data/schema_inmemory.go:364`

**Existing tests:** `internal/data/schema_inmemory_test.go:605`
- ✅ Forward slash → underscore
- ✅ Multiple slashes
- ✅ Already normalized
- ✅ Empty string

### Missing Coverage (NEW tests to add)

#### File: `internal/data/schema_inmemory_test.go`

**Test 1: Case Normalization**
```go
func TestNormalizeResourceType_CaseHandling(t *testing.T) {
    tests := []struct {
        name     string
        input    string
        expected string
    }{
        {
            name:     "uppercase to lowercase",
            input:    "RHEL",
            expected: "rhel",
        },
        {
            name:     "mixed case to lowercase",
            input:    "RhEl",
            expected: "rhel",
        },
        {
            name:     "uppercase with slash",
            input:    "RHEL/HOST",
            expected: "rhel_host",
        },
        {
            name:     "mixed case with slash",
            input:    "RhEl/HoSt",
            expected: "rhel_host",
        },
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            result := NormalizeResourceType(tt.input)
            if result != tt.expected {
                t.Errorf("NormalizeResourceType(%q) = %q, want %q", tt.input, result, tt.expected)
            }
        })
    }
}
```

**Test 2: Multiple Consecutive Slashes**
```go
func TestNormalizeResourceType_MultipleSlashes(t *testing.T) {
    tests := []struct {
        name     string
        input    string
        expected string
    }{
        {
            name:     "double slash",
            input:    "a//b",
            expected: "a__b",
        },
        {
            name:     "triple slash",
            input:    "a///b",
            expected: "a___b",
        },
        {
            name:     "mixed single and double slashes",
            input:    "a/b//c",
            expected: "a_b__c",
        },
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            result := NormalizeResourceType(tt.input)
            if result != tt.expected {
                t.Errorf("NormalizeResourceType(%q) = %q, want %q", tt.input, result, tt.expected)
            }
        })
    }
}
```

**Test 3: Special Characters (Current Behavior)**
```go
func TestNormalizeResourceType_SpecialCharacters(t *testing.T) {
    // These tests document CURRENT behavior (may not be desired)
    tests := []struct {
        name        string
        input       string
        expected    string
        description string
    }{
        {
            name:        "dash preserved",
            input:       "k8s-cluster",
            expected:    "k8s-cluster",
            description: "Dashes are currently preserved",
        },
        {
            name:        "dot preserved",
            input:       "v1.host",
            expected:    "v1.host",
            description: "Dots are currently preserved",
        },
        {
            name:        "underscore preserved",
            input:       "k8s_cluster",
            expected:    "k8s_cluster",
            description: "Underscores are preserved",
        },
        {
            name:        "special chars with slash",
            input:       "k8s-v1/cluster.prod",
            expected:    "k8s-v1_cluster.prod",
            description: "Only slashes are replaced",
        },
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            result := NormalizeResourceType(tt.input)
            if result != tt.expected {
                t.Errorf("NormalizeResourceType(%q) = %q, want %q (%s)", 
                    tt.input, result, tt.expected, tt.description)
            }
        })
    }
}
```

**Test 4: Unicode Handling (Current Behavior)**
```go
func TestNormalizeResourceType_Unicode(t *testing.T) {
    // Documents current unicode behavior (may change with validation)
    tests := []struct {
        name        string
        input       string
        description string
    }{
        {
            name:        "accented characters preserved",
            input:       "café",
            description: "Unicode is currently preserved and lowercased",
        },
        {
            name:        "emoji preserved",
            input:       "🚀host",
            description: "Emoji currently preserved (lowercased if applicable)",
        },
        {
            name:        "chinese characters preserved",
            input:       "主机/服务器",
            description: "Non-Latin scripts preserved, slash replaced",
        },
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            result := NormalizeResourceType(tt.input)
            // Just verify it doesn't panic and returns something
            if result == "" && tt.input != "" {
                t.Errorf("NormalizeResourceType(%q) returned empty string", tt.input)
            }
            // Document what it actually returns
            t.Logf("NormalizeResourceType(%q) = %q (%s)", tt.input, result, tt.description)
        })
    }
}
```

**Total new tests for NormalizeResourceType:** ~12 test cases

---

## Area 2: Existing Model Type Constructors - Missing Edge Cases

### ResourceType Constructor

**Current:** `internal/biz/model/common.go:222`
- ✅ Empty string validation exists
- ✅ Trimming happens
- ✅ Basic tests exist in `common_test.go`

**Missing coverage:** Edge cases around normalization in Serialize()

#### File: `internal/biz/model/common_test.go`

**Test: Serialize() Normalization**
```go
func TestResourceType_Serialize(t *testing.T) {
    t.Parallel()
    
    tests := []struct {
        name     string
        input    string
        expected string
    }{
        {
            name:     "lowercase preserved",
            input:    "host",
            expected: "host",
        },
        {
            name:     "uppercase normalized to lowercase",
            input:    "HOST",
            expected: "host",
        },
        {
            name:     "mixed case normalized",
            input:    "HoSt",
            expected: "host",
        },
        {
            name:     "with underscore",
            input:    "k8s_cluster",
            expected: "k8s_cluster",
        },
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            t.Parallel()
            
            rt, err := NewResourceType(tt.input)
            if err != nil {
                t.Fatalf("NewResourceType(%q) error = %v", tt.input, err)
            }
            
            serialized := rt.Serialize()
            if serialized != tt.expected {
                t.Errorf("Serialize() = %q, want %q", serialized, tt.expected)
            }
        })
    }
}
```

**Total new tests for ResourceType:** ~4 test cases

---

### ReporterType Constructor

**Current:** `internal/biz/model/common.go:240`
- ✅ Empty string validation exists
- ✅ Trimming happens
- ✅ Serialize() lowercases

**Missing coverage:** Edge cases

#### File: `internal/biz/model/common_test.go`

**Test: ReporterType Edge Cases**
```go
func TestReporterType_EdgeCases(t *testing.T) {
    t.Parallel()
    
    tests := []struct {
        name        string
        input       string
        expectError bool
        expectedStr string
    }{
        {
            name:        "valid lowercase",
            input:       "hbi",
            expectError: false,
            expectedStr: "hbi",
        },
        {
            name:        "uppercase normalized",
            input:       "HBI",
            expectError: false,
            expectedStr: "hbi", // After Serialize()
        },
        {
            name:        "with dash",
            input:       "my-reporter",
            expectError: false,
            expectedStr: "my-reporter",
        },
        {
            name:        "leading whitespace trimmed",
            input:       "  hbi",
            expectError: false,
            expectedStr: "hbi",
        },
        {
            name:        "trailing whitespace trimmed",
            input:       "hbi  ",
            expectError: false,
            expectedStr: "hbi",
        },
        {
            name:        "only whitespace",
            input:       "   ",
            expectError: true,
        },
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            t.Parallel()
            
            rt, err := NewReporterType(tt.input)
            
            if tt.expectError {
                if err == nil {
                    t.Errorf("Expected error, got nil")
                }
            } else {
                if err != nil {
                    t.Errorf("Unexpected error: %v", err)
                }
                serialized := rt.Serialize()
                if serialized != tt.expectedStr {
                    t.Errorf("Serialize() = %q, want %q", serialized, tt.expectedStr)
                }
            }
        })
    }
}
```

**Total new tests for ReporterType:** ~6 test cases

---

### ReporterInstanceId Constructor

**Current:** `internal/biz/model/common.go:259`
- ✅ Empty string validation exists
- ✅ Trimming happens
- ✅ Serialize() lowercases

**Missing coverage:** Edge cases

#### File: `internal/biz/model/common_test.go`

**Test: ReporterInstanceId Edge Cases**
```go
func TestReporterInstanceId_EdgeCases(t *testing.T) {
    t.Parallel()
    
    tests := []struct {
        name        string
        input       string
        expectError bool
        expectedStr string
    }{
        {
            name:        "valid UUID",
            input:       "550e8400-e29b-41d4-a716-446655440000",
            expectError: false,
            expectedStr: "550e8400-e29b-41d4-a716-446655440000",
        },
        {
            name:        "uppercase UUID normalized",
            input:       "550E8400-E29B-41D4-A716-446655440000",
            expectError: false,
            expectedStr: "550e8400-e29b-41d4-a716-446655440000",
        },
        {
            name:        "custom string id",
            input:       "my-instance-123",
            expectError: false,
            expectedStr: "my-instance-123",
        },
        {
            name:        "whitespace trimmed",
            input:       "  instance-1  ",
            expectError: false,
            expectedStr: "instance-1",
        },
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            t.Parallel()
            
            ri, err := NewReporterInstanceId(tt.input)
            
            if tt.expectError {
                if err == nil {
                    t.Errorf("Expected error, got nil")
                }
            } else {
                if err != nil {
                    t.Errorf("Unexpected error: %v", err)
                }
                serialized := ri.Serialize()
                if serialized != tt.expectedStr {
                    t.Errorf("Serialize() = %q, want %q", serialized, tt.expectedStr)
                }
            }
        })
    }
}
```

**Total new tests for ReporterInstanceId:** ~4 test cases

---

### ConsistencyToken Constructor

**Current tests exist:** `internal/biz/model/common_test.go:533`

**Check if any edge cases missing:**

#### File: `internal/biz/model/common_test.go`

**Test: ConsistencyToken Edge Cases** (if not already covered)
```go
func TestConsistencyToken_EdgeCases(t *testing.T) {
    t.Parallel()
    
    tests := []struct {
        name        string
        input       string
        expectError bool
    }{
        {
            name:        "typical token",
            input:       "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9",
            expectError: false,
        },
        {
            name:        "numeric token",
            input:       "1234567890",
            expectError: false,
        },
        {
            name:        "whitespace trimmed",
            input:       "  token123  ",
            expectError: false,
        },
        {
            name:        "single character",
            input:       "a",
            expectError: false,
        },
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            t.Parallel()
            
            ct, err := NewConsistencyToken(tt.input)
            
            if tt.expectError {
                if err == nil {
                    t.Errorf("Expected error, got nil")
                }
            } else {
                if err != nil {
                    t.Errorf("Unexpected error: %v", err)
                }
                if ct.String() == "" {
                    t.Errorf("Expected non-empty token")
                }
            }
        })
    }
}
```

**Total new tests for ConsistencyToken:** ~4 test cases

---

### TransactionId Constructor

**Current:** `internal/biz/model/common.go:441`
**NOTE:** Has TODO comment - currently NO validation!

```go
// TODO: this needs to return an error if the transactionId is invalid
// This will break a bunch of tests.
func NewTransactionId(transactionId string) TransactionId {
    return TransactionId(transactionId)
}
```

**Missing coverage:** Document current behavior before fixing

#### File: `internal/biz/model/common_test.go`

**Test: TransactionId Current Behavior**
```go
func TestTransactionId_CurrentBehavior(t *testing.T) {
    t.Parallel()
    
    // These tests document CURRENT behavior (no validation)
    // TODO: Update when validation is added per TODO in common.go:441
    tests := []struct {
        name        string
        input       string
        description string
    }{
        {
            name:        "valid UUID",
            input:       "550e8400-e29b-41d4-a716-446655440000",
            description: "Currently accepts any string",
        },
        {
            name:        "empty string",
            input:       "",
            description: "Currently accepts empty (should reject when TODO fixed)",
        },
        {
            name:        "whitespace only",
            input:       "   ",
            description: "Currently accepts whitespace (should reject when TODO fixed)",
        },
        {
            name:        "very long string",
            input:       string(make([]byte, 1000)),
            description: "Currently accepts any length (should limit when TODO fixed)",
        },
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            t.Parallel()
            
            // Current implementation doesn't return error
            txnId := NewTransactionId(tt.input)
            
            // Document what we get
            t.Logf("NewTransactionId(%q) = %q (%s)", tt.input, txnId.String(), tt.description)
            
            // Just verify it doesn't panic
            _ = txnId.String()
        })
    }
}
```

**Total new tests for TransactionId:** ~4 test cases (documenting current lack of validation)

---

## Summary: Category 1 Tests

| Type | New Tests | Location | Status |
|------|-----------|----------|--------|
| NormalizeResourceType | 12 | `internal/data/schema_inmemory_test.go` | Should PASS |
| ResourceType.Serialize() | 4 | `internal/biz/model/common_test.go` | Should PASS |
| ReporterType | 6 | `internal/biz/model/common_test.go` | Should PASS |
| ReporterInstanceId | 4 | `internal/biz/model/common_test.go` | Should PASS |
| ConsistencyToken | 4 | `internal/biz/model/common_test.go` | Should PASS |
| TransactionId | 4 | `internal/biz/model/common_test.go` | Should PASS |
| **TOTAL** | **34** | **2 files** | **All should PASS** |

---

## Implementation Order

1. **NormalizeResourceType tests** (12 tests) - Most important, documents behavior before moving to model
2. **ResourceType tests** (4 tests) - Verifies Serialize() normalization
3. **Other model types** (18 tests) - Edge cases and whitespace handling
4. **TransactionId tests** (4 tests) - Documents current lack of validation

---

## Acceptance Criteria

- ✅ All 34 tests PASS immediately (no code changes needed)
- ✅ Tests document current behavior
- ✅ Tests use table-driven approach
- ✅ Tests are parallel where possible
- ✅ Tests provide safety net for future refactoring

---

## Next Steps

1. Write these 34 tests
2. Run test suite - verify all PASS
3. Commit: "Add missing test coverage for existing code (Category 1)"
4. Then address Category 2 (new validation) in separate PRs

---

## Notes

- These tests document **CURRENT** behavior, not desired behavior
- Some behaviors may change when validation is added (Category 2)
- TransactionId tests note the TODO and document lack of validation
- NormalizeResourceType tests will be useful when moving function to model
