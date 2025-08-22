package testutil

import (
	"time"

	"github.com/AhaSend/ahasend-cli/internal/config"
	"github.com/AhaSend/ahasend-go/models/common"
	"github.com/AhaSend/ahasend-go/models/responses"
)

// TestProfile returns a test profile
func TestProfile() config.Profile {
	return config.Profile{
		APIKey:    "test-api-key",
		APIURL:    "https://api.ahasend.com",
		AccountID: "test-account-id",
		Name:      "Test Profile",
	}
}

// TestDomain returns a test domain
func TestDomain() *responses.Domain {
	now := time.Now()
	return &responses.Domain{
		Domain:         "example.com",
		DNSValid:       true,
		CreatedAt:      now,
		UpdatedAt:      now,
		LastDNSCheckAt: &now,
		DNSRecords: []responses.DNSRecord{
			{
				Type:    "TXT",
				Host:    "_dmarc.example.com",
				Content: "v=DMARC1; p=none;",
			},
			{
				Type:    "CNAME",
				Host:    "link.example.com",
				Content: "track.ahasend.com",
			},
		},
	}
}

// TestDomainList returns a test paginated domains response
func TestDomainList() *responses.PaginatedDomainsResponse {
	domains := []responses.Domain{
		*TestDomain(),
		{
			Domain:         "test.com",
			DNSValid:       false,
			CreatedAt:      time.Now(),
			UpdatedAt:      time.Now(),
			LastDNSCheckAt: nil,
		},
	}

	return &responses.PaginatedDomainsResponse{
		Data: domains,
		Pagination: common.PaginationInfo{
			HasMore:    false,
			NextCursor: nil,
		},
	}
}
