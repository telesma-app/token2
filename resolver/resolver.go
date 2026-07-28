// Package resolver correlates one FIDO attachment with Token2's auxiliary
// interfaces and returns one transport-neutral device identity.
package resolver

import (
	"context"
	"errors"
	"strings"
	"time"

	ghid "github.com/go-ctap/hid"
	"github.com/go-ctap/pcsc"
	"github.com/go-ctap/token2"
	"github.com/go-ctap/token2/apdu"
	token2hid "github.com/go-ctap/token2/transport/hid"
	token2pcsc "github.com/go-ctap/token2/transport/pcsc"
)

const (
	token2VendorID             uint16 = 0x349e
	fidoUsagePage              uint16 = 0xf1d0
	fidoUsage                  uint16 = 0x01
	resolveAttempts                   = 3
	defaultResolveRetryDelay          = 200 * time.Millisecond
	selectApplicationOperation        = "select Token2 OTP application"
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
	ATR            *token2.ATRInfo
}

// Result is one resolved Token2 identity.
type Result struct {
	Identity   token2.Identity
	ModelKnown bool
}

// Resolver resolves targets against local PC/SC and HID topology.
type Resolver struct {
	enumerateReaders func() ([]string, error)
	enumerateHID     func() ([]hidInfo, error)
	openPCSC         func(string) (token2.Device, error)
	openHID          func(string) (token2.Device, error)
	retryDelay       time.Duration
}

type hidInfo struct {
	path           string
	serial         string
	productID      uint16
	usagePage      uint16
	usage          uint16
	instanceID     string
	parentDeviceID string
}

// NewLocal creates a resolver backed by the host HID and PC/SC services.
func NewLocal() *Resolver {
	return &Resolver{
		enumerateReaders: localReaders,
		enumerateHID:     localHID,
		openPCSC: func(reader string) (token2.Device, error) {
			return token2pcsc.Open(reader)
		},
		openHID: func(path string) (token2.Device, error) {
			return token2hid.Open(path)
		},
		retryDelay: defaultResolveRetryDelay,
	}
}

// ResolveSmartCard reads identity from the exact PC/SC reader.
func (r *Resolver) ResolveSmartCard(ctx context.Context, reader string) (Result, error) {
	if err := ctx.Err(); err != nil {
		return Result{}, err
	}

	opened, err := r.openPCSC(reader)
	if err != nil {
		return Result{}, err
	}
	defer opened.Close()

	result, err := readIdentity(ctx, opened)
	if isMissingApplication(err) {
		return Result{}, ErrNotApplicable
	}

	return result, err
}

// ResolveHID correlates a FIDO HID attachment with Token2's PC/SC and feature
// HID interfaces.
func (r *Resolver) ResolveHID(ctx context.Context, target HIDTarget) (Result, error) {
	if err := ctx.Err(); err != nil {
		return Result{}, err
	}

	var result Result
	var err error
	for attempt := 0; attempt < resolveAttempts; attempt++ {
		result, err = r.resolveHIDOnce(ctx, target)
		if ctxErr := ctx.Err(); ctxErr != nil {
			return Result{}, ctxErr
		}
		if err == nil || errors.Is(err, ErrAmbiguous) {
			return result, err
		}
		if !errors.Is(err, ErrIdentityUnavailable) || attempt == resolveAttempts-1 {
			return Result{}, err
		}
		if err := waitForRetry(ctx, r.retryDelay); err != nil {
			return Result{}, err
		}
	}

	return result, err
}

func (r *Resolver) resolveHIDOnce(ctx context.Context, target HIDTarget) (Result, error) {
	var proved []Result
	var sourceErr error

	pcscCandidates, err := r.pcscCandidates(ctx)
	if ctxErr := ctx.Err(); ctxErr != nil {
		return Result{}, ctxErr
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
		return proved[0], nil
	case 0:
		return Result{}, errors.Join(ErrIdentityUnavailable, sourceErr)
	default:
		return Result{}, ErrAmbiguous
	}
}

func (r *Resolver) pcscCandidates(ctx context.Context) ([]Result, error) {
	var candidates []Result
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
		result, err := readIdentity(ctx, opened)
		_ = opened.Close()
		if err != nil {
			if !isMissingApplication(err) {
				sourceErr = errors.Join(sourceErr, err)
			}
			continue
		}
		candidates = append(candidates, result)
	}

	return deduplicate(candidates), sourceErr
}

func (r *Resolver) featureHIDCandidates(ctx context.Context, target HIDTarget) ([]Result, error) {
	var candidates []Result
	var sourceErr error
	infos, err := r.enumerateHID()
	if err != nil {
		return nil, err
	}
	for _, info := range infos {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if info.productID != target.ProductID ||
			info.usagePage == fidoUsagePage && info.usage == fidoUsage {
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
		result, err := readIdentity(ctx, opened)
		_ = opened.Close()
		if err != nil {
			sourceErr = errors.Join(sourceErr, err)
			continue
		}
		candidates = append(candidates, result)
	}

	return deduplicate(candidates), sourceErr
}

func readIdentity(ctx context.Context, opened token2.Device) (Result, error) {
	serialReader, ok := opened.(token2.SerialNumberDevice)
	if !ok {
		return Result{}, ErrIdentityUnavailable
	}
	serial, err := serialReader.SerialNumber(ctx)
	if err != nil {
		return Result{}, err
	}

	identity, known := token2.Identify(serial)
	return Result{
		Identity:   identity,
		ModelKnown: known,
	}, nil
}

func matchingSerial(candidates []Result, serial string) []Result {
	var matched []Result
	for _, candidate := range candidates {
		if candidate.Identity.SerialNumber == serial {
			matched = append(matched, candidate)
		}
	}
	return matched
}

func matchingATR(candidates []Result, atr token2.ATRInfo, productID uint16) []Result {
	if atr.ProductID == 0 ||
		atr.SerialSuffix == "" ||
		productID != 0 && atr.ProductID != productID {
		return nil
	}

	var matched []Result
	for _, candidate := range candidates {
		if strings.HasSuffix(candidate.Identity.SerialNumber, atr.SerialSuffix) {
			matched = append(matched, candidate)
		}
	}
	return matched
}

func deduplicate(candidates []Result) []Result {
	seen := make(map[string]struct{}, len(candidates))
	unique := candidates[:0]
	for _, candidate := range candidates {
		serial := candidate.Identity.SerialNumber
		if _, ok := seen[serial]; ok {
			continue
		}
		seen[serial] = struct{}{}
		unique = append(unique, candidate)
	}
	return unique
}

func isMissingApplication(err error) bool {
	var status *apdu.StatusError
	return errors.As(err, &status) &&
		status.Operation == selectApplicationOperation &&
		status.SW == 0x6a82
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
			usagePage:      info.UsagePage,
			usage:          info.Usage,
			instanceID:     info.InstanceID,
			parentDeviceID: info.ParentDeviceID,
		})
	}
	return infos, nil
}
