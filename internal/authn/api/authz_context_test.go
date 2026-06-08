package api

import "testing"

func TestAuthzContext_ExtractPrincipal(t *testing.T) {
	tests := []struct {
		name     string
		authzCtx AuthzContext
		want     string
	}{
		{
			name: "nil subject returns unknown",
			authzCtx: AuthzContext{
				Protocol: ProtocolGRPC,
				Subject:  nil,
			},
			want: "unknown",
		},
		{
			name: "clientID present returns clientID",
			authzCtx: AuthzContext{
				Protocol: ProtocolGRPC,
				Subject: &Claims{
					ClientID:  ClientID("test-client-id"),
					SubjectId: SubjectId("test-subject-id"),
				},
			},
			want: "test-client-id",
		},
		{
			name: "only subjectId present returns subjectId",
			authzCtx: AuthzContext{
				Protocol: ProtocolGRPC,
				Subject: &Claims{
					ClientID:  ClientID(""),
					SubjectId: SubjectId("test-subject-id"),
				},
			},
			want: "test-subject-id",
		},
		{
			name: "both clientID and subjectId empty returns unknown",
			authzCtx: AuthzContext{
				Protocol: ProtocolGRPC,
				Subject: &Claims{
					ClientID:  ClientID(""),
					SubjectId: SubjectId(""),
				},
			},
			want: "unknown",
		},
		{
			name: "prefers clientID over subjectId when both present",
			authzCtx: AuthzContext{
				Protocol: ProtocolGRPC,
				Subject: &Claims{
					ClientID:  ClientID("client-123"),
					SubjectId: SubjectId("subject-456"),
				},
			},
			want: "client-123",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.authzCtx.ExtractPrincipal()
			if got != tt.want {
				t.Errorf("AuthzContext.ExtractPrincipal() = %v, want %v", got, tt.want)
			}
		})
	}
}
