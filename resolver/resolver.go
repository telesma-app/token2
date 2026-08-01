// Package resolver correlates one FIDO attachment with Token2's auxiliary
// interfaces and returns transport-neutral device information.
package resolver

import (
	"context"
	"errors"
	"strings"
	"time"

	ghid "github.com/go-ctap/hid"
	"github.com/go-ctap/pcsc"
	"github.com/go-ctap/token2"
	token2hid "github.com/go-ctap/token2/transport/hid"
	token2pcsc "github.com/go-ctap/token2/transport/pcsc"
)

const (
	token2VendorID           uint16 = 0x349e
	resolveAttempts                 = 3
	defaultResolveRetryDelay        = 200 * time.Millisecond
)

var (
	// ErrNotApplicable reports that the exact smart-card target is not a
	// Token2 device.
	ErrNotApplicable = errors.New("token2 resolver: target is not Token2")
	// ErrIdentityUnavailable reports that Token2 was expected, but no
	// auxiliary interface could be correlated with the target.
	ErrIdentityUnavailable = errors.New("token2 resolver: identity unavailable")
	// ErrAmbiguous reports conflicting, independently correlated identities.
	ErrAmbiguous = errors.New("token2 resolver: ambiguous identity")
)

// HIDTarget describes one FIDO HID attachment and the evidence that may
// correlate it with Token2 auxiliary interfaces.
type HIDTarget struct {
	ReportedSerial string
	ProductID      uint16
	InstanceID     string
	ParentDeviceID string
	ATR            *token2.ATR
}

type device interface {
	Close() error
	DeviceInfo(context.Context) (token2.DeviceInfo, error)
}

type atrDevice interface {
	ATR(context.Context) (token2.ATR, error)
}

type configurationDevice interface {
	Configuration(context.Context) (token2.Configuration, error)
}

// Resolver resolves targets against local PC/SC and HID topology.
type Resolver struct {
	enumerateReaders func() ([]string, error)
	enumerateHID     func() ([]hidInfo, error)
	openPCSC         func(string) (device, error)
	openHID          func(string) (device, error)
	retryDelay       time.Duration
}

type hidInfo struct {
	path           string
	serial         string
	productID      uint16
	instanceID     string
	parentDeviceID string
}

// NewLocal creates a resolver backed by the host HID and PC/SC services.
func NewLocal() *Resolver {
	return &Resolver{
		enumerateReaders: localReaders,
		enumerateHID:     localHID,
		openPCSC: func(reader string) (device, error) {
			return token2pcsc.Open(reader)
		},
		openHID: func(path string) (device, error) {
			return token2hid.Open(path)
		},
		retryDelay: defaultResolveRetryDelay,
	}
}

// ResolveSmartCard reads device information from the exact PC/SC reader.
func (r *Resolver) ResolveSmartCard(
	ctx context.Context,
	reader string,
) (token2.DeviceInfo, error) {
	if err := ctx.Err(); err != nil {
		return token2.DeviceInfo{}, err
	}

	opened, err := r.openPCSC(reader)
	if err != nil {
		return token2.DeviceInfo{}, err
	}
	defer opened.Close()

	info, err := readDeviceInfo(ctx, opened)
	if isMissingApplication(err) {
		return token2.DeviceInfo{}, ErrNotApplicable
	}

	return info, err
}

// ResolveHID correlates a FIDO HID attachment with Token2's PC/SC and feature
// HID interfaces.
func (r *Resolver) ResolveHID(
	ctx context.Context,
	target HIDTarget,
) (token2.DeviceInfo, error) {
	if err := ctx.Err(); err != nil {
		return token2.DeviceInfo{}, err
	}

	var info token2.DeviceInfo
	var err error
	for attempt := range resolveAttempts {
		info, err = r.resolveHIDOnce(ctx, target)
		if ctxErr := ctx.Err(); ctxErr != nil {
			return token2.DeviceInfo{}, ctxErr
		}
		if err == nil || errors.Is(err, ErrAmbiguous) {
			return info, err
		}
		if !errors.Is(err, ErrIdentityUnavailable) || attempt == resolveAttempts-1 {
			return token2.DeviceInfo{}, err
		}
		if err := waitForRetry(ctx, r.retryDelay); err != nil {
			return token2.DeviceInfo{}, err
		}
	}

	return info, err
}

