package token2

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIdentify(t *testing.T) {
	identity, ok := Identify("72103654095303")
	require.True(t, ok)

	assert.Equal(t, "72103", identity.Prefix)
	assert.Equal(t, byte('6'), identity.CheckDigit)
	assert.Equal(t, "54095303", identity.Suffix)
	assert.Equal(t, "R3.2", identity.Model.Release)
	assert.Equal(t, "Bio3 Dual A+C PIN+", identity.Model.FormFactor)
}

func TestIdentifyCustomCard(t *testing.T) {
	identity, ok := Identify("70000042")
	require.True(t, ok)

	assert.Equal(t, "R3.1", identity.Model.Release)
	assert.Equal(t, "Custom system access card", identity.Model.FormFactor)
}

func TestIdentifyRejectsInvalidSerialNumber(t *testing.T) {
	for _, serialNumber := range []string{
		"",
		"72103",
		"721036",
		"72103x54095303",
		"+72103654095303",
		"18446744073709551616",
	} {
		t.Run(serialNumber, func(t *testing.T) {
			identity, ok := Identify(serialNumber)

			assert.False(t, ok)
			assert.Equal(t, serialNumber, identity.SerialNumber)
			assert.Empty(t, identity.Prefix)
			assert.Empty(t, identity.Suffix)
		})
	}
}

func TestModelDisplayName(t *testing.T) {
	tests := []struct {
		name  string
		model Model
		want  string
	}{
		{
			name: "complete model",
			model: Model{
				Branding:   "Token2",
				FormFactor: "Bio3 Dual A+C PIN+",
				Release:    "R3.2",
			},
			want: "Token2 Bio3 Dual A+C PIN+",
		},
		{
			name: "model without branding",
			model: Model{
				FormFactor: "Mini USB-C PIN+",
				Release:    "R3.1",
			},
			want: "Mini USB-C PIN+",
		},
		{
			name: "empty model",
			want: "",
		},
		{
			name: "trims catalog fields",
			model: Model{
				Branding:   " Token2 ",
				FormFactor: " USB-A NFC ",
				Release:    " R1 ",
			},
			want: "Token2 USB-A NFC",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.model.DisplayName())
		})
	}
}

func TestModelsReturnsCopy(t *testing.T) {
	catalog := Models()
	require.NotEmpty(t, catalog)

	catalog[0].Prefix = "changed"
	identity, ok := Identify("86105012345678")
	require.True(t, ok)
	assert.Equal(t, "86105", identity.Model.Prefix)
}
