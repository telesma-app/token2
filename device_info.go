package token2

// Descriptor holds a device observation. A non-nil Value is a completed read,
// including zero values and empty strings. Err holds the original read failure.
// Both may be set when data was received before an error. Neither is set when
// the interface did not provide a reading. Errors are not serialized to JSON.
type Descriptor[T any] struct {
	Value *T    `json:"value"`
	Err   error `json:"-"`
}

// DeviceInfo contains the independent vendor readings from one interface.
// Correlation with other interfaces belongs to the consuming device runtime.
type DeviceInfo struct {
	SerialNumber  Descriptor[string]        `json:"serialNumber"`
	ATR           Descriptor[ATR]           `json:"atr"`
	Configuration Descriptor[Configuration] `json:"configuration"`
	Model         *Model                    `json:"model,omitempty"`
}

func (info DeviceInfo) HasData() bool {
	return info.SerialNumber.Value != nil || info.ATR.Value != nil || info.Configuration.Value != nil
}
