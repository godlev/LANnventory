package identity

import "testing"

func TestAssessMAC(t *testing.T) {
	tests := []struct {
		name       string
		mac        string
		deviceType string
		code       MACAssessmentCode
		confidence MACAssessmentConfidence
	}{
		{
			name:       "global hardware address",
			mac:        "00:11:22:33:44:55",
			deviceType: "phone",
			code:       MACAssessmentGloballyAdministered,
			confidence: MACAssessmentConfidenceNone,
		},
		{
			name:       "local virtual machine",
			mac:        "02:11:22:33:44:55",
			deviceType: "virtual-machine",
			code:       MACAssessmentExpectedVirtual,
			confidence: MACAssessmentConfidenceHigh,
		},
		{
			name:       "local container",
			mac:        "02:11:22:33:44:56",
			deviceType: "container",
			code:       MACAssessmentExpectedVirtual,
			confidence: MACAssessmentConfidenceHigh,
		},
		{
			name:       "local phone is only possible privacy address",
			mac:        "02:11:22:33:44:57",
			deviceType: "phone",
			code:       MACAssessmentPossiblePrivateRandom,
			confidence: MACAssessmentConfidenceLow,
		},
		{
			name:       "local tablet is only possible privacy address",
			mac:        "02:11:22:33:44:58",
			deviceType: "tablet",
			code:       MACAssessmentPossiblePrivateRandom,
			confidence: MACAssessmentConfidenceLow,
		},
		{
			name:       "local laptop is only possible privacy address",
			mac:        "02:11:22:33:44:59",
			deviceType: "laptop",
			code:       MACAssessmentPossiblePrivateRandom,
			confidence: MACAssessmentConfidenceLow,
		},
		{
			name:       "local unknown physical type stays unknown",
			mac:        "02:11:22:33:44:60",
			deviceType: "tv",
			code:       MACAssessmentLocallyAdministered,
			confidence: MACAssessmentConfidenceNone,
		},
		{
			name:       "local unassigned type stays unknown",
			mac:        "02:11:22:33:44:61",
			deviceType: "",
			code:       MACAssessmentLocallyAdministered,
			confidence: MACAssessmentConfidenceNone,
		},
		{
			name:       "multicast is never privacy assessed",
			mac:        "01:00:5E:00:00:01",
			deviceType: "phone",
			code:       MACAssessmentGroupAddress,
			confidence: MACAssessmentConfidenceNone,
		},
		{
			name:       "invalid is never privacy assessed",
			mac:        "not-a-mac",
			deviceType: "phone",
			code:       MACAssessmentInvalid,
			confidence: MACAssessmentConfidenceNone,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assessment := AssessMAC(tt.mac, tt.deviceType)
			if assessment.Code != tt.code {
				t.Fatalf("Code = %q, want %q", assessment.Code, tt.code)
			}
			if assessment.Confidence != tt.confidence {
				t.Fatalf("Confidence = %q, want %q", assessment.Confidence, tt.confidence)
			}
			if assessment.Reason == "" {
				t.Fatal("Reason is empty")
			}
		})
	}
}
