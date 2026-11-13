package cosign

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"

	bundlev1 "github.com/sigstore/protobuf-specs/gen/pb-go/bundle/v1"
	"google.golang.org/protobuf/encoding/protojson"
)

type signatureBundle struct {
	*bundlev1.Bundle
	// store the raw DSSE envelope too to avoid re-marshalling it through Go code (it changes hashes)
	rawDSSEEnvelope json.RawMessage
}

func latestSignatureBundle(output []byte) (*signatureBundle, error) {
	start := bytes.IndexByte(output, '{')
	if start == -1 {
		return nil, fmt.Errorf("no JSON object in bundle output")
	}

	var best *signatureBundle
	bestIndex := int64(-1)
	unmarshalOptions := protojson.UnmarshalOptions{DiscardUnknown: true}
	decoder := json.NewDecoder(bytes.NewReader(output[start:]))
	for {
		var raw json.RawMessage
		if err := decoder.Decode(&raw); errors.Is(err, io.EOF) {
			break
		} else if err != nil {
			return nil, fmt.Errorf("decode signature bundle JSON: %w", err)
		}

		bundle := new(bundlev1.Bundle)
		if err := unmarshalOptions.Unmarshal(raw, bundle); err != nil {
			continue
		}
		entries := bundle.GetVerificationMaterial().GetTlogEntries()
		if len(entries) == 0 {
			continue
		}
		if index := entries[0].GetLogIndex(); index > bestIndex {

			// Rekor hashes the envelope's exact JSON bytes - store the raw message too
			var fields map[string]json.RawMessage
			if err := json.Unmarshal(raw, &fields); err != nil {
				return nil, fmt.Errorf("extract DSSE envelope JSON: %w", err)
			}
			envelope := fields["dsseEnvelope"]
			if envelope == nil {
				envelope = fields["dsse_envelope"]
			}
			best = &signatureBundle{Bundle: bundle, rawDSSEEnvelope: envelope}
			bestIndex = index
		}
	}
	if best == nil {
		return nil, fmt.Errorf("no bundle with valid tlog entry")
	}
	return best, nil
}
