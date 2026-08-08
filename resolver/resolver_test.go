package resolver

import (
	"context"
	"errors"
	"testing"

	"github.com/telesma-app/token2"
	token2pcsc "github.com/telesma-app/token2/transport/pcsc"
)

func TestResolveSmartCardUsesExactReader(t *testing.T) {
	card := &fakeDevice{serial: "66202208969539"}
	resolver := testResolver()
	resolver.openPCSC = func(reader string) (device, error) {
		if reader != "reader-one" {
			t.Fatalf("reader = %q", reader)
		}
		return card, nil
	}

	result, err := resolver.ResolveSmartCard(t.Context(), "reader-one")
	if err != nil {
		t.Fatalf("ResolveSmartCard: %v", err)
	}
	if result.SerialNumber != card.serial ||
		result.ModelName("") != "Token2 FIDO Card NFC with ISO 7816 PIN+ PIV+" {
		t.Fatalf("result = %#v", result)
	}
}

func TestResolveSmartCardCollectsOptionalInformation(t *testing.T) {
	card := &fakeRichDevice{
		fakeDevice: fakeDevice{serial: "72103654095303"},
		atr: token2.ATR{
			ProductID:    0x1234,
			SerialSuffix: "54095303",
		},
		configuration: token2.Configuration{
			Raw:                 make([]byte, 10),
			TransferType:        2,
			DeviceConfiguration: 0xaa,
			Appearance:          token2.Appearance{0x86, 0x01, 0x10, 0x00},
			FIDOVersion:         token2.FIDOVersion{Major: 2, Minor: 1, Patch: 2},
			DeviceExtension:     0xd5,
		},
	}
	resolver := testResolver()
	resolver.openPCSC = func(string) (device, error) {
		return card, nil
	}

	info, err := resolver.ResolveSmartCard(t.Context(), "reader-one")
	if err != nil {
		t.Fatalf("ResolveSmartCard: %v", err)
	}
	if info.ProductID != card.atr.ProductID {
		t.Fatalf("product ID = %04x", info.ProductID)
	}
	if info.Appearance == nil || *info.Appearance != card.configuration.Appearance {
		t.Fatalf("appearance = %#v", info.Appearance)
	}
	if info.FIDOVersion == nil || *info.FIDOVersion != card.configuration.FIDOVersion {
		t.Fatalf("FIDO version = %#v", info.FIDOVersion)
	}
	if !info.InterfaceStateKnown || !info.FIDOEnabled || info.HOTPKeystrokeEnabled || !info.CCIDEnabled {
		t.Fatalf("interface state = %#v", info)
	}
	if !info.CapabilitiesKnown ||
		!info.FIDOPINSet ||
		info.FIDOPINLocked ||
		info.SupportsHOTP ||
		!info.SupportsTOTP ||
		info.SupportsNFC ||
		!info.SupportsCCID ||
		info.SupportsFIDO21 ||
		!info.HasFingerprintSensor ||
		!info.SupportsFingerprintRegistration ||
		!info.SupportsMandatoryFingerprint ||
		!info.OTPRequiresFingerprint ||
		!info.SupportsButtonHOTP ||
		!info.ButtonHOTPConfigured ||
		!info.ButtonHOTPSendsEnter ||
		!info.ButtonHOTPRequiresLongPress ||
		info.ButtonHOTPUsesNumericKeypad {
		t.Fatalf("device details = %#v", info)
	}
}

func TestApplyConfigurationDecodesNegativeBits(t *testing.T) {
	info := token2.DeviceInfo{}
	applyConfiguration(&info, token2.Configuration{
		Raw:                 make([]byte, 10),
		DeviceConfiguration: token2.DeviceConfigurationHOTPSuppressEnterMask,
		DeviceExtension:     token2.DeviceExtensionButtonHOTPUnsupportedMask,
	})

	if info.ButtonHOTPSendsEnter || info.SupportsButtonHOTP {
		t.Fatalf("button HOTP = %#v", info)
	}
}

func TestApplyLegacyConfiguration(t *testing.T) {
	info := token2.DeviceInfo{}
	applyConfiguration(&info, token2.Configuration{
		Raw:          []byte{token2.TransferTypeHOTPKeystrokeDisabledMask},
		TransferType: token2.TransferTypeHOTPKeystrokeDisabledMask,
	})

	if !info.InterfaceStateKnown ||
		!info.FIDOEnabled ||
		info.HOTPKeystrokeEnabled ||
		!info.CCIDEnabled {
		t.Fatalf("interface state = %#v", info)
	}
	if info.CapabilitiesKnown || info.Appearance != nil || info.FIDOVersion != nil {
		t.Fatalf("device details = %#v", info)
	}
}

func TestResolveSmartCardClassifiesMissingApplication(t *testing.T) {
	resolver := testResolver()
	resolver.openPCSC = func(string) (device, error) {
		return &fakeDevice{serialErr: token2pcsc.ErrOTPApplicationNotAvailable}, nil
	}

	_, err := resolver.ResolveSmartCard(t.Context(), "reader-one")
	if !errors.Is(err, ErrNotApplicable) {
		t.Fatalf("error = %v, want ErrNotApplicable", err)
	}
}

