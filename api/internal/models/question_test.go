package models

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestQuestionPublic_StripsSensitiveFields is the regression gate for the
// client-safety contract that's referenced in QuestionValidation's doc
// comment. The base Question/QuestionValidation/QuestionOption types now
// round-trip through JSON (so the DB spec-storage path works), which means
// struct-tag-level stripping is no longer the protection — ToPublic() is.
//
// If this test ever fails, it means something in QuestionPublic /
// QuestionOptionPublic drifted and sensitive fields (validation answers,
// regex patterns, "this is the correct option" flags) would leak to
// students in API responses.
func TestQuestionPublic_StripsSensitiveFields(t *testing.T) {
	q := Question{
		ID:          "q1",
		Type:        QuestionTypeMultipleChoice,
		Description: "Pick the right one",
		Points:      10,
		Hint:        "legacy hint",
		DependsOn:   []string{"q0"},
		Order:       3,
		Required:    true,
		MultiSelect: true,
		Validation: &QuestionValidation{
			Type:          ValidationTypeExact,
			Answer:        "secret-answer",
			Pattern:       "secret-regex",
			CaseSensitive: true,
		},
		Options: []QuestionOption{
			{ID: "a", Text: "Right", Correct: true},
			{ID: "b", Text: "Wrong", Correct: false},
		},
	}

	pub := q.ToPublic()

	encoded, err := json.Marshal(pub)
	if err != nil {
		t.Fatalf("json.Marshal(QuestionPublic): %v", err)
	}
	out := string(encoded)

	// Hard-ban every field that must never reach the client.
	banned := []string{
		"secret-answer",   // Validation.Answer
		"secret-regex",    // Validation.Pattern
		`"validation"`,    // The whole validation sub-object is absent from the public type
		`"correct"`,       // QuestionOption.Correct's JSON tag
		`"caseSensitive"`, // Implementation detail of validation; also stripped
	}
	for _, needle := range banned {
		if strings.Contains(out, needle) {
			t.Errorf("QuestionPublic JSON leaks %q: %s", needle, out)
		}
	}

	// Sanity-check: the fields that SHOULD reach clients are still there.
	required := []string{`"id":"q1"`, `"type":"multiple_choice"`, `"description":"Pick the right one"`, `"points":10`}
	for _, needle := range required {
		if !strings.Contains(out, needle) {
			t.Errorf("QuestionPublic JSON missing expected %q: %s", needle, out)
		}
	}

	// Option IDs + text are part of the client-facing surface (students need to
	// render the choices). Option.Correct is NOT.
	for _, opt := range pub.Options {
		if opt.ID == "" || opt.Text == "" {
			t.Errorf("expected public option to carry id + text, got %+v", opt)
		}
	}
}

// TestQuestionValidation_RoundTripsThroughJSON asserts the fix from issue
// #121: Answer and Pattern must round-trip through json.Marshal/Unmarshal so
// the DB spec-storage path (parseAndValidateSpec → lab_templates.spec) can
// recover the validation data when the submit-answer handler unmarshals it.
// If this test fails, validation will silently accept empty answers as
// correct (reproducing the production bug that #121 fixed).
func TestQuestionValidation_RoundTripsThroughJSON(t *testing.T) {
	orig := QuestionValidation{
		Type:          ValidationTypeExact,
		Answer:        "correct",
		Pattern:       `^\d+$`,
		CaseSensitive: true,
	}

	encoded, err := json.Marshal(orig)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}

	var decoded QuestionValidation
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}

	if decoded.Answer != orig.Answer {
		t.Errorf("Answer lost in round-trip: got %q, want %q", decoded.Answer, orig.Answer)
	}
	if decoded.Pattern != orig.Pattern {
		t.Errorf("Pattern lost in round-trip: got %q, want %q", decoded.Pattern, orig.Pattern)
	}
	if decoded.CaseSensitive != orig.CaseSensitive {
		t.Errorf("CaseSensitive lost in round-trip")
	}
}

// TestQuestionOption_CorrectRoundTripsThroughJSON is the matching assertion
// for multiple-choice options — Correct must survive JSON round-trip so the
// handler knows which option(s) are right when validating submissions.
func TestQuestionOption_CorrectRoundTripsThroughJSON(t *testing.T) {
	orig := []QuestionOption{
		{ID: "a", Text: "Option A", Correct: true},
		{ID: "b", Text: "Option B", Correct: false},
	}

	encoded, err := json.Marshal(orig)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}

	var decoded []QuestionOption
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}

	if len(decoded) != len(orig) {
		t.Fatalf("length changed: got %d, want %d", len(decoded), len(orig))
	}
	for i := range orig {
		if decoded[i].Correct != orig[i].Correct {
			t.Errorf("options[%d].Correct lost: got %v, want %v", i, decoded[i].Correct, orig[i].Correct)
		}
	}
}
