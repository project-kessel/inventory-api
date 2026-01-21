package middleware

import (
	"context"
	"strings"

	"github.com/go-kratos/kratos/v2/errors"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-kratos/kratos/v2/middleware"
	"github.com/go-kratos/kratos/v2/transport"

	pb "github.com/project-kessel/inventory-api/api/kessel/inventory/v1beta2"
	authnapi "github.com/project-kessel/inventory-api/internal/authn/api"
	authzapi "github.com/project-kessel/inventory-api/internal/authz/api"
	kessel "github.com/project-kessel/relations-api/api/kessel/relations/v1beta1"
)

const (
	metaauthorizerReason = "META_AUTHORIZATION_FAILED"
	// CheckSelfRelation is the relation used for meta-authorization checks on CheckSelf requests
	CheckSelfRelation = "check_self"
)

var (
	ErrMetaAuthorizerFailed = errors.Forbidden(metaauthorizerReason, "Meta-authorization check failed")
	ErrGRPCNotAllowed       = errors.Forbidden("GRPC_NOT_ALLOWED", "CheckSelf requests are not allowed via gRPC transport. Use HTTP with x-rh-identity authentication.")
)

// MetaAuthorizerConfig holds configuration for meta-authorization middleware
type MetaAuthorizerConfig struct {
	// Authorizer is used to perform the actual authorization checks
	// If nil, the middleware will use decision logic only (no actual metacheck)
	Authorizer authzapi.Authorizer
	// Namespace is the namespace to use for metachecks (e.g., "rbac")
	Namespace string
	// Enabled controls whether meta authorization is enabled
	Enabled bool
	// Logger is used for logging middleware operations
	Logger *log.Helper
}

// MetaAuthorizerMiddleware creates a middleware that performs meta-authorization checks
// on CheckSelf requests based on the flowchart logic.
//
// IMPORTANT: This middleware ONLY processes:
//   - HTTP requests (not gRPC)
//   - Requests with x-rh-identity auth type
//   - CheckSelfRequest type
//
// Flowchart Logic for CheckSelf:
//   - Inputs: authclaims, object, relation, [subject] - implicit
//   - Metacheck: object = object, relation = "check_self", subject = authclaims
//   - Decision Logic:
//     1. if relation == "check_self" → return allow
//     2. if subject.authtype == "oidc" → return allow
//     3. return deny (if metacheck fails)
//
// The middleware:
//  1. Checks if transport is HTTP (skip if gRPC)
//  2. Checks if auth type is x-rh-identity (skip if not)
//  3. Only handles CheckSelf requests
//  4. Extracts authclaims (identity) from context
//  5. Extracts object and relation from CheckSelfRequest
//  6. Applies decision logic (check_self relation or OIDC → allow)
//  7. If decision logic doesn't apply, performs metacheck
//  8. Allows or denies based on metacheck result
func MetaAuthorizerMiddleware(config MetaAuthorizerConfig, logger log.Logger) func(middleware.Handler) middleware.Handler {
	logHelper := log.NewHelper(log.With(logger, "subsystem", "metaauthorizer"))

	return func(next middleware.Handler) middleware.Handler {
		return func(ctx context.Context, req any) (any, error) {
			if !config.Enabled {
				logHelper.Debug("Meta-authorizer middleware disabled, skipping")
				return next(ctx, req)
			}

			// Only handle CheckSelf requests
			checkSelfReq, ok := req.(*pb.CheckSelfRequest)
			if !ok {
				// For other request types, pass through without meta-authz
				logHelper.Debugf("Skipping meta-authorization: request is not CheckSelfRequest (type: %T)", req)
				return next(ctx, req)
			}

			// // Block gRPC CheckSelf requests - only HTTP with x-rh-identity is allowed
			t, ok := transport.FromServerContext(ctx)
			if ok && t.Kind() == transport.KindGRPC {
				logHelper.Warnf("Blocking gRPC CheckSelf request: gRPC transport is not allowed for CheckSelf requests")
				return nil, ErrGRPCNotAllowed
			}

			// Get authclaims (identity) from context
			identity, err := GetIdentity(ctx)
			if err != nil {
				logHelper.Warnf("Failed to get identity for meta-authorization: %v", err)
				return nil, errors.Unauthorized("UNAUTHENTICATED", "failed to get identity for meta-authorization")
			}

			// Only process requests with x-rh-identity auth type
			if identity.AuthType != "x-rh-identity" {
				// Skip meta-authorization for non-x-rh-identity auth types (e.g., OIDC)
				logHelper.Debugf("Skipping meta-authorization: auth type is not x-rh-identity (type: %s)", identity.AuthType)
				return next(ctx, req)
			}

			originalRelation := checkSelfReq.Relation
			// Create a temporary request with relation="check_self" for Decision Rule 1 check
			// We preserve the original relation for the actual service handler
			tempReq := createTempRequestForDecisionLogic(checkSelfReq)

			logHelper.Debugf("Meta-authorization: Using relation '%s' for decision logic (original relation was '%s')", CheckSelfRelation, originalRelation)

			// Apply flowchart decision logic using tempReq with relation="check_self"
			allowed, err := performMetaAuthorizerDecision(ctx, tempReq, identity, config, logHelper)
			if err != nil {
				logHelper.Errorf("Meta-authorization decision failed: %v", err)
				return nil, err
			}

			if !allowed {
				logHelper.Warnf("Meta-authorization check failed: relation=%s, resourceType=%s, resourceId=%s, userID=%s",
					originalRelation,
					checkSelfReq.Object.GetResourceType(),
					checkSelfReq.Object.GetResourceId(),
					identity.UserID)
				return nil, ErrMetaAuthorizerFailed
			}

			logHelper.Debugf("Meta-authorization check passed: relation=%s, resourceType=%s, resourceId=%s, userID=%s",
				originalRelation,
				checkSelfReq.Object.GetResourceType(),
				checkSelfReq.Object.GetResourceId(),
				identity.UserID)

			// Meta-authorization passed, proceed to next handler
			return next(ctx, req)
		}
	}
}

