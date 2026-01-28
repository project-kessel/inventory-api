package v1

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"google.golang.org/protobuf/proto"
)

// GetLivezRequest Tests

func TestGetLivezRequest_BasicBehavior(t *testing.T) {
	t.Run("empty request", func(t *testing.T) {
		req := &GetLivezRequest{}
		assert.NotNil(t, req)
	})

	t.Run("reset", func(t *testing.T) {
		req := &GetLivezRequest{}
		req.Reset()
		assert.NotNil(t, req)
	})

	t.Run("string representation", func(t *testing.T) {
		req := &GetLivezRequest{}
		// String() should not panic, even for empty request
		assert.NotPanics(t, func() {
			_ = req.String()
		})
	})

	t.Run("nil pointer safety", func(t *testing.T) {
		var req *GetLivezRequest
		// String() should be safe to call on nil
		assert.NotPanics(t, func() {
			_ = req.String()
		})
	})
}

func TestGetLivezRequest_ProtoInterface(t *testing.T) {
	// Verify protobuf interface implementation
	var req interface{} = &GetLivezRequest{}
	_, ok := req.(proto.Message)
	assert.True(t, ok, "GetLivezRequest should implement proto.Message")
}

// GetLivezResponse Tests

func TestGetLivezResponse_FullRoundTrip(t *testing.T) {
	resp := &GetLivezResponse{
		Status: "ok",
		Code:   200,
	}

	t.Run("json roundtrip", func(t *testing.T) {
		data, err := json.Marshal(resp)
		assert.NoError(t, err)

		var decoded GetLivezResponse
		err = json.Unmarshal(data, &decoded)
		assert.NoError(t, err)

		assert.Equal(t, "ok", decoded.GetStatus())
		assert.Equal(t, uint32(200), decoded.GetCode())
	})

	t.Run("protobuf roundtrip", func(t *testing.T) {
		data, err := proto.Marshal(resp)
		assert.NoError(t, err)

		var decoded GetLivezResponse
		err = proto.Unmarshal(data, &decoded)
		assert.NoError(t, err)

		assert.Equal(t, "ok", decoded.GetStatus())
		assert.Equal(t, uint32(200), decoded.GetCode())
	})
}

func TestGetLivezResponse_BasicBehavior(t *testing.T) {
	t.Run("reset", func(t *testing.T) {
		resp := &GetLivezResponse{
			Status: "ok",
			Code:   200,
		}
		resp.Reset()
		assert.Equal(t, "", resp.GetStatus())
		assert.Equal(t, uint32(0), resp.GetCode())
	})

	t.Run("string representation", func(t *testing.T) {
		resp := &GetLivezResponse{
			Status: "ok",
			Code:   200,
		}
		s := resp.String()
		assert.NotEmpty(t, s, "String() should return non-empty representation")
	})

	t.Run("nil pointer safety", func(t *testing.T) {
		var resp *GetLivezResponse
		// All getters should be safe to call on nil and return zero values
		assert.Equal(t, "", resp.GetStatus())
		assert.Equal(t, uint32(0), resp.GetCode())
	})

	t.Run("empty struct", func(t *testing.T) {
		var resp GetLivezResponse
		// All getters should return zero values, not panic
		assert.Equal(t, "", resp.GetStatus())
		assert.Equal(t, uint32(0), resp.GetCode())
	})
}

func TestGetLivezResponse_ProtoInterface(t *testing.T) {
	// Verify protobuf interface implementation
	var resp interface{} = &GetLivezResponse{}
	_, ok := resp.(proto.Message)
	assert.True(t, ok, "GetLivezResponse should implement proto.Message")
}

func TestGetLivezResponse_StatusCodes(t *testing.T) {
	testCases := []struct {
		name   string
		status string
		code   uint32
	}{
		{"success", "ok", 200},
		{"error", "error", 500},
		{"unavailable", "unavailable", 503},
		{"empty status", "", 0},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			resp := &GetLivezResponse{
				Status: tc.status,
				Code:   tc.code,
			}
			assert.Equal(t, tc.status, resp.GetStatus())
			assert.Equal(t, tc.code, resp.GetCode())
		})
	}
}

func TestGetLivezResponse_EdgeCases(t *testing.T) {
	t.Run("max uint32 code", func(t *testing.T) {
		resp := &GetLivezResponse{
			Status: "error",
			Code:   ^uint32(0), // Max uint32 value
		}
		assert.Equal(t, uint32(4294967295), resp.GetCode())
	})

	t.Run("very long status string", func(t *testing.T) {
		longStatus := string(make([]byte, 10000))
		for i := range longStatus {
			longStatus = longStatus[:i] + "a"
		}
		resp := &GetLivezResponse{
			Status: longStatus,
			Code:   200,
		}
		assert.Equal(t, len(longStatus), len(resp.GetStatus()))
	})

	t.Run("unicode characters in status", func(t *testing.T) {
		resp := &GetLivezResponse{
			Status: "健康 ✓ 🚀",
			Code:   200,
		}
		assert.Equal(t, "健康 ✓ 🚀", resp.GetStatus())
	})

	t.Run("newlines in status", func(t *testing.T) {
		resp := &GetLivezResponse{
			Status: "line1\nline2\nline3",
			Code:   200,
		}
		assert.Contains(t, resp.GetStatus(), "\n")
	})
}

