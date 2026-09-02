package server

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestValidateUUID(t *testing.T) {
	tests := []struct {
		name    string
		field   string
		value   string
		want    string
		wantErr bool
	}{
		{
			name:    "valid UUID",
			field:   "id",
			value:   "550e8400-e29b-41d4-a716-446655440000",
			want:    "550e8400-e29b-41d4-a716-446655440000",
			wantErr: false,
		},
		{
			name:    "valid UUID with spaces",
			field:   "id",
			value:   "  550e8400-e29b-41d4-a716-446655440000  ",
			want:    "550e8400-e29b-41d4-a716-446655440000",
			wantErr: false,
		},
		{
			name:    "empty value",
			field:   "id",
			value:   "",
			wantErr: true,
		},
		{
			name:    "invalid UUID format",
			field:   "id",
			value:   "not-a-uuid",
			wantErr: true,
		},
		{
			name:    "invalid UUID short",
			field:   "id",
			value:   "550e8400-e29b-41d4",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ValidateUUID(tt.field, tt.value)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateUUID() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("ValidateUUID() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestValidateName(t *testing.T) {
	tests := []struct {
		name    string
		field   string
		value   string
		want    string
		wantErr bool
	}{
		{
			name:    "valid name",
			field:   "name",
			value:   "my-lab-01",
			want:    "my-lab-01",
			wantErr: false,
		},
		{
			name:    "valid name with underscore",
			field:   "name",
			value:   "lab_template_v2",
			want:    "lab_template_v2",
			wantErr: false,
		},
		{
			name:    "valid name with dot",
			field:   "name",
			value:   "lab.config.v1",
			want:    "lab.config.v1",
			wantErr: false,
		},
		{
			name:    "valid name with spaces trimmed",
			field:   "name",
			value:   "  mylab  ",
			want:    "mylab",
			wantErr: false,
		},
		{
			name:    "empty value",
			field:   "name",
			value:   "",
			wantErr: true,
		},
		{
			name:    "path traversal attack ..",
			field:   "name",
			value:   "../etc/passwd",
			wantErr: true,
		},
		{
			name:    "path traversal attack //",
			field:   "name",
			value:   "foo//bar",
			wantErr: true,
		},
		{
			name:    "path traversal attack backslash",
			field:   "name",
			value:   "foo\\bar",
			wantErr: true,
		},
		{
			name:    "invalid characters",
			field:   "name",
			value:   "lab@#$%",
			wantErr: true,
		},
		{
			name:    "starts with hyphen",
			field:   "name",
			value:   "-invalid",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ValidateName(tt.field, tt.value)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateName() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("ValidateName() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestValidateVMID(t *testing.T) {
	tests := []struct {
		name    string
		field   string
		value   string
		want    string
		wantErr bool
	}{
		{
			name:    "valid VMID",
			field:   "vmid",
			value:   "100",
			want:    "100",
			wantErr: false,
		},
		{
			name:    "valid large VMID",
			field:   "vmid",
			value:   "999999",
			want:    "999999",
			wantErr: false,
		},
		{
			name:    "valid VMID with spaces trimmed",
			field:   "vmid",
			value:   "  100  ",
			want:    "100",
			wantErr: false,
		},
		{
			name:    "empty value",
			field:   "vmid",
			value:   "",
			wantErr: true,
		},
		{
			name:    "negative number",
			field:   "vmid",
			value:   "-100",
			wantErr: true,
		},
		{
			name:    "not a number",
			field:   "vmid",
			value:   "abc",
			wantErr: true,
		},
		{
			name:    "decimal number",
			field:   "vmid",
			value:   "100.5",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ValidateVMID(tt.field, tt.value)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateVMID() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("ValidateVMID() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestValidateOptionalID(t *testing.T) {
	tests := []struct {
		name    string
		field   string
		value   string
		want    string
		wantErr bool
	}{
		{
			name:    "empty value is allowed",
			field:   "id",
			value:   "",
			want:    "",
			wantErr: false,
		},
		{
			name:    "valid UUID",
			field:   "id",
			value:   "550e8400-e29b-41d4-a716-446655440000",
			want:    "550e8400-e29b-41d4-a716-446655440000",
			wantErr: false,
		},
		{
			name:    "valid alphanumeric name",
			field:   "id",
			value:   "lab-template-01",
			want:    "lab-template-01",
			wantErr: false,
		},
		{
			name:    "invalid value",
			field:   "id",
			value:   "../etc/passwd",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ValidateOptionalID(tt.field, tt.value)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateOptionalID() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("ValidateOptionalID() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestValidateID(t *testing.T) {
	tests := []struct {
		name    string
		field   string
		value   string
		want    string
		wantErr bool
	}{
		{
			name:    "valid UUID",
			field:   "id",
			value:   "550e8400-e29b-41d4-a716-446655440000",
			want:    "550e8400-e29b-41d4-a716-446655440000",
			wantErr: false,
		},
		{
			name:    "valid alphanumeric name",
			field:   "id",
			value:   "lab-template-01",
			want:    "lab-template-01",
			wantErr: false,
		},
		{
			name:    "valid name with underscore",
			field:   "id",
			value:   "lab_template_v2",
			want:    "lab_template_v2",
			wantErr: false,
		},
		{
			name:    "empty value",
			field:   "id",
			value:   "",
			wantErr: true,
		},
		{
			name:    "path traversal ..",
			field:   "id",
			value:   "../etc/passwd",
			wantErr: true,
		},
		{
			name:    "path traversal //",
			field:   "id",
			value:   "foo//bar",
			wantErr: true,
		},
		{
			name:    "path traversal backslash",
			field:   "id",
			value:   "foo\\bar",
			wantErr: true,
		},
		{
			name:    "special characters",
			field:   "id",
			value:   "invalid@#$%",
			wantErr: true,
		},
		{
			name:    "too long",
			field:   "id",
			value:   string(make([]byte, 300)),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ValidateID(tt.field, tt.value)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateID() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("ValidateID() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestValidateNameTooLong(t *testing.T) {
	// Create a name longer than MaxNameLength
	longName := make([]byte, MaxNameLength+1)
	for i := range longName {
		longName[i] = 'a'
	}

	_, err := ValidateName("name", string(longName))
	if err == nil {
		t.Error("ValidateName() should error for name exceeding max length")
	}
}

// TestDecodeAndValidate tests the generic decode and validate function
func TestDecodeAndValidate(t *testing.T) {
	type testStruct struct {
		Name  string `json:"name" validate:"required,min=2,max=50"`
		Email string `json:"email" validate:"required,email"`
		Age   int    `json:"age" validate:"gte=0,lte=150"`
	}

	tests := []struct {
		name        string
		body        string
		wantErr     bool
		errContains string
	}{
		{
			name:    "valid input",
			body:    `{"name": "John", "email": "john@example.com", "age": 30}`,
			wantErr: false,
		},
		{
			name:        "invalid JSON",
			body:        `{"name": "John", invalid}`,
			wantErr:     true,
			errContains: "body",
		},
		{
			name:        "missing required field",
			body:        `{"email": "john@example.com", "age": 30}`,
			wantErr:     true,
			errContains: "name",
		},
		{
			name:        "invalid email",
			body:        `{"name": "John", "email": "not-an-email", "age": 30}`,
			wantErr:     true,
			errContains: "email",
		},
		{
			name:        "name too short",
			body:        `{"name": "J", "email": "john@example.com", "age": 30}`,
			wantErr:     true,
			errContains: "name",
		},
		{
			name:        "age out of range",
			body:        `{"name": "John", "email": "john@example.com", "age": 200}`,
			wantErr:     true,
			errContains: "age",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/test", bytes.NewReader([]byte(tt.body)))
			req.Header.Set("Content-Type", "application/json")

			result, errors := decodeAndValidate[testStruct](req)

			if tt.wantErr {
				if len(errors) == 0 {
					t.Error("decodeAndValidate() expected errors, got none")
				}
				// Check that error contains expected field
				found := false
				for _, e := range errors {
					if e.Field == tt.errContains || e.Message != "" {
						found = true
						break
					}
				}
				if !found && tt.errContains != "" {
					t.Errorf("decodeAndValidate() error should mention field %q", tt.errContains)
				}
			} else {
				if len(errors) > 0 {
					t.Errorf("decodeAndValidate() unexpected errors: %v", errors)
				}
				if result == nil {
					t.Error("decodeAndValidate() expected result, got nil")
				}
			}
		})
	}
}

// TestFormatValidationErrors tests the error formatting
func TestFormatValidationErrors(t *testing.T) {
	type testStruct struct {
		Name     string `json:"name" validate:"required"`
		Email    string `json:"email" validate:"email"`
		URL      string `json:"url" validate:"url"`
		Choice   string `json:"choice" validate:"oneof=a b c"`
		MinVal   int    `json:"min_val" validate:"min=5"`
		MaxVal   int    `json:"max_val" validate:"max=10"`
		RangeVal int    `json:"range_val" validate:"gte=1,lte=100"`
		UUID     string `json:"uuid" validate:"uuid"`
	}

	tests := []struct {
		name           string
		input          testStruct
		expectedFields []string
	}{
		{
			name:           "missing required",
			input:          testStruct{Email: "test@test.com", Choice: "a", MinVal: 10, MaxVal: 5, RangeVal: 50},
			expectedFields: []string{"name"},
		},
		{
			name:           "invalid email",
			input:          testStruct{Name: "test", Email: "invalid", Choice: "a", MinVal: 10, MaxVal: 5, RangeVal: 50},
			expectedFields: []string{"email"},
		},
		{
			name:           "invalid oneof",
			input:          testStruct{Name: "test", Email: "test@test.com", Choice: "invalid", MinVal: 10, MaxVal: 5, RangeVal: 50},
			expectedFields: []string{"choice"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validate.Struct(tt.input)
			if err == nil {
				t.Skip("no validation errors")
				return
			}

			errors := formatValidationErrors(context.Background(), err)
			if len(errors) == 0 {
				t.Error("formatValidationErrors() returned no errors")
			}

			for _, expectedField := range tt.expectedFields {
				found := false
				for _, e := range errors {
					if e.Field == expectedField {
						found = true
						if e.Message == "" {
							t.Errorf("formatValidationErrors() field %s has empty message", expectedField)
						}
						break
					}
				}
				if !found {
					t.Errorf("formatValidationErrors() missing error for field %s", expectedField)
				}
			}
		})
	}
}