func createTempRequestForDecisionLogic(originalReq *pb.CheckSelfRequest) *pb.CheckSelfRequest {
	return &pb.CheckSelfRequest{
		Object:      originalReq.Object,
		Relation:    CheckSelfRelation,
		Consistency: originalReq.Consistency,
	}
}

// extractConsistencyToken extracts the consistency token string from a CheckSelfRequest.
// Returns empty string if consistency is nil or uses minimize_latency.
// Returns the token string if consistency uses at_least_as_fresh.
func extractConsistencyToken(req *pb.CheckSelfRequest) string {
	consistency := req.GetConsistency()
	if consistency == nil {
		return ""
	}

	// Check if consistency has at_least_as_fresh token
	if token := consistency.GetAtLeastAsFresh(); token != nil {
		return token.GetToken()
	}

	// If minimize_latency is set, return empty string (minimize latency)
	return ""
}

// performMetaAuthorizerDecision implements the flowchart decision logic for CheckSelf.
func performMetaAuthorizerDecision(ctx context.Context, req *pb.CheckSelfRequest, identity *authnapi.Identity, config MetaAuthorizerConfig, logger *log.Helper) (bool, error) {
	// Decision rule 1: If relation == "check_self" → allow
	// Note: req.Relation is set to "check_self" before calling this function, so this always applies
	if req.Relation == CheckSelfRelation {
		logger.Debugf("Meta-authorization: Decision Rule 1 applies (relation == '%s'), allowing without metacheck", CheckSelfRelation)
		return true, nil
	}
	logger.Warnf("Meta-authorization: Decision Rule 1 did NOT apply (req.Relation='%s', CheckSelfRelation='%s'), this should not happen!", req.Relation, CheckSelfRelation)

	consistencyToken := extractConsistencyToken(req)
	if consistencyToken != "" {
		logger.Debugf("Meta-authorization: Using consistency token for metacheck")
	} else {
		logger.Debugf("Meta-authorization: No consistency token, using minimize latency")
	}
	logger.Debugf("Meta-authorization: About to call performMetaAuthorizerMetacheck")
	return performMetaAuthorizerMetacheck(ctx, req.Object, req.Relation, consistencyToken, identity, config, logger)
}

