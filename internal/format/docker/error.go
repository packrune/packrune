// SPDX-License-Identifier: AGPL-3.0-only WITH Commons-Clause
// Copyright (C) 2026 Packrune Contributors

// Docker Registry V2 error response format. The spec requires JSON envelopes
// with stable error codes — clients display these to users, so we follow it
// exactly.

package docker

import (
	"encoding/json"
	"net/http"
)

// errorCode is one of the documented Docker registry error codes.
type errorCode string

const (
	errCodeBlobUnknown       errorCode = "BLOB_UNKNOWN"
	errCodeManifestUnknown   errorCode = "MANIFEST_UNKNOWN"
	errCodeManifestInvalid   errorCode = "MANIFEST_INVALID"
	errCodeNameUnknown       errorCode = "NAME_UNKNOWN"
	errCodeNameInvalid       errorCode = "NAME_INVALID"
	errCodeDigestInvalid     errorCode = "DIGEST_INVALID"
	errCodeBlobUploadUnknown errorCode = "BLOB_UPLOAD_UNKNOWN"
	errCodeUnauthorized      errorCode = "UNAUTHORIZED"
	errCodeDenied            errorCode = "DENIED"
	errCodeUnsupported       errorCode = "UNSUPPORTED"
)

type errorBody struct {
	Errors []errorItem `json:"errors"`
}

type errorItem struct {
	Code    errorCode `json:"code"`
	Message string    `json:"message"`
	Detail  any       `json:"detail,omitempty"`
}

func writeError(w http.ResponseWriter, status int, code errorCode, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(errorBody{
		Errors: []errorItem{{Code: code, Message: message}},
	})
}
