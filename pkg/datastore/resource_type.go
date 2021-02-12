package datastore

import (
	"database/sql/driver"
	"fmt"

	pb "github.com/whywaita/myshoes/api/proto"
)

// ResourceType is runner machine spec
type ResourceType int

// ResourceTypes variables
const (
	ResourceTypeUnknown ResourceType = iota
	ResourceTypeNano
	ResourceTypeMicro
	ResourceTypeSmall
	ResourceTypeMedium
	ResourceTypeLarge
	ResourceTypeXLarge
	ResourceType2XLarge
	ResourceType3XLarge
	ResourceType4XLarge
)

// String implement interface for fmt.Stringer
func (r ResourceType) String() string {
	switch r {
	case ResourceTypeNano:
		return "nano"
	case ResourceTypeMicro:
		return "micro"
	case ResourceTypeSmall:
		return "small"
	case ResourceTypeMedium:
		return "medium"
	case ResourceTypeLarge:
		return "large"
	case ResourceTypeXLarge:
		return "xlarge"
	case ResourceType2XLarge:
		return "2xlarge"
	case ResourceType3XLarge:
		return "3xlarge"
	case ResourceType4XLarge:
		return "4xlarge"
	}

	return "unknown"
}

// Value implements the database/sql/driver Valuer interface.
func (r ResourceType) Value() (driver.Value, error) {
	return driver.Value(r.String()), nil
}

// Scan implements the database/sql Scanner interface.
func (r *ResourceType) Scan(src interface{}) error {
	var rt *ResourceType
	switch src := src.(type) {
	case string:
		unmarshaled := UnmarshalResourceType(src)
		rt = &unmarshaled
	case []uint8:
		str := string(src)
		unmarshaled := UnmarshalResourceType(str)
		rt = &unmarshaled
	default:
		return fmt.Errorf("incompatible type for ResourceType: %T", src)
	}

	*r = *rt
	return nil
}

// UnmarshalResourceType cast type to ResourceType
func UnmarshalResourceType(src interface{}) ResourceType {
	switch src := src.(type) {
	case string:
		return UnmarshalResourceTypeString(src)
	case pb.ResourceType:
		return UnmarshalResourceTypePb(src)
	}

	return ResourceTypeUnknown
}

// UnmarshalResourceTypeString cast type from string to ResourceType
func UnmarshalResourceTypeString(in string) ResourceType {
	switch in {
	case "nano":
		return ResourceTypeNano
	case "micro":
		return ResourceTypeMicro
	case "small":
		return ResourceTypeSmall
	case "medium":
		return ResourceTypeMedium
	case "large":
		return ResourceTypeLarge
	case "xlarge":
		return ResourceTypeXLarge
	case "2xlarge":
		return ResourceType2XLarge
	case "3xlarge":
		return ResourceType3XLarge
	case "4xlarge":
		return ResourceType4XLarge
	}

	return ResourceTypeUnknown
}

// UnmarshalResourceTypePb cast type from pb.ResourType to ResourceType
func UnmarshalResourceTypePb(in pb.ResourceType) ResourceType {
	switch in {
	case pb.ResourceType_Nano:
		return ResourceTypeNano
	case pb.ResourceType_Micro:
		return ResourceTypeMicro
	case pb.ResourceType_Small:
		return ResourceTypeSmall
	case pb.ResourceType_Medium:
		return ResourceTypeMedium
	case pb.ResourceType_Large:
		return ResourceTypeLarge
	case pb.ResourceType_XLarge:
		return ResourceTypeXLarge
	case pb.ResourceType_XLarge2:
		return ResourceType2XLarge
	case pb.ResourceType_XLarge3:
		return ResourceType3XLarge
	case pb.ResourceType_XLarge4:
		return ResourceType4XLarge
	}

	return ResourceTypeUnknown
}

// ToPb convert type of protobuf
func (r ResourceType) ToPb() pb.ResourceType {
	switch r {
	case ResourceTypeNano:
		return pb.ResourceType_Nano
	case ResourceTypeMicro:
		return pb.ResourceType_Micro
	case ResourceTypeSmall:
		return pb.ResourceType_Small
	case ResourceTypeMedium:
		return pb.ResourceType_Medium
	case ResourceTypeLarge:
		return pb.ResourceType_Large
	case ResourceTypeXLarge:
		return pb.ResourceType_XLarge
	case ResourceType2XLarge:
		return pb.ResourceType_XLarge2
	case ResourceType3XLarge:
		return pb.ResourceType_XLarge3
	case ResourceType4XLarge:
		return pb.ResourceType_XLarge4
	}

	return pb.ResourceType_Unknown
}
