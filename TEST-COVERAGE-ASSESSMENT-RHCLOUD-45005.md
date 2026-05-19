# Test Coverage Assessment - RHCLOUD-45005
## Move Logic to Model (3 Areas)

**Date:** 2026-04-27  
**Jira:** https://redhat.atlassian.net/browse/RHCLOUD-45005  
**Branch:** RHCLOUD-45005-add-move-logic-model-value-types

---

## Executive Summary

**Good News:** Areas 1 and 2 already have substantial test coverage. Area 3 needs additional validation tests.

### Assessment by Area

| Area | Status | Test Coverage | Gaps Identified |
|------|--------|---------------|-----------------|
| **Area 1:** Type aliases (Version, ConsistencyToken) | ✅ **ALREADY DONE** | Comprehensive | None - types exist with proper DDD patterns |
| **Area 2:** Move NormalizeResourceType to model | ⚠️ **MOVE NEEDED** | Good (existing tests) | Need to move tests with function |
| **Area 3:** Add validation to model types | ❌ **WORK NEEDED** | Basic validation exists | Missing regex/character limit validation |

---

## Area 1: Type Aliases / Value Objects

### Current State ✅

**Types already exist as proper tiny types in `internal/biz/model/common.go`:**

- `type Version uint` (lines 143-162)
  - Constructor: `NewVersion(uint) Version`
  - Methods: `Uint()`, `Increment()`, `Serialize()`
  - Deserializer: `DeserializeVersion(uint) Version`

- `type ConsistencyToken string` (lines 277-293)
  - Constructor: `NewConsistencyToken(string) (ConsistencyToken, error)`
  - Validates: non-empty (trim + check)
  - Methods: `String()`, `Serialize()`
  - Deserializer: `DeserializeConsistencyToken(string) ConsistencyToken`

### Test Coverage ✅

**File:** `internal/biz/model/common_test.go`

**Version tests:**
- `TestVersion_Initialization` - zero, positive, large values, type safety
- `TestVersion_Increment` - increment behavior, immutability, rollover

**ConsistencyToken tests:**
- `TestConsistencyToken_Initialization` (line 533)
- Validates empty string handling

### Conclusion

**✅ Area 1 is COMPLETE.** Both types already follow DDD tiny type pattern from RHCLOUD-45010:
- Unexported fields (via type alias on primitive)
- Constructor with validation
- Serialization/deserialization
- Proper test coverage

**No PR needed for Area 1.**

---

## Area 2: Move NormalizeResourceType to Model

### Current State ⚠️

**Location:** `internal/data/schema_inmemory.go:364`

```go
func NormalizeResourceType(resourceType string) string {
	return strings.ToLower(strings.ReplaceAll(resourceType, "/", "_"))
}
```

**Problem:** This is normalization logic that should be part of the `ResourceType` value object in the model, not in the data layer.

**Usage:** Called in 11 places within `schema_inmemory.go`:
- Lines: 39, 53, 64, 80, 91, 107, 127, 144, 164, 182, 211

### Test Coverage ✅

**File:** `internal/data/schema_inmemory_test.go:605`

**Test:** `TestNormalizeResourceType`
- Cases: forward slash, no slash, multiple slashes, empty string, already normalized
- **Coverage: Good**

### What Needs to Happen

1. **Move function** to `internal/biz/model/common.go` as a method on `ResourceType`
2. **Move tests** to `internal/biz/model/common_test.go`
3. **Update all callers** in `schema_inmemory.go` to use `resourceType.Normalize()` or similar
4. **Consider:** Should normalization happen in the constructor, or as a separate method?

### Proposed Approach

**Option A:** Normalize in constructor (automatic)
```go
func NewResourceType(resourceType string) (ResourceType, error) {
    resourceType = strings.TrimSpace(resourceType)
    if resourceType == "" {
        return ResourceType(""), fmt.Errorf("%w: ResourceType", ErrEmpty)
    }
    // Normalize: lowercase + replace / with _
    normalized := strings.ToLower(strings.ReplaceAll(resourceType, "/", "_"))
    return ResourceType(normalized), nil
}
```

**Option B:** Separate normalize method (explicit)
```go
func (rt ResourceType) Normalize() ResourceType {
    normalized := strings.ToLower(strings.ReplaceAll(string(rt), "/", "_"))
    return ResourceType(normalized)
}
```

**Recommendation:** Option A - normalize in constructor. This ensures ResourceType is always in canonical form.

### Test Gap Analysis

**Existing tests cover:**
- ✅ Forward slash replacement
- ✅ Case normalization (implicit via ToLower)
- ✅ Empty string handling
- ✅ Already-normalized inputs