func (r *Resolver) resolveHIDOnce(
	ctx context.Context,
	target HIDTarget,
) (token2.DeviceInfo, error) {
	var proved []token2.DeviceInfo
	var sourceErr error

	pcscCandidates, err := r.pcscCandidates(ctx)
	if ctxErr := ctx.Err(); ctxErr != nil {
		return token2.DeviceInfo{}, ctxErr
	}
	sourceErr = errors.Join(sourceErr, err)
	if target.ReportedSerial != "" {
		proved = append(proved, matchingSerial(pcscCandidates, target.ReportedSerial)...)
	}

	if target.ATR != nil {
		proved = append(proved, matchingATR(pcscCandidates, *target.ATR, target.ProductID)...)
	}

	hidCandidates, err := r.featureHIDCandidates(ctx, target)
	sourceErr = errors.Join(sourceErr, err)
	proved = append(proved, hidCandidates...)

	proved = deduplicate(proved)
	switch len(proved) {
	case 1:
		info := proved[0]
		if target.ATR != nil {
			info.ProductID = target.ATR.ProductID
		}
		return info, nil
	case 0:
		return token2.DeviceInfo{}, errors.Join(ErrIdentityUnavailable, sourceErr)
	default:
		return token2.DeviceInfo{}, ErrAmbiguous
	}
}

func (r *Resolver) pcscCandidates(ctx context.Context) ([]token2.DeviceInfo, error) {
	var candidates []token2.DeviceInfo
	var sourceErr error
	readers, err := r.enumerateReaders()
	if err != nil {
		return nil, err
	}
	for _, reader := range readers {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		opened, err := r.openPCSC(reader)
		if err != nil {
			sourceErr = errors.Join(sourceErr, err)
			continue
		}
		info, err := readDeviceInfo(ctx, opened)
		_ = opened.Close()
		if err != nil {
			if !isMissingApplication(err) {
				sourceErr = errors.Join(sourceErr, err)
			}
			continue
		}
		candidates = append(candidates, info)
	}

	return deduplicate(candidates), sourceErr
}

func (r *Resolver) featureHIDCandidates(
	ctx context.Context,
	target HIDTarget,
) ([]token2.DeviceInfo, error) {
	var candidates []token2.DeviceInfo
	var sourceErr error
	infos, err := r.enumerateHID()
	if err != nil {
		return nil, err
	}
	for _, info := range infos {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if info.productID != target.ProductID {
			continue
		}

		proved := target.ReportedSerial != "" && info.serial == target.ReportedSerial ||
			target.ParentDeviceID != "" && info.parentDeviceID == target.ParentDeviceID ||
			target.InstanceID != "" && info.instanceID == target.InstanceID
		if !proved {
			continue
		}

		opened, err := r.openHID(info.path)
		if err != nil {
			sourceErr = errors.Join(sourceErr, err)
			continue
		}
		info, err := readDeviceInfo(ctx, opened)
		_ = opened.Close()
		if err != nil {
			sourceErr = errors.Join(sourceErr, err)
			continue
		}
		candidates = append(candidates, info)
	}

	return deduplicate(candidates), sourceErr
}

func readDeviceInfo(ctx context.Context, opened device) (token2.DeviceInfo, error) {
	info, err := opened.DeviceInfo(ctx)
	if err != nil {
		return token2.DeviceInfo{}, err
	}

	if source, ok := opened.(atrDevice); ok {
		atr, err := source.ATR(ctx)
		if ctxErr := ctx.Err(); ctxErr != nil {
			return token2.DeviceInfo{}, ctxErr
		}
		if err == nil {
			info.ProductID = atr.ProductID
		}
	}
	if source, ok := opened.(configurationDevice); ok {
		configuration, err := source.Configuration(ctx)
		if ctxErr := ctx.Err(); ctxErr != nil {
			return token2.DeviceInfo{}, ctxErr
		}
		if err == nil {
			applyConfiguration(&info, configuration)
		}
	}

	return info, nil
}

