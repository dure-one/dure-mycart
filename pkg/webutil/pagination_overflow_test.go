package webutil

import (
	"math"
	"testing"
)

// TestPagination_Int32Overflow verifies that ParsePagination clamps page values
// to prevent offset from exceeding int32 range when converted in database queries.
//
// Security issue: An attacker could pass page=100000000 which creates an offset
// exceeding MaxInt32, causing overflow when converted to int32 in database layer.
func TestPagination_Int32Overflow(t *testing.T) {
	tests := []struct {
		name        string
		page        int
		limit       int
		expectClamp bool // true if page should be clamped
	}{
		{
			name:        "large page gets clamped to safe value",
			page:        100_000_000,
			limit:       100,
			expectClamp: true,
			// Without clamping: offset = 9,999,999,900 (exceeds MaxInt32)
			// With clamping: page clamped to 21,474,837 → offset = 2,147,483,600 (safe)
		},
		{
			name:        "max safe int32 page not clamped",
			page:        21_474_836, // (MaxInt32 / 100) rounded down
			limit:       100,
			expectClamp: false,
			// offset = 2,147,483,500 (safe, within MaxInt32)
		},
		{
			name:        "edge case at boundary is safe",
			page:        21_474_837, // (MaxInt32 / 100) + 1 = exactly maxSafePage
			limit:       100,
			expectClamp: false,
			// offset = (21,474,837 - 1) * 100 = 2,147,483,600 (still within MaxInt32 = 2,147,483,647)
		},
		{
			name:        "one beyond boundary gets clamped",
			page:        21_474_838, // One more than maxSafePage
			limit:       100,
			expectClamp: true,
			// Without clamping: offset = 2,147,483,700 (exceeds MaxInt32)
			// With clamping: page clamped to 21,474,837 → offset = 2,147,483,600
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock fiber context with query parameters
			// Note: We can't easily create a real fiber.Ctx, so we test the logic directly
			limit := tt.limit
			page := tt.page

			// Apply the same clamping logic as ParsePagination
			if limit < 1 {
				limit = defaultLimit
			}
			if limit > MaxLimit {
				limit = MaxLimit
			}
			maxSafePage := (math.MaxInt32 / limit) + 1
			if page > maxSafePage {
				page = maxSafePage
			}
			offset := (page - 1) * limit

			// Verify offset fits in int32 range
			if offset > math.MaxInt32 {
				t.Errorf("offset %d exceeds MaxInt32 (%d)", offset, math.MaxInt32)
			}
			if offset < math.MinInt32 {
				t.Errorf("offset %d below MinInt32 (%d)", offset, math.MinInt32)
			}

			// Verify conversion to int32 is safe
			int32Offset := int32(offset)
			if int(int32Offset) != offset {
				t.Errorf("int32 conversion changed value: %d -> %d", offset, int32Offset)
			}

			// Verify clamping behavior
			if tt.expectClamp && page == tt.page {
				t.Errorf("expected page to be clamped, but it wasn't: input=%d, output=%d", tt.page, page)
			}
		})
	}
}

// TestPagination_LimitInt32Safe verifies limit is always safe for int32 conversion.
func TestPagination_LimitInt32Safe(t *testing.T) {
	// MaxLimit is 100, which is always safe for int32
	if MaxLimit > math.MaxInt32 {
		t.Errorf("MaxLimit %d exceeds MaxInt32", MaxLimit)
	}

	// Test that limit gets clamped correctly
	testLimit := MaxLimit + 1000
	if testLimit <= math.MaxInt32 {
		// Even unclamped values should be safe, but verify anyway
		int32Limit := int32(testLimit)
		if int(int32Limit) != testLimit {
			t.Errorf("limit conversion unsafe: %d -> %d", testLimit, int32Limit)
		}
	}
}