// performMetaAuthorizerMetacheck performs the metacheck for CheckSelf requests.
func performMetaAuthorizerMetacheck(ctx context.Context, object *pb.ResourceReference, relation string, consistencyToken string, identity *authnapi.Identity, config MetaAuthorizerConfig, logHelper *log.Helper) (bool, error) {
	// If no authorizer configured, deny (fail-safe)
	if config.Authorizer == nil {
		logHelper.Warn("Meta-authorization metacheck failed: no authorizer configured (fail-safe deny)")
		return false, nil
	}

	// Convert identity (authclaims) to SubjectReference for the metacheck
	subjectRef := subjectReferenceFromIdentityForMetaAuthorizer(identity)
	subjectID := subjectRef.Subject.Id

	logHelper.Debugf("Meta-authorization metacheck: subject '%s' can '%s' resource '%s/%s' in namespace '%s'",
		subjectID,
		relation,
		object.ResourceType,
		object.ResourceId,
		config.Namespace)

	// Perform the metacheck using the authorizer
	allowed, _, err := config.Authorizer.Check(
		ctx,
		config.Namespace,
		relation,
		consistencyToken, // Use consistency token from request
		object.ResourceType,
		object.ResourceId,
		subjectRef,
	)

	if err != nil {
		logHelper.Errorf("Meta-authorization metacheck error: %v (subject: %s, resource: %s/%s)",
			err,
			subjectID,
			object.ResourceType,
			object.ResourceId)
		return false, err
	}

	allowedResult := allowed == kessel.CheckResponse_ALLOWED_TRUE
	if allowedResult {
		logHelper.Debugf("Meta-authorization metacheck passed: subject '%s' can '%s' resource '%s/%s'",
			subjectID,
			relation,
			object.ResourceType,
			object.ResourceId)
	} else {
		logHelper.Debugf("Meta-authorization metacheck denied: subject '%s' cannot '%s' resource '%s/%s'",
			subjectID,
			relation,
			object.ResourceType,
			object.ResourceId)
	}

	// CheckResponse_ALLOWED_TRUE means the metacheck passed
	return allowedResult, nil
}

// subjectReferenceFromIdentityForMetaAuthorizer converts identity (authclaims) to a SubjectReference
// for use in meta-authorization metachecks.
//
// Conversion logic:
//   - x-rh-identity: Uses UserID if available, otherwise Principal
//   - OIDC: Parses Principal (extracts subject from "domain/subject" format)
//   - Namespace: Always "rbac" for meta-authorization
//   - Type: Always "principal"
func subjectReferenceFromIdentityForMetaAuthorizer(identity *authnapi.Identity) *kessel.SubjectReference {
	// Determine subject ID based on authentication type
	var subjectID string

	if identity.AuthType == "x-rh-identity" {
		// For x-rh-identity, prefer UserID if available
		if identity.UserID != "" {
			subjectID = identity.UserID
		} else if identity.Principal != "" {
			subjectID = identity.Principal
		}
	} else {
		// For OIDC and other auth types, parse Principal
		subjectID = identity.Principal
		if parts := strings.SplitN(identity.Principal, "/", 2); len(parts) == 2 {
			subjectID = parts[1]
		}
	}

	// Determine namespace - always "rbac" for meta-authz
	namespace := "rbac"

	return &kessel.SubjectReference{
		Relation: nil,
		Subject: &kessel.ObjectReference{
			Type: &kessel.ObjectType{
				Namespace: namespace,
				Name:      "principal",
			},
			Id: subjectID,
		},
	}
}