func TestResolveSmartCardDoesNotClassifySerialStatusAsMissingApplication(t *testing.T) {
	status := errors.New("read serial number: APDU status 6a82")
	resolver := testResolver()
	resolver.openPCSC = func(string) (device, error) {
		return &fakeDevice{serialErr: status}, nil
	}

	_, err := resolver.ResolveSmartCard(t.Context(), "reader-one")
	if !errors.Is(err, status) || errors.Is(err, ErrNotApplicable) {
		t.Fatalf("error = %v, want original serial-number status", err)
	}
}

func TestResolveHIDDoesNotUseUniqueUnprovedCandidate(t *testing.T) {
	resolver := testResolver()
	resolver.enumerateReaders = func() ([]string, error) {
		return []string{"reader-one"}, nil
	}
	resolver.openPCSC = func(string) (device, error) {
		return &fakeDevice{serial: "66103930925563"}, nil
	}

	_, err := resolver.ResolveHID(t.Context(), HIDTarget{
		ProductID: 0x0001,
	})
	if !errors.Is(err, ErrIdentityUnavailable) {
		t.Fatalf("error = %v, want ErrIdentityUnavailable", err)
	}
}

func TestResolveHIDPreservesUnavailableClassificationWhenSourceFails(t *testing.T) {
	sourceErr := errors.New("reader is busy")
	resolver := testResolver()
	resolver.enumerateReaders = func() ([]string, error) {
		return []string{"reader-one"}, nil
	}
	resolver.openPCSC = func(string) (device, error) {
		return nil, sourceErr
	}

	_, err := resolver.ResolveHID(t.Context(), HIDTarget{})
	if !errors.Is(err, ErrIdentityUnavailable) || !errors.Is(err, sourceErr) {
		t.Fatalf("error = %v, want identity-unavailable and source errors", err)
	}
}

func TestResolveHIDMatchesExactReportedSerial(t *testing.T) {
	resolver := testResolver()
	resolver.enumerateReaders = func() ([]string, error) {
		return []string{"first", "second"}, nil
	}
	resolver.openPCSC = func(reader string) (device, error) {
		serials := map[string]string{
			"first":  "66103930925563",
			"second": "66202208969539",
		}
		return &fakeDevice{serial: serials[reader]}, nil
	}

	result, err := resolver.ResolveHID(t.Context(), HIDTarget{
		ReportedSerial: "66103930925563",
	})
	if err != nil || result.SerialNumber != "66103930925563" {
		t.Fatalf("result = %#v, error = %v", result, err)
	}
}

func TestResolveHIDMatchesATR(t *testing.T) {
	resolver := testResolver()
	resolver.enumerateReaders = func() ([]string, error) {
		return []string{"first", "second"}, nil
	}
	resolver.openPCSC = func(reader string) (device, error) {
		serials := map[string]string{
			"first":  "66103930925563",
			"second": "66202208969539",
		}
		return &fakeDevice{serial: serials[reader]}, nil
	}

	result, err := resolver.ResolveHID(t.Context(), HIDTarget{
		ProductID: 0x1234,
		ATR: &token2.ATR{
			ProductID:    0x1234,
			SerialSuffix: "30925563",
		},
	})
	if err != nil || result.SerialNumber != "66103930925563" {
		t.Fatalf("result = %#v, error = %v", result, err)
	}
	if result.ProductID != 0x1234 {
		t.Fatalf("product ID = %04x", result.ProductID)
	}
}

func TestResolveHIDMatchesFeatureInterfaceByParent(t *testing.T) {
	resolver := testResolver()
	resolver.enumerateHID = func() ([]hidInfo, error) {
		return []hidInfo{
			{
				path:           "feature-one",
				productID:      0x1234,
				parentDeviceID: "parent-one",
			},
			{
				path:           "feature-two",
				productID:      0x1234,
				parentDeviceID: "parent-two",
			},
		}, nil
	}
	resolver.openHID = func(path string) (device, error) {
		serials := map[string]string{
			"feature-one": "66103930925563",
			"feature-two": "66202208969539",
		}
		return &fakeDevice{serial: serials[path]}, nil
	}

	result, err := resolver.ResolveHID(t.Context(), HIDTarget{
		ProductID:      0x1234,
		ParentDeviceID: "parent-two",
	})
	if err != nil || result.SerialNumber != "66202208969539" {
		t.Fatalf("result = %#v, error = %v", result, err)
	}
}

