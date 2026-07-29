package resolver

import (
	"context"
	"errors"
	"testing"

	"github.com/go-ctap/token2"
	"github.com/go-ctap/token2/apdu"
)

func TestResolveSmartCardUsesExactReader(t *testing.T) {
	card := &fakeDevice{serial: "66202208969539"}
	resolver := testResolver()
	resolver.openPCSC = func(reader string) (token2.Device, error) {
		if reader != "reader-one" {
			t.Fatalf("reader = %q", reader)
		}
		return card, nil
	}

	result, err := resolver.ResolveSmartCard(t.Context(), "reader-one")
	if err != nil {
		t.Fatalf("ResolveSmartCard: %v", err)
	}
	if result.Identity.SerialNumber != card.serial ||
		!result.ModelKnown ||
		result.Identity.Model.DisplayName() != "Token2 FIDO Card NFC with ISO 7816 PIN+ PIV+" {
		t.Fatalf("result = %#v", result)
	}
}

func TestResolveSmartCardClassifiesMissingApplication(t *testing.T) {
	resolver := testResolver()
	resolver.openPCSC = func(string) (token2.Device, error) {
		return &fakeDevice{serialErr: &apdu.StatusError{
			Operation: selectApplicationOperation,
			SW:        0x6a82,
		}}, nil
	}

	_, err := resolver.ResolveSmartCard(t.Context(), "reader-one")
	if !errors.Is(err, ErrNotApplicable) {
		t.Fatalf("error = %v, want ErrNotApplicable", err)
	}
}

func TestResolveSmartCardDoesNotClassifySerialStatusAsMissingApplication(t *testing.T) {
	status := &apdu.StatusError{
		Operation: "read serial number",
		SW:        0x6a82,
	}
	resolver := testResolver()
	resolver.openPCSC = func(string) (token2.Device, error) {
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
	resolver.openPCSC = func(string) (token2.Device, error) {
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
	resolver.openPCSC = func(string) (token2.Device, error) {
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
	resolver.openPCSC = func(reader string) (token2.Device, error) {
		serials := map[string]string{
			"first":  "66103930925563",
			"second": "66202208969539",
		}
		return &fakeDevice{serial: serials[reader]}, nil
	}

	result, err := resolver.ResolveHID(t.Context(), HIDTarget{
		ReportedSerial: "66103930925563",
	})
	if err != nil || result.Identity.SerialNumber != "66103930925563" {
		t.Fatalf("result = %#v, error = %v", result, err)
	}
}

func TestResolveHIDMatchesATR(t *testing.T) {
	resolver := testResolver()
	resolver.enumerateReaders = func() ([]string, error) {
		return []string{"first", "second"}, nil
	}
	resolver.openPCSC = func(reader string) (token2.Device, error) {
		serials := map[string]string{
			"first":  "66103930925563",
			"second": "66202208969539",
		}
		return &fakeDevice{serial: serials[reader]}, nil
	}

	result, err := resolver.ResolveHID(t.Context(), HIDTarget{
		ProductID: 0x1234,
		ATR: &token2.ATRInfo{
			ProductID:    0x1234,
			SerialSuffix: "30925563",
		},
	})
	if err != nil || result.Identity.SerialNumber != "66103930925563" {
		t.Fatalf("result = %#v, error = %v", result, err)
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
	resolver.openHID = func(path string) (token2.Device, error) {
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
	if err != nil || result.Identity.SerialNumber != "66202208969539" {
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
	resolver.openHID = func(path string) (token2.Device, error) {
		if path != "fido-interface" {
			t.Fatalf("path = %q", path)
		}

		return &fakeDevice{serial: "72102935780528"}, nil
	}

	result, err := resolver.ResolveHID(t.Context(), HIDTarget{
		ProductID:  0x1234,
		InstanceID: "instance",
	})
	if err != nil || result.Identity.SerialNumber != "72102935780528" {
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
	resolver.openHID = func(string) (token2.Device, error) {
		return &fakeDevice{serial: "66202208969539"}, nil
	}

	result, err := resolver.ResolveHID(t.Context(), HIDTarget{
		ProductID:      0x1234,
		ParentDeviceID: "parent",
	})
	if err != nil || result.Identity.SerialNumber != "66202208969539" {
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
	resolver.openPCSC = func(string) (token2.Device, error) {
		cancel()
		return &fakeDevice{serialErr: context.Canceled}, nil
	}

	_, err := resolver.ResolveHID(ctx, HIDTarget{})
	if !errors.Is(err, context.Canceled) || errors.Is(err, ErrIdentityUnavailable) {
		t.Fatalf("error = %v, want context cancellation only", err)
	}
}

func TestMatchingATRRequiresProductAndSuffix(t *testing.T) {
	candidates := []Result{
		resultForSerial("66103930925563"),
		resultForSerial("66202208969539"),
	}

	matched := matchingATR(candidates, token2.ATRInfo{
		ProductID:    0x1234,
		SerialSuffix: "30925563",
	}, 0x1234)
	if len(matched) != 1 || matched[0].Identity.SerialNumber != "66103930925563" {
		t.Fatalf("matched = %#v", matched)
	}

	if got := matchingATR(candidates, token2.ATRInfo{
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
	resolver.openPCSC = func(string) (token2.Device, error) {
		return &fakeDevice{serial: "66103930925563"}, nil
	}
	resolver.enumerateHID = func() ([]hidInfo, error) {
		return []hidInfo{{
			path:           "feature",
			productID:      0x1234,
			parentDeviceID: "parent",
		}}, nil
	}
	resolver.openHID = func(string) (token2.Device, error) {
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

func (d *fakeDevice) Close() error { return nil }

func (d *fakeDevice) SerialNumber(context.Context) (string, error) {
	return d.serial, d.serialErr
}

func testResolver() *Resolver {
	return &Resolver{
		enumerateReaders: func() ([]string, error) { return nil, nil },
		enumerateHID:     func() ([]hidInfo, error) { return nil, nil },
		openPCSC: func(string) (token2.Device, error) {
			return nil, errors.New("unexpected PC/SC open")
		},
		openHID: func(string) (token2.Device, error) {
			return nil, errors.New("unexpected HID open")
		},
	}
}

func resultForSerial(serial string) Result {
	identity, known := token2.Identify(serial)
	return Result{
		Identity:   identity,
		ModelKnown: known,
	}
}
