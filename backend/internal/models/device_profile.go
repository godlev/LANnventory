package models

// DeviceProfile stores generic manually managed device-profile information.
//
// Imported/discovered integration data must not be written to this model. Integrations
// use separate source-specific state so automatic collection can never silently
// overwrite user-managed values.
type DeviceProfile struct {
	Mac               string `gorm:"column:MAC;primaryKey" json:"mac"`
	Manufacturer      string `gorm:"column:MANUFACTURER" json:"manufacturer"`
	Model             string `gorm:"column:MODEL" json:"model"`
	ManagementAddress string `gorm:"column:MANAGEMENT_ADDRESS" json:"managementAddress"`
	UpdatedAt         string `gorm:"column:UPDATED_AT" json:"updatedAt"`
}

// DeviceProfileUpdate describes a partial managed profile update.
type DeviceProfileUpdate struct {
	Manufacturer      *string
	Model             *string
	ManagementAddress *string
}
