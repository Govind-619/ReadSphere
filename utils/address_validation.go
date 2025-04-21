package utils

import (
	"regexp"
	"strings"
)

var (
	addressLineRegex      = regexp.MustCompile(`^[a-zA-Z0-9\s,.'#\-/]+$`)
	addressLine2Regex     = regexp.MustCompile(`^[a-zA-Z0-9\s,.'#\-/]*$`)
	cityRegex             = regexp.MustCompile(`^[a-zA-Z\s]+$`)
	postalCodeIndiaRegex  = regexp.MustCompile(`^[1-9][0-9]{5}$`)
)

// ValidateAddressFields validates address fields according to business rules
func ValidateAddressFields(line1, line2, city, state, country, postalCode string, isDefault *bool) []FieldValidationError {
	errs := []FieldValidationError{}

	// line1: required, length, content
	line1 = strings.TrimSpace(line1)
	if line1 == "" {
		errs = append(errs, FieldValidationError{"line1", "Address Line 1 is required"})
	} else {
		if len(line1) > 150 {
			errs = append(errs, FieldValidationError{"line1", "Address Line 1 must not exceed 150 characters"})
		}
		if !addressLineRegex.MatchString(line1) {
			errs = append(errs, FieldValidationError{"line1", "Address Line 1 contains invalid characters"})
		}
	}

	// line2: optional, length, content
	line2 = strings.TrimSpace(line2)
	if len(line2) > 0 {
		if len(line2) > 100 {
			errs = append(errs, FieldValidationError{"line2", "Address Line 2 must not exceed 100 characters"})
		}
		if !addressLine2Regex.MatchString(line2) {
			errs = append(errs, FieldValidationError{"line2", "Address Line 2 contains invalid characters"})
		}
	}

	// city: required, length, content
	city = strings.TrimSpace(city)
	if city == "" {
		errs = append(errs, FieldValidationError{"city", "City is required"})
	} else {
		if len(city) > 100 {
			errs = append(errs, FieldValidationError{"city", "City must not exceed 100 characters"})
		}
		if !cityRegex.MatchString(city) {
			errs = append(errs, FieldValidationError{"city", "City must only contain letters and spaces"})
		}
	}

	// state: required, length
	state = strings.TrimSpace(state)
	if state == "" {
		errs = append(errs, FieldValidationError{"state", "State is required"})
	} else if len(state) > 100 {
		errs = append(errs, FieldValidationError{"state", "State must not exceed 100 characters"})
	}

	// country: required, length
	country = strings.TrimSpace(country)
	if country == "" {
		errs = append(errs, FieldValidationError{"country", "Country is required"})
	} else if len(country) > 100 {
		errs = append(errs, FieldValidationError{"country", "Country must not exceed 100 characters"})
	}
	// TODO: Optionally check against ISO country list

	// postal_code: required, Indian PIN validation
	postalCode = strings.TrimSpace(postalCode)
	if postalCode == "" {
		errs = append(errs, FieldValidationError{"postal_code", "Postal code is required"})
	} else {
		if country == "India" || country == "INDIA" || country == "india" {
			if !postalCodeIndiaRegex.MatchString(postalCode) {
				errs = append(errs, FieldValidationError{"postal_code", "Postal code must be a valid 6-digit Indian PIN (e.g., 600028)"})
			}
		}
		// TODO: Add more country-specific validations
	}

	return errs
}