func applyConfiguration(info *token2.DeviceInfo, configuration token2.Configuration) {
	info.InterfaceStateKnown = true
	info.FIDOEnabled = configuration.TransferType&token2.TransferTypeFIDODisabledMask == 0
	info.HOTPKeystrokeEnabled = configuration.TransferType&token2.TransferTypeHOTPKeystrokeDisabledMask == 0
	info.CCIDEnabled = configuration.TransferType&token2.TransferTypeCCIDDisabledMask == 0

	if len(configuration.Raw) < 10 {
		return
	}

	appearance := configuration.Appearance
	version := configuration.FIDOVersion
	info.Appearance = &appearance
	info.FIDOVersion = &version
	info.CapabilitiesKnown = true
	info.FIDOPINSet = configuration.DeviceConfiguration&token2.DeviceConfigurationFIDOPINSetMask != 0
	info.FIDOPINLocked = configuration.DeviceConfiguration&token2.DeviceConfigurationFIDOPINLockedMask != 0
	info.SupportsHOTP = configuration.DeviceConfiguration&token2.DeviceConfigurationHOTPMask != 0
	info.SupportsTOTP = configuration.DeviceExtension&token2.DeviceExtensionTOTPMask != 0
	info.SupportsNFC = configuration.DeviceConfiguration&token2.DeviceConfigurationNFCMask != 0
	info.SupportsCCID = configuration.DeviceExtension&token2.DeviceExtensionCCIDMask != 0
	info.SupportsFIDO21 = configuration.DeviceExtension&token2.DeviceExtensionFIDO21Mask != 0
	info.HasFingerprintSensor = configuration.DeviceConfiguration&token2.DeviceConfigurationFingerprintSensorMask != 0
	info.SupportsFingerprintRegistration = configuration.DeviceExtension&token2.DeviceExtensionFingerprintRegistrationMask != 0
	info.SupportsMandatoryFingerprint = configuration.DeviceExtension&token2.DeviceExtensionMandatoryFingerprintSupportMask != 0
	info.OTPRequiresFingerprint = configuration.DeviceExtension&token2.DeviceExtensionOTPRequiresFingerprintMask != 0
	info.SupportsButtonHOTP = configuration.DeviceExtension&token2.DeviceExtensionButtonHOTPUnsupportedMask == 0
	info.ButtonHOTPConfigured = configuration.DeviceConfiguration&token2.DeviceConfigurationButtonHOTPConfiguredMask != 0
	info.ButtonHOTPSendsEnter = configuration.DeviceConfiguration&token2.DeviceConfigurationHOTPSuppressEnterMask == 0
	info.ButtonHOTPRequiresLongPress = configuration.DeviceConfiguration&token2.DeviceConfigurationHOTPLongPressMask != 0
	info.ButtonHOTPUsesNumericKeypad = configuration.DeviceExtension&token2.DeviceExtensionHOTPNumericKeypadMask != 0
}

func matchingSerial(
	candidates []token2.DeviceInfo,
	serial string,
) []token2.DeviceInfo {
	var matched []token2.DeviceInfo
	for _, candidate := range candidates {
		if candidate.SerialNumber == serial {
			matched = append(matched, candidate)
		}
	}
	return matched
}

func matchingATR(
	candidates []token2.DeviceInfo,
	atr token2.ATR,
	productID uint16,
) []token2.DeviceInfo {
	if atr.ProductID == 0 ||
		atr.SerialSuffix == "" ||
		productID != 0 && atr.ProductID != productID {
		return nil
	}

	var matched []token2.DeviceInfo
	for _, candidate := range candidates {
		if strings.HasSuffix(candidate.SerialNumber, atr.SerialSuffix) {
			matched = append(matched, candidate)
		}
	}
	return matched
}

func deduplicate(candidates []token2.DeviceInfo) []token2.DeviceInfo {
	seen := make(map[string]struct{}, len(candidates))
	unique := candidates[:0]
	for _, candidate := range candidates {
		serial := candidate.SerialNumber
		if _, ok := seen[serial]; ok {
			continue
		}
		seen[serial] = struct{}{}
		unique = append(unique, candidate)
	}
	return unique
}

func isMissingApplication(err error) bool {
	return errors.Is(err, token2pcsc.ErrOTPApplicationNotAvailable)
}

func waitForRetry(ctx context.Context, delay time.Duration) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if delay <= 0 {
		return nil
	}

	timer := time.NewTimer(delay)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func localReaders() ([]string, error) {
	var readers []string
	for info, err := range pcsc.Enumerate() {
		if err != nil {
			return nil, err
		}
		if info.State&pcsc.ReaderStatePresent != 0 {
			readers = append(readers, info.Name)
		}
	}
	return readers, nil
}

func localHID() ([]hidInfo, error) {
	var infos []hidInfo
	for info, err := range ghid.Enumerate(ghid.WithVendorID(token2VendorID)) {
		if err != nil {
			return nil, err
		}
		infos = append(infos, hidInfo{
			path:           info.Path,
			serial:         info.SerialNbr,
			productID:      info.ProductID,
			instanceID:     info.InstanceID,
			parentDeviceID: info.ParentDeviceID,
		})
	}
	return infos, nil
}