func TestResolveHIDUsesMatchedFIDOInterfaceForFeatureReports(t *testing.T) {
	resolver := testResolver()
	resolver.enumerateHID = func() ([]hidInfo, error) {
		return []hidInfo{{
			path:       "fido-interface",
			productID:  0x1234,
			instanceID: "instance",
		}}, nil
	}
	resolver.openHID = func(path string) (device, error) {
		if path != "fido-interface" {
			t.Fatalf("path = %q", path)
		}

		return &fakeDevice{serial: "72102935780528"}, nil
	}

	result, err := resolver.ResolveHID(t.Context(), HIDTarget{
		ProductID:  0x1234,
		InstanceID: "instance",
	})
	if err != nil || result.SerialNumber != "72102935780528" {
		t.Fatalf("result = %#v, error = %v", result, err)
	}
}

func TestResolveHIDRetriesWhileCompositeInterfacesAppear(t *testing.T) {
	resolver := testResolver()
	enumerations := 0
	resolver.enumerateHID = func() ([]hidInfo, error) {
		enumerations++
		if enumerations < resolveAttempts {
			return nil, nil
		}

		return []hidInfo{{
			path:           "feature",
			productID:      0x1234,
			parentDeviceID: "parent",
		}}, nil
	}
	resolver.openHID = func(string) (device, error) {
		return &fakeDevice{serial: "66202208969539"}, nil
	}

	result, err := resolver.ResolveHID(t.Context(), HIDTarget{
		ProductID:      0x1234,
		ParentDeviceID: "parent",
	})
	if err != nil || result.SerialNumber != "66202208969539" {
		t.Fatalf("result = %#v, error = %v", result, err)
	}
	if enumerations != resolveAttempts {
		t.Fatalf("HID enumerations = %d, want %d", enumerations, resolveAttempts)
	}
}

func TestResolveHIDReturnsContextErrorDirectly(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	resolver := testResolver()
	resolver.enumerateReaders = func() ([]string, error) {
		return []string{"reader"}, nil
	}
	resolver.openPCSC = func(string) (device, error) {
		cancel()
		return &fakeDevice{serialErr: context.Canceled}, nil
	}

	_, err := resolver.ResolveHID(ctx, HIDTarget{})
	if !errors.Is(err, context.Canceled) || errors.Is(err, ErrIdentityUnavailable) {
		t.Fatalf("error = %v, want context cancellation only", err)
	}
}

func TestMatchingATRRequiresProductAndSuffix(t *testing.T) {
	candidates := []token2.DeviceInfo{
		resultForSerial("66103930925563"),
		resultForSerial("66202208969539"),
	}

	matched := matchingATR(candidates, token2.ATR{
		ProductID:    0x1234,
		SerialSuffix: "30925563",
	}, 0x1234)
	if len(matched) != 1 || matched[0].SerialNumber != "66103930925563" {
		t.Fatalf("matched = %#v", matched)
	}

	if got := matchingATR(candidates, token2.ATR{
		ProductID:    0x9999,
		SerialSuffix: "30925563",
	}, 0x1234); got != nil {
		t.Fatalf("product mismatch = %#v", got)
	}
}

func TestResolveHIDRejectsConflictingProofs(t *testing.T) {
	resolver := testResolver()
	resolver.enumerateReaders = func() ([]string, error) {
		return []string{"reader"}, nil
	}
	resolver.openPCSC = func(string) (device, error) {
		return &fakeDevice{serial: "66103930925563"}, nil
	}
	resolver.enumerateHID = func() ([]hidInfo, error) {
		return []hidInfo{{
			path:           "feature",
			productID:      0x1234,
			parentDeviceID: "parent",
		}}, nil
	}
	resolver.openHID = func(string) (device, error) {
		return &fakeDevice{serial: "66202208969539"}, nil
	}

	_, err := resolver.ResolveHID(t.Context(), HIDTarget{
		ReportedSerial: "66103930925563",
		ProductID:      0x1234,
		ParentDeviceID: "parent",
	})
	if !errors.Is(err, ErrAmbiguous) {
		t.Fatalf("error = %v, want ErrAmbiguous", err)
	}
}

type fakeDevice struct {
	serial    string
	serialErr error
}

type fakeRichDevice struct {
	fakeDevice
	atr           token2.ATR
	configuration token2.Configuration
}

func (d *fakeRichDevice) ATR(context.Context) (token2.ATR, error) {
	return d.atr, nil
}

func (d *fakeRichDevice) Configuration(context.Context) (token2.Configuration, error) {
	return d.configuration, nil
}

func (d *fakeDevice) Close() error { return nil }

func (d *fakeDevice) DeviceInfo(context.Context) (token2.DeviceInfo, error) {
	info, _ := token2.Identify(d.serial)
	return info, d.serialErr
}

func testResolver() *Resolver {
	return &Resolver{
		enumerateReaders: func() ([]string, error) { return nil, nil },
		enumerateHID:     func() ([]hidInfo, error) { return nil, nil },
		openPCSC: func(string) (device, error) {
			return nil, errors.New("unexpected PC/SC open")
		},
		openHID: func(string) (device, error) {
			return nil, errors.New("unexpected HID open")
		},
	}
}

func resultForSerial(serial string) token2.DeviceInfo {
	info, _ := token2.Identify(serial)
	return info
}
