package token2

import "testing"

func TestIdentifyModel(t *testing.T) {
	for _, tt := range []struct{ serial, release, name string }{
		{"72103654095303", "R3.2", "Token2 Bio3 Dual A+C PIN+"},
		{"70000042", "R3.1", "Custom system access card"},
		{"65111999999999", "R3.4", "Mini USB-C PIN+ PIV+ OTP Protection"},
	} {
		model, ok := Identify(tt.serial)
		if !ok || model.Release != tt.release || model.Name() != tt.name {
			t.Errorf("Identify(%q) = %#v, %v", tt.serial, model, ok)
		}
	}
}

func TestIdentifyUnknownModel(t *testing.T) {
	for _, serial := range []string{
		"", "72103", "721036", "72103x54095303",
		"+72103654095303", "-72103654095303", " 72103654095303", "72103654095303\n",
		"72103_654095303", "７２１０３６５４０９５３０３", "18446744073709551616", "99999999999999",
	} {
		if model, ok := Identify(serial); ok || model != (Model{}) {
			t.Errorf("Identify(%q) = %#v, %v", serial, model, ok)
		}
	}
}