func TestGetLivezResponse_Equality(t *testing.T) {
	resp1 := &GetLivezResponse{Status: "ok", Code: 200}
	resp2 := &GetLivezResponse{Status: "ok", Code: 200}
	resp3 := &GetLivezResponse{Status: "error", Code: 500}

	// Use protobuf's Equal method
	assert.True(t, proto.Equal(resp1, resp2), "identical responses should be equal")
	assert.False(t, proto.Equal(resp1, resp3), "different responses should not be equal")
}

func TestGetLivezResponse_Clone(t *testing.T) {
	original := &GetLivezResponse{
		Status: "ok",
		Code:   200,
	}

	// Clone using proto.Clone
	cloned := proto.Clone(original).(*GetLivezResponse)

	// Verify they're equal
	assert.True(t, proto.Equal(original, cloned))
	assert.Equal(t, original.GetStatus(), cloned.GetStatus())
	assert.Equal(t, original.GetCode(), cloned.GetCode())

	// Modify clone shouldn't affect original
	cloned.Status = "modified"
	assert.NotEqual(t, original.GetStatus(), cloned.GetStatus())
}

// GetReadyzRequest Tests

func TestGetReadyzRequest_BasicBehavior(t *testing.T) {
	t.Run("empty request", func(t *testing.T) {
		req := &GetReadyzRequest{}
		assert.NotNil(t, req)
	})

	t.Run("reset", func(t *testing.T) {
		req := &GetReadyzRequest{}
		req.Reset()
		assert.NotNil(t, req)
	})

	t.Run("string representation", func(t *testing.T) {
		req := &GetReadyzRequest{}
		// String() should not panic, even for empty request
		assert.NotPanics(t, func() {
			_ = req.String()
		})
	})

	t.Run("nil pointer safety", func(t *testing.T) {
		var req *GetReadyzRequest
		// String() should be safe to call on nil
		assert.NotPanics(t, func() {
			_ = req.String()
		})
	})
}

func TestGetReadyzRequest_ProtoInterface(t *testing.T) {
	// Verify protobuf interface implementation
	var req interface{} = &GetReadyzRequest{}
	_, ok := req.(proto.Message)
	assert.True(t, ok, "GetReadyzRequest should implement proto.Message")
}

// GetReadyzResponse Tests

func TestGetReadyzResponse_FullRoundTrip(t *testing.T) {
	resp := &GetReadyzResponse{
		Status: "ready",
		Code:   200,
	}

	t.Run("json roundtrip", func(t *testing.T) {
		data, err := json.Marshal(resp)
		assert.NoError(t, err)

		var decoded GetReadyzResponse
		err = json.Unmarshal(data, &decoded)
		assert.NoError(t, err)

		assert.Equal(t, "ready", decoded.GetStatus())
		assert.Equal(t, uint32(200), decoded.GetCode())
	})

	t.Run("protobuf roundtrip", func(t *testing.T) {
		data, err := proto.Marshal(resp)
		assert.NoError(t, err)

		var decoded GetReadyzResponse
		err = proto.Unmarshal(data, &decoded)
		assert.NoError(t, err)

		assert.Equal(t, "ready", decoded.GetStatus())
		assert.Equal(t, uint32(200), decoded.GetCode())
	})
}

func TestGetReadyzResponse_BasicBehavior(t *testing.T) {
	t.Run("reset", func(t *testing.T) {
		resp := &GetReadyzResponse{
			Status: "ready",
			Code:   200,
		}
		resp.Reset()
		assert.Equal(t, "", resp.GetStatus())
		assert.Equal(t, uint32(0), resp.GetCode())
	})

	t.Run("string representation", func(t *testing.T) {
		resp := &GetReadyzResponse{
			Status: "ready",
			Code:   200,
		}
		s := resp.String()
		assert.NotEmpty(t, s, "String() should return non-empty representation")
	})

	t.Run("nil pointer safety", func(t *testing.T) {
		var resp *GetReadyzResponse
		// All getters should be safe to call on nil and return zero values
		assert.Equal(t, "", resp.GetStatus())
		assert.Equal(t, uint32(0), resp.GetCode())
	})

	t.Run("empty struct", func(t *testing.T) {
		var resp GetReadyzResponse
		// All getters should return zero values, not panic
		assert.Equal(t, "", resp.GetStatus())
		assert.Equal(t, uint32(0), resp.GetCode())
	})
}

func TestGetReadyzResponse_ProtoInterface(t *testing.T) {
	// Verify protobuf interface implementation
	var resp interface{} = &GetReadyzResponse{}
	_, ok := resp.(proto.Message)
	assert.True(t, ok, "GetReadyzResponse should implement proto.Message")
}

func TestGetReadyzResponse_StatusCodes(t *testing.T) {
	testCases := []struct {
		name   string
		status string
		code   uint32
	}{
		{"ready", "ready", 200},
		{"not ready", "not ready", 503},
		{"degraded", "degraded", 429},
		{"empty status", "", 0},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			resp := &GetReadyzResponse{
				Status: tc.status,
				Code:   tc.code,
			}
			assert.Equal(t, tc.status, resp.GetStatus())
			assert.Equal(t, tc.code, resp.GetCode())
		})
	}
}