**Missing tests:**
- ⚠️ Mixed case input (e.g., "RHEL/Host" → "rhel_host")
- ⚠️ Leading/trailing whitespace with normalization
- ⚠️ Unicode characters behavior
- ⚠️ Very long resource type names

**Recommendation:** Add 4-5 additional test cases when moving tests.

---

## Area 3: Add Validation to Model Types

### Current State ❌

**Basic validation exists** (empty checks) but **missing:**
1. Regex validation for format
2. Character limits (max length)
3. Character set restrictions (alphanumeric, specific symbols)

### Types Needing Enhanced Validation

Based on AGENTS.md and security/API guidelines:

| Type | Current Validation | Missing Validation |
|------|-------------------|-------------------|
| `ResourceType` | ✅ Non-empty | ❌ Regex: `^[a-z0-9_]+$` (after normalization)<br>❌ Max length: 64 chars |
| `ReporterType` | ✅ Non-empty | ❌ Regex: `^[a-z0-9_-]+$`<br>❌ Max length: 64 chars |
| `ReporterInstanceId` | ✅ Non-empty | ❌ Max length: 256 chars<br>❌ No control characters |
| `LocalResourceId` | ✅ Non-empty | ❌ Max length: 256 chars<br>❌ URL-safe characters |
| `ConsistencyToken` | ✅ Non-empty | ❌ Max length: 512 chars<br>❌ Base64 or alphanumeric |
| `LockId` | ✅ Non-empty | ❌ Max length: 128 chars |
| `LockToken` | ✅ Non-empty | ❌ Max length: 512 chars |
| `TransactionId` | ❌ **NO VALIDATION** | ❌ Non-empty check<br>❌ Format validation |

### Test Coverage Gaps

**Current tests only verify:**
- ✅ Empty string rejection
- ✅ Successful creation with valid input

**Missing test coverage:**
- ❌ Max length boundary testing
- ❌ Invalid character rejection
- ❌ Regex pattern compliance
- ❌ Control character rejection
- ❌ Unicode handling
- ❌ Very long inputs (DoS prevention)

### Validation Rules to Add

#### ResourceType (priority: HIGH)
```go
const maxResourceTypeLength = 64
var resourceTypeRegex = regexp.MustCompile(`^[a-z0-9_]+$`)

func NewResourceType(resourceType string) (ResourceType, error) {
    resourceType = strings.TrimSpace(resourceType)
    if resourceType == "" {
        return ResourceType(""), fmt.Errorf("%w: ResourceType", ErrEmpty)
    }
    
    // Normalize first
    normalized := strings.ToLower(strings.ReplaceAll(resourceType, "/", "_"))
    
    // Validate length
    if len(normalized) > maxResourceTypeLength {
        return ResourceType(""), fmt.Errorf("%w: ResourceType exceeds max length %d", ErrInvalidFormat, maxResourceTypeLength)
    }
    
    // Validate character set
    if !resourceTypeRegex.MatchString(normalized) {
        return ResourceType(""), fmt.Errorf("%w: ResourceType contains invalid characters", ErrInvalidFormat)
    }
    
    return ResourceType(normalized), nil
}
```

#### ConsistencyToken (priority: MEDIUM)
```go
const maxConsistencyTokenLength = 512

func NewConsistencyToken(token string) (ConsistencyToken, error) {
    token = strings.TrimSpace(token)
    if token == "" {
        return ConsistencyToken(""), fmt.Errorf("%w: ConsistencyToken", ErrEmpty)
    }
    
    if len(token) > maxConsistencyTokenLength {
        return ConsistencyToken(""), fmt.Errorf("%w: ConsistencyToken exceeds max length", ErrInvalidFormat)
    }
    
    return ConsistencyToken(token), nil
}
```

#### TransactionId (priority: HIGH - currently has TODO)
```go
const maxTransactionIdLength = 128

// Current implementation has TODO about validation
func NewTransactionId(transactionId string) (TransactionId, error) {
    transactionId = strings.TrimSpace(transactionId)
    if transactionId == "" {
        return TransactionId(""), fmt.Errorf("%w: TransactionId", ErrEmpty)
    }
    
    if len(transactionId) > maxTransactionIdLength {
        return TransactionId(""), fmt.Errorf("%w: TransactionId exceeds max length", ErrInvalidFormat)
    }
    
    return TransactionId(transactionId), nil
}
```

### Required Test Coverage for Area 3

For **each type with enhanced validation**, add tests for:

1. **Valid boundary cases:**
   - ✅ Minimum valid input (1 char after trim)
   - ✅ Maximum valid input (at length limit)
   - ✅ Valid characters throughout range

2. **Invalid inputs:**
   - ❌ Empty string (already tested)
   - ❌ Only whitespace
   - ❌ Exceeds max length by 1
   - ❌ Far exceeds max length (2x, 10x)
   - ❌ Invalid characters (specific to each type)
   - ❌ Control characters (\n, \t, \0)
   - ❌ Unicode characters (if not allowed)
   - ❌ Leading/trailing whitespace (should be trimmed)

3. **Edge cases:**
   - ❌ Exact length boundary
   - ❌ Special characters behavior
   - ❌ Case sensitivity (where applicable)

### Estimated Test Count

| Type | New Tests Needed | Priority |
|------|-----------------|----------|
| ResourceType | 8-10 tests | HIGH |
| ReporterType | 6-8 tests | MEDIUM |
| ReporterInstanceId | 6-8 tests | MEDIUM |
| LocalResourceId | 6-8 tests | MEDIUM |
| ConsistencyToken | 5-7 tests | MEDIUM |
| TransactionId | 6-8 tests | HIGH |
| LockId | 5-6 tests | LOW |
| LockToken | 5-6 tests | LOW |

**Total:** ~47-67 new validation tests needed

---

## Recommendations for PR Sequence

### PR 1: Area 2 - Move NormalizeResourceType to Model (RECOMMENDED FIRST)
**Complexity:** Low  
**Risk:** Low  
**Dependencies:** None  
**Test Coverage:** Good (existing tests move with function)

**Steps:**
1. Add characterization tests if needed (verify current behavior)
2. Move `NormalizeResourceType` logic to `ResourceType` constructor
3. Move tests to `common_test.go`
4. Update all callers in `schema_inmemory.go`
5. Add 4-5 additional edge case tests
6. Verify no behavior change

**Estimated effort:** 2-4 hours

---

### PR 2: Area 3 - Add Validation (HIGH PRIORITY types)
**Complexity:** Medium  
**Risk:** Medium (could break existing code with invalid data)  
**Dependencies:** PR 1 (if normalizing in constructor)

**Scope:** ResourceType, TransactionId only

**Steps:**
1. Write failing tests for validation (TDD)
2. Add validation logic to constructors
3. Update error types if needed
4. Fix any callers that break
5. Verify no existing tests break (or fix them)

**Estimated effort:** 4-6 hours

---

### PR 3: Area 3 - Add Validation (REMAINING types)
**Complexity:** Medium  
**Risk:** Low-Medium  
**Dependencies:** PR 2 (learn from first validation PR)

**Scope:** ConsistencyToken, ReporterType, ReporterInstanceId, LocalResourceId, LockId, LockToken

**Steps:** Same as PR 2, but for remaining types

**Estimated effort:** 6-8 hours

---

## Test Coverage Summary

### What EXISTS ✅
- Version type tests (comprehensive)
- ConsistencyToken type tests (basic)
- NormalizeResourceType function tests (good)
- Basic empty-string validation tests for most types

### What's MISSING ❌
- Regex validation tests
- Max length boundary tests
- Invalid character rejection tests
- Control character tests
- DoS prevention tests (very long inputs)
- Unicode handling tests

### Test-First Approach

For each PR:
1. **Write tests first** (TDD red phase)
2. **Implement validation** (green phase)
3. **Refactor if needed** (refactor phase)
4. **Run full test suite** to catch regressions

---

## Risk Assessment

### Low Risk Areas
- ✅ Area 1 (already done)
- ✅ Moving NormalizeResourceType (existing tests, no new validation)

### Medium Risk Areas
- ⚠️ ResourceType validation (widely used, could break existing data)
- ⚠️ TransactionId validation (currently has no validation, TODO exists)

### Mitigation Strategies
1. **Characterization tests** before refactoring
2. **Incremental rollout** (one type at a time)
3. **Database audit** - check if existing data would fail new validation
4. **Graceful degradation** - log validation failures initially, then enforce

---

## Next Steps

1. **Review this assessment** with team/stakeholders
2. **Choose PR 1 scope** (Area 2 - move NormalizeResourceType)
3. **Create test plan** for PR 1
4. **Begin TDD workflow** (write tests, implement, refactor)
5. **Repeat for PRs 2-3**

---

## Notes

- All validation should align with API contracts and security guidelines
- Consider backward compatibility - existing data may not meet new validation rules
- Validation in model is NOT redundant with protobuf validation (defense in depth)
- See AGENTS.md for reference: "validation should not only be in the proto but also in the model"
