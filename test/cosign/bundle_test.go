package cosign

import (
	"bytes"
	"encoding/json"
	"math"
	"testing"
)

func TestLatestSignatureBundle(t *testing.T) {
	for _, tc := range []struct {
		name      string
		output    string
		wantIndex int64
		wantErr   bool
	}{
		{
			name:   "omitted zero index",
			output: `{"verificationMaterial":{"tlogEntries":[{"inclusionProof":{"treeSize":"1"}}]}}`,
		},
		{
			name:   "string zero index",
			output: `{"verificationMaterial":{"tlogEntries":[{"logIndex":"0"}]}}`,
		},
		{
			name:   "number zero index",
			output: `{"verificationMaterial":{"tlogEntries":[{"logIndex":0}]}}`,
		},
		{
			name:      "string index",
			output:    `{"verificationMaterial":{"tlogEntries":[{"logIndex":"12"}]}}`,
			wantIndex: 12,
		},
		{
			name:      "number index",
			output:    `{"verificationMaterial":{"tlogEntries":[{"logIndex":12}]}}`,
			wantIndex: 12,
		},
		{
			name:      "64 bit index",
			output:    `{"verificationMaterial":{"tlogEntries":[{"logIndex":"9223372036854775807"}]}}`,
			wantIndex: math.MaxInt64,
		},
		{
			name:      "warning before bundle",
			output:    "WARNING: using a test repository\n" + `{"verificationMaterial":{"tlogEntries":[{"logIndex":"12"}]}}`,
			wantIndex: 12,
		},
		{
			name: "multiple bundles",
			output: `{"verificationMaterial":{"tlogEntries":[{"logIndex":"12"}]}}
{"verificationMaterial":{"tlogEntries":[{"logIndex":"3"}]}}`,
			wantIndex: 12,
		},
		{
			name: "skip bundle without entries",
			output: `{"verificationMaterial":{"tlogEntries":[]}}
{"verificationMaterial":{"tlogEntries":[{}]}}`,
		},
		{
			name:    "missing verification material",
			output:  `{}`,
			wantErr: true,
		},
		{
			name:    "empty entries",
			output:  `{"verificationMaterial":{"tlogEntries":[]}}`,
			wantErr: true,
		},
		{
			name:    "invalid index",
			output:  `{"verificationMaterial":{"tlogEntries":[{"logIndex":"invalid"}]}}`,
			wantErr: true,
		},
		{
			name:    "fractional index",
			output:  `{"verificationMaterial":{"tlogEntries":[{"logIndex":1.5}]}}`,
			wantErr: true,
		},
		{
			name:    "negative index",
			output:  `{"verificationMaterial":{"tlogEntries":[{"logIndex":"-1"}]}}`,
			wantErr: true,
		},
		{
			name:    "malformed JSON",
			output:  `{"verificationMaterial":`,
			wantErr: true,
		},
		{
			name: "malformed JSON after valid bundle",
			output: `{"verificationMaterial":{"tlogEntries":[{}]}}
{"verificationMaterial":`,
			wantErr: true,
		},
		{
			name:    "no JSON",
			output:  "no signature bundles found",
			wantErr: true,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			bundle, err := latestSignatureBundle([]byte(tc.output))
			if (err != nil) != tc.wantErr {
				t.Fatalf("latestSignatureBundle() error = %v, wantErr %v", err, tc.wantErr)
			}
			if tc.wantErr {
				return
			}
			if got := bundle.GetVerificationMaterial().GetTlogEntries()[0].GetLogIndex(); got != tc.wantIndex {
				t.Fatalf("selected log index = %d, want %d", got, tc.wantIndex)
			}
		})
	}
}

func TestLatestSignatureBundlePreservesDSSEEnvelope(t *testing.T) {
	const originalEnvelopeJSON = `{
		"payload":"AAEC/w==",
		"payloadType":"application/test",
		"signatures":[{"keyid":"test-key","sig":"AQID"}]
	}`
	output := `{"verificationMaterial":{"tlogEntries":[{"logIndex":"12"}]},"dsseEnvelope":` + originalEnvelopeJSON + `}
	{"verificationMaterial":{"tlogEntries":[{"logIndex":"3"}]}}`
	bundle, err := latestSignatureBundle([]byte(output))
	if err != nil {
		t.Fatal(err)
	}
	if bundle.GetDsseEnvelope() == nil {
		t.Fatal("selected bundle lost its DSSE envelope")
	}
	data := bundle.rawDSSEEnvelope
	if !bytes.Equal(data, []byte(originalEnvelopeJSON)) {
		t.Fatalf("DSSE envelope bytes changed:\ngot:  %s\nwant: %s", data, originalEnvelopeJSON)
	}

	var envelope struct {
		Payload     []byte `json:"payload"`
		PayloadType string `json:"payloadType"`
		Signatures  []struct {
			KeyID string `json:"keyid"`
			Sig   []byte `json:"sig"`
		} `json:"signatures"`
	}
	if err := json.Unmarshal(data, &envelope); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(envelope.Payload, []byte{0, 1, 2, 255}) || envelope.PayloadType != "application/test" {
		t.Fatalf("DSSE payload changed: %s", data)
	}
	if len(envelope.Signatures) != 1 || envelope.Signatures[0].KeyID != "test-key" || !bytes.Equal(envelope.Signatures[0].Sig, []byte{1, 2, 3}) {
		t.Fatalf("DSSE signature changed: %s", data)
	}
}
